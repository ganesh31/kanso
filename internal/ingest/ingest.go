// Package ingest loads market data from a Source into the store: the full
// backfill of a stock list and its benchmark, and the nightly update
// (spec §6.1, §6.4).
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"

	"kanso/internal/data"
	"kanso/internal/market"
)

// Ingest log statuses.
const (
	StatusOK    = "ok"
	StatusError = "error"
)

// maxFailedFraction is the share of symbols that may fail before the run
// as a whole fails.
const maxFailedFraction = 0.05

// Options configures a run.
type Options struct {
	// Universe names the stock list seeded from Symbols.
	Universe string
	// Symbols are bare NSE symbols ("TCS"), e.g. from ReadNSEConstituents.
	Symbols []string
	// From and To bound the dates fetched, inclusive. An update fetches a
	// recent window ending at To; From is where a full history starts.
	From, To market.Date
	// Log receives one line per symbol; nil discards.
	Log *slog.Logger
}

// Summary is the outcome of a run.
type Summary struct {
	// Instruments is the number of symbols attempted, the benchmark
	// included.
	Instruments int `json:"instruments"`
	// Bars is the number of bars written.
	Bars int `json:"bars"`
	// Failed lists the symbols that wrote nothing, in run order.
	Failed []string `json:"failed"`
	// AsOf is the benchmark's latest stored date after the run.
	AsOf market.Date `json:"as_of"`
	// Behind lists the stock list's members whose latest bar is older than
	// AsOf, by symbol.
	Behind []string `json:"behind"`
}

// TooManyFailures reports whether more than 5% of symbols failed.
func (s Summary) TooManyFailures() bool {
	return s.Instruments > 0 && float64(len(s.Failed)) > maxFailedFraction*float64(s.Instruments)
}

// Backfill seeds the universe from opts.Symbols (membership start unknown)
// and fetches every symbol's history, the benchmark ^NSEI first as an
// index. The fetch starts at the earlier of From and the symbol's first
// stored bar and replaces its stored history, so a rerun with a later From
// drops nothing and every bar is on the source's current adjustment basis.
// Each symbol gets an ingest_log entry; a symbol whose fetch fails or
// returns no bars writes nothing and is listed in Summary.Failed. The
// error is only for store failures that stop the run.
func Backfill(ctx context.Context, st *data.Store, src data.Source, opts Options) (Summary, error) {
	return run(ctx, st, opts, func(in market.Instrument) (int, string, error) {
		n, from, err := fetchFull(ctx, st, src, in, opts.From, opts.To)
		return n, fmt.Sprintf("%d bars %s..%s", n, from, opts.To), err
	})
}

// Update is the nightly incremental run (spec §6.1). For each symbol it
// fetches from its last stored date minus updateOverlapDays to opts.To and
// upserts that window. A symbol with no stored bars gets its full history
// from opts.From. Seeding, logging and failures are as in Backfill.
func Update(ctx context.Context, st *data.Store, src data.Source, opts Options) (Summary, error) {
	return run(ctx, st, opts, func(in market.Instrument) (int, string, error) {
		latest, err := st.LatestBarDate(ctx, in.ID)
		if err != nil {
			return 0, "", fmt.Errorf("%w: %w", errStore, err)
		}
		if latest == "" {
			n, _, err := fetchFull(ctx, st, src, in, opts.From, opts.To)
			return n, fmt.Sprintf("%d bars %s..%s (new)", n, opts.From, opts.To), err
		}
		from := market.Date(latest.Time().AddDate(0, 0, -updateOverlapDays).Format(market.DateLayout))
		bars, splits, err := fetch(ctx, src, in, from, opts.To)
		if err != nil {
			return 0, "", err
		}
		stored, err := st.Daily(ctx, in.ID, from, opts.To)
		if err != nil {
			return 0, "", fmt.Errorf("%w: %w", errStore, err)
		}
		if reason := refetchReason(from, splits, stored, bars); reason != "" {
			n, fullFrom, err := fetchFull(ctx, st, src, in, opts.From, opts.To)
			if err != nil {
				return 0, "", fmt.Errorf("refetch after %s: %w", reason, err)
			}
			return n, fmt.Sprintf("refetched: %s; %d bars %s..%s", reason, n, fullFrom, opts.To), nil
		}
		if err := st.UpsertDaily(ctx, in.ID, bars, splits); err != nil {
			return 0, "", fmt.Errorf("%w: %w", errStore, err)
		}
		return len(bars), fmt.Sprintf("%d bars %s..%s", len(bars), from, opts.To), nil
	})
}

// Refetch reasons, logged as "refetched: <reason>".
const (
	reasonSplit    = "split"
	reasonRestated = "restated"
)

// maxRestatement is the largest relative change of a stored close that an
// update still treats as the same history.
const maxRestatement = 0.005

