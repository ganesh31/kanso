// Package ingest loads market data from a Source into the store: the full
// backfill of a stock list and its benchmark (spec §6.4).
package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// Options configures a backfill.
type Options struct {
	// Universe names the stock list seeded from Symbols.
	Universe string
	// Symbols are bare NSE symbols ("TCS"), e.g. from ReadNSEConstituents.
	Symbols []string
	// From and To bound the dates fetched, inclusive.
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
}

// TooManyFailures reports whether more than 5% of symbols failed.
func (s Summary) TooManyFailures() bool {
	return s.Instruments > 0 && float64(len(s.Failed)) > maxFailedFraction*float64(s.Instruments)
}

// Backfill seeds the universe from opts.Symbols (membership start unknown)
// and fetches every symbol's history, the benchmark ^NSEI first as an
// index. Each symbol gets an ingest_log entry; a symbol whose fetch fails
// or returns no bars writes nothing and is listed in Summary.Failed. The
// error is only for store failures that stop the run.
func Backfill(ctx context.Context, st *data.Store, src data.Source, opts Options) (Summary, error) {
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
		n, fetchErr := fetchOne(ctx, st, src, in, opts.From, opts.To)
		status, msg := StatusOK, fmt.Sprintf("%d bars %s..%s", n, opts.From, opts.To)
		if fetchErr != nil {
			if errors.Is(fetchErr, errStore) {
				return sum, fetchErr
			}
			status, msg = StatusError, fetchErr.Error()
			sum.Failed = append(sum.Failed, in.Symbol)
			log.Warn("ingest failed", "symbol", in.Symbol, "err", fetchErr)
		} else {
			sum.Bars += n
			log.Info("ingested", "symbol", in.Symbol, "bars", n)
		}
		if err := st.LogIngest(ctx, in.ID, status, msg); err != nil {
			return sum, fmt.Errorf("log ingest %s: %w", in.Symbol, err)
		}
	}
	return sum, nil
}

// errStore marks a store write failure, which stops the run.
var errStore = errors.New("store")

func fetchOne(ctx context.Context, st *data.Store, src data.Source, in market.Instrument, from, to market.Date) (int, error) {
	bars, splits, err := src.Daily(ctx, YahooTicker(in), from, to)
	if err != nil {
		return 0, err
	}
	if len(bars) == 0 {
		return 0, errors.New("no bars returned")
	}
	if err := st.UpsertBars(ctx, in.ID, bars); err != nil {
		return 0, fmt.Errorf("%w: %w", errStore, err)
	}
	if err := st.SaveSplits(ctx, in.ID, splits); err != nil {
		return 0, fmt.Errorf("%w: %w", errStore, err)
	}
	return len(bars), nil
}

// YahooTicker is the Yahoo symbol of an NSE instrument: indices ("^NSEI")
// as they are, equities with the ".NS" suffix.
func YahooTicker(in market.Instrument) string {
	if strings.HasPrefix(in.Symbol, "^") {
		return in.Symbol
	}
	return in.Symbol + ".NS"
}
