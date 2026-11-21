package api

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"

	"kanso/internal/market"
)

// backtestServer serves BREAK, which makes a new 3-year high on bar 795
// and holds it to the end of data (bar 799), plus EMPTY with no bars.
func backtestServer(t *testing.T) *httptest.Server {
	t.Helper()
	bars := make([]market.Bar, 800)
	for i := range bars {
		bars[i] = market.Bar{Date: date(i), Open: 99, High: 100, Low: 98, Close: 99, Volume: 1}
	}
	bars[300].High = 130
	for i := 795; i < 800; i++ {
		bars[i] = market.Bar{Date: date(i), Open: 131, High: 132, Low: 130, Close: 131, Volume: 1}
	}
	prov := market.NewMemProvider(
		[]market.Instrument{{ID: 1, Symbol: "BREAK", Type: market.Equity}, {ID: 2, Symbol: "EMPTY", Type: market.Equity}},
		map[string][]market.Bar{"BREAK": bars}, map[string][]string{"NIFTY50": {"BREAK", "EMPTY"}})
	srv := httptest.NewServer(New(prov, Config{}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBacktestReturnsTradesStatsAndHonesty(t *testing.T) {
	status, body := do(t, backtestServer(t), "POST", "/api/backtest",
		`{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}]}}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		System struct {
			Exits struct {
				StopPct float64 `json:"stop_pct"`
			} `json:"exits"`
		} `json:"system"`
		Trades []struct {
			Symbol     string  `json:"symbol"`
			SignalDate string  `json:"signal_date"`
			EntryDate  string  `json:"entry_date"`
			ExitDate   string  `json:"exit_date"`
			EntryPrice float64 `json:"entry_price"`
			ExitPrice  float64 `json:"exit_price"`
			ExitReason string  `json:"exit_reason"`
		} `json:"trades"`
		TradeStats struct {
			Trades int `json:"trades"`
		} `json:"trade_stats"`
		Honesty struct {
			FillModel    string `json:"fill_model"`
			Survivorship bool   `json:"survivorship"`
			Skipped      []struct {
				Symbol string `json:"symbol"`
				Reason string `json:"reason"`
			} `json:"skipped"`
		} `json:"honesty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.System.Exits.StopPct != 8 {
		t.Errorf("system defaults not filled: %s", body)
	}
	if len(got.Trades) != 1 || got.TradeStats.Trades != 1 {
		t.Fatalf("want one trade, got %s", body)
	}
	tr := got.Trades[0]
	if tr.Symbol != "BREAK" || tr.SignalDate != string(date(795)) || tr.EntryDate != string(date(796)) ||
		tr.ExitReason != "open" || tr.ExitPrice != 131 || math.Abs(tr.EntryPrice-131.131) > 1e-9 {
		t.Errorf("trade %+v", tr)
	}
	h := got.Honesty
	if h.FillModel != "next-day open" || !h.Survivorship || len(h.Skipped) != 1 || h.Skipped[0].Symbol != "EMPTY" || h.Skipped[0].Reason != "no_data" {
		t.Errorf("honesty %+v", h)
	}
}

func TestBacktestUnknownUniverse404(t *testing.T) {
	status, body := do(t, backtestServer(t), "POST", "/api/backtest",
		`{"name":"x","universe":"NOPE","entry":{"all":[{"type":"n_year_high"}]}}`)
	if status != 404 {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestDefaultSystemEndpoint(t *testing.T) {
	status, body := do(t, backtestServer(t), "GET", "/api/default-system", "")
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		System struct {
			Universe string `json:"universe"`
			Entry    struct {
				All []struct {
					Type string `json:"type"`
				} `json:"all"`
			} `json:"entry"`
		} `json:"system"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.System.Universe != "NIFTY50" || len(got.System.Entry.All) != 4 || got.System.Entry.All[0].Type != "n_year_high" {
		t.Fatalf("got %s", body)
	}
}