// refetchReason says why an update window starting at from can't simply
// be upserted: the source reported a split in it, or a stored close in
// the overlap differs from the fetched one by more than 0.5% (a bonus or
// a silent restatement). Either way the source has rescaled history the
// window doesn't cover. "" means the window can be upserted.
func refetchReason(from market.Date, splits []data.Split, stored, fetched []market.Bar) string {
	for _, sp := range splits {
		if sp.Date >= from {
			return reasonSplit
		}
	}
	old := make(map[market.Date]float64, len(stored))
	for _, b := range stored {
		old[b.Date] = b.Close
	}
	for _, b := range fetched {
		if c, ok := old[b.Date]; ok && math.Abs(b.Close-c)/c > maxRestatement {
			return reasonRestated
		}
	}
	return ""
}

// updateOverlapDays is how many calendar days before the last stored bar
// an update refetches, so restated recent bars are seen and compared.
const updateOverlapDays = 10

// ingestFunc fetches and stores one instrument, returning the bars written
// and the ingest log message. An error wrapping errStore stops the run.
type ingestFunc func(in market.Instrument) (int, string, error)

func run(ctx context.Context, st *data.Store, opts Options, ingestOne ingestFunc) (Summary, error) {
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	insts := []market.Instrument{{Exchange: "NSE", Symbol: market.Benchmark, Type: market.Index, LotSize: 1}}
	for _, sym := range opts.Symbols {
		if sym != market.Benchmark {
			insts = append(insts, market.Instrument{Exchange: "NSE", Symbol: sym, Type: market.Equity, LotSize: 1})
		}
	}
	var members []int64
	for i, in := range insts {
		id, err := st.UpsertInstrument(ctx, in)
		if err != nil {
			return Summary{}, err
		}
		insts[i].ID = id
		if in.Type == market.Equity {
			members = append(members, id)
		}
	}
	if err := st.ReplaceUniverse(ctx, opts.Universe, members); err != nil {
		return Summary{}, fmt.Errorf("seed universe %s: %w", opts.Universe, err)
	}

	sum := Summary{Instruments: len(insts), Failed: []string{}}
	for _, in := range insts {
		if err := ctx.Err(); err != nil {
			return sum, err
		}
		n, msg, fetchErr := ingestOne(in)
		status := StatusOK
		if fetchErr != nil {
			if errors.Is(fetchErr, errStore) {
				return sum, fetchErr
			}
			status, msg = StatusError, fetchErr.Error()
			sum.Failed = append(sum.Failed, in.Symbol)
			log.Warn("ingest failed", "symbol", in.Symbol, "err", fetchErr)
		} else {
			sum.Bars += n
			log.Info("ingested", "symbol", in.Symbol, "bars", n, "detail", msg)
		}
		if err := st.LogIngest(ctx, in.ID, status, msg); err != nil {
			return sum, fmt.Errorf("log ingest %s: %w", in.Symbol, err)
		}
	}
	var err error
	if sum.AsOf, sum.Behind, err = Behind(ctx, st, opts.Universe); err != nil {
		return sum, fmt.Errorf("freshness: %w", err)
	}
	return sum, nil
}

// errStore marks a store write failure, which stops the run.
var errStore = errors.New("store")

// fetch gets one instrument's bars in [from, to]; an empty response is an
// error.
func fetch(ctx context.Context, src data.Source, in market.Instrument, from, to market.Date) ([]market.Bar, []data.Split, error) {
	bars, splits, err := src.Daily(ctx, YahooTicker(in), from, to)
	if err != nil {
		return nil, nil, err
	}
	if len(bars) == 0 {
		return nil, nil, errors.New("no bars returned")
	}
	return bars, splits, nil
}

// fetchFull fetches an instrument's history from the earlier of from and
// its first stored bar to to, and atomically replaces its stored bars with
// it, so nothing is dropped and every bar is on the source's current
// adjustment basis. It returns the bars written and the start fetched.
func fetchFull(ctx context.Context, st *data.Store, src data.Source, in market.Instrument, from, to market.Date) (int, market.Date, error) {
	earliest, err := st.EarliestBarDate(ctx, in.ID)
	if err != nil {
		return 0, "", fmt.Errorf("%w: %w", errStore, err)
	}
	if earliest != "" && earliest < from {
		from = earliest
	}
	bars, splits, err := fetch(ctx, src, in, from, to)
	if err != nil {
		return 0, from, err
	}
	if err := st.ReplaceDaily(ctx, in.ID, bars, splits); err != nil {
		return 0, from, fmt.Errorf("%w: %w", errStore, err)
	}
	return len(bars), from, nil
}

// YahooTicker is the Yahoo symbol of an NSE instrument: indices ("^NSEI")
// as they are, equities with the ".NS" suffix.
func YahooTicker(in market.Instrument) string {
	if strings.HasPrefix(in.Symbol, "^") {
		return in.Symbol
	}
	return in.Symbol + ".NS"
}
