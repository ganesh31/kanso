package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"kanso/internal/market"
)

// Source fetches an instrument's split-adjusted daily bars, with the split
// events it reports, for from <= date <= to. On error it returns no data.
type Source interface {
	Daily(ctx context.Context, symbol string, from, to market.Date) ([]market.Bar, []Split, error)
}

// Yahoo defaults.
const (
	YahooBaseURL      = "https://query1.finance.yahoo.com"
	yahooUserAgent    = "kanso/0.1"
	yahooMaxRetries   = 3
	yahooBaseDelay    = 500 * time.Millisecond
	yahooResponseSize = 32 << 20
)

// ist is the exchange zone Yahoo's request periods are computed in.
var ist = time.FixedZone("IST", 19800)

// Yahoo is the Source over Yahoo Finance's public chart endpoint (spec
// §6.1). It is unofficial and rate-limited, so 429 and 5xx responses are
// retried with exponential backoff.
type Yahoo struct {
	// BaseURL defaults to YahooBaseURL.
	BaseURL string
	// Client defaults to an http.Client with a 30s timeout.
	Client *http.Client
	// MaxRetries is the number of retries after the first try; 0 or less
	// means the default, 3 (4 tries).
	MaxRetries int
	// BaseDelay is the first backoff, doubled on each retry; 0 means
	// 500ms.
	BaseDelay time.Duration
}

var _ Source = (*Yahoo)(nil)

// Daily implements Source. symbol is Yahoo's ticker ("TCS.NS", "^NSEI").
func (y *Yahoo) Daily(ctx context.Context, symbol string, from, to market.Date) ([]market.Bar, []Split, error) {
	body, err := y.fetch(ctx, symbol, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("yahoo %s: %w", symbol, err)
	}
	bars, splits, err := ParseChart(body)
	if err != nil {
		return nil, nil, fmt.Errorf("yahoo %s: %w", symbol, err)
	}
	return bars, splits, nil
}

// errRetryable marks a response worth retrying.
var errRetryable = errors.New("retryable")

func (y *Yahoo) fetch(ctx context.Context, symbol string, from, to market.Date) ([]byte, error) {
	base, client, retries, delay := y.BaseURL, y.Client, y.MaxRetries, y.BaseDelay
	if base == "" {
		base = YahooBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	if retries <= 0 {
		retries = yahooMaxRetries
	}
	if delay <= 0 {
		delay = yahooBaseDelay
	}
	start, err := time.ParseInLocation(market.DateLayout, string(from), ist)
	if err != nil {
		return nil, fmt.Errorf("bad from date %q", from)
	}
	end, err := time.ParseInLocation(market.DateLayout, string(to), ist)
	if err != nil {
		return nil, fmt.Errorf("bad to date %q", to)
	}
	q := url.Values{
		"period1":  {strconv.FormatInt(start.Unix(), 10)},
		"period2":  {strconv.FormatInt(end.AddDate(0, 0, 1).Unix(), 10)},
		"interval": {"1d"},
		"events":   {"split"},
	}
	u := base + "/v8/finance/chart/" + url.PathEscape(symbol) + "?" + q.Encode()

	for try := 0; ; try++ {
		body, err := y.get(ctx, client, u)
		if err == nil || !errors.Is(err, errRetryable) || try == retries {
			return body, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay << try):
		}
	}
}

func (y *Yahoo) get(ctx context.Context, client *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", yahooUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %w", errRetryable, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, yahooResponseSize))
	if err != nil {
		return nil, fmt.Errorf("%w: read body: %w", errRetryable, err)
	}
	switch {
	case resp.StatusCode == http.StatusOK:
		return body, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return nil, fmt.Errorf("%w: HTTP %d", errRetryable, resp.StatusCode)
	default:
		if _, _, perr := ParseChart(body); perr != nil {
			return nil, fmt.Errorf("HTTP %d: %w", resp.StatusCode, perr)
		}
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
}

type chartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				GMTOffset int64 `json:"gmtoffset"`
			} `json:"meta"`
			Timestamp []int64 `json:"timestamp"`
			Events    struct {
				Splits map[string]struct {
					Date        int64   `json:"date"`
					Numerator   float64 `json:"numerator"`
					Denominator float64 `json:"denominator"`
				} `json:"splits"`
			} `json:"events"`
			Indicators struct {
				Quote []struct {
					Open   []*float64 `json:"open"`
					High   []*float64 `json:"high"`
					Low    []*float64 `json:"low"`
					Close  []*float64 `json:"close"`
					Volume []*float64 `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// ParseChart parses a chart endpoint response into bars sorted by date and
// split events. Dates are the exchange's trading dates, from the
// response's gmtoffset. Rows with any null price are dropped, a duplicate
// date keeps its last row, and a null volume reads as 0.
func ParseChart(body []byte) ([]market.Bar, []Split, error) {
	var resp chartResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, nil, fmt.Errorf("parse chart: %w", err)
	}
	if e := resp.Chart.Error; e != nil {
		return nil, nil, fmt.Errorf("chart error %s: %s", e.Code, e.Description)
	}
	if len(resp.Chart.Result) == 0 {
		return nil, nil, errors.New("chart: no result")
	}
	res := resp.Chart.Result[0]
	date := func(ts int64) market.Date {
		return market.Date(time.Unix(ts+res.Meta.GMTOffset, 0).UTC().Format(market.DateLayout))
	}

	byDate := map[market.Date]market.Bar{}
	if len(res.Indicators.Quote) > 0 {
		q := res.Indicators.Quote[0]
		at := func(xs []*float64, i int) *float64 {
			if i < len(xs) {
				return xs[i]
			}
			return nil
		}
		for i, ts := range res.Timestamp {
			o, h, l, c := at(q.Open, i), at(q.High, i), at(q.Low, i), at(q.Close, i)
			if o == nil || h == nil || l == nil || c == nil {
				continue
			}
			var v float64
			if p := at(q.Volume, i); p != nil {
				v = *p
			}
			d := date(ts)
			byDate[d] = market.Bar{Date: d, Open: *o, High: *h, Low: *l, Close: *c, Volume: v}
		}
	}
	bars := make([]market.Bar, 0, len(byDate))
	for _, b := range byDate {
		bars = append(bars, b)
	}
	sort.Slice(bars, func(i, j int) bool { return bars[i].Date < bars[j].Date })

	splits := make([]Split, 0, len(res.Events.Splits))
	for _, s := range res.Events.Splits {
		splits = append(splits, Split{Date: date(s.Date), Numerator: s.Numerator, Denominator: s.Denominator})
	}
	sort.Slice(splits, func(i, j int) bool { return splits[i].Date < splits[j].Date })
	return bars, splits, nil
}
