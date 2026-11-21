package patterns

import (
	"context"
	"fmt"

	"kanso/internal/market"
)

// Context is the shared state patterns evaluate against: the series, the
// benchmark aligned to it, and values that providers publish for others
// (the breakout level).
type Context struct {
	Series *market.Series
	// Bench is the benchmark series, or nil when none was loaded.
	Bench *market.Series
	// BenchIdx[i] is the index of the bench bar with the same date as
	// Series bar i, or -1.
	BenchIdx []int
	// Level and LevelIdx are filled in by the BreakoutLevel provider.
	// LevelIdx[t] == -1 means there is no level at t.
	Level    []float64
	LevelIdx []int

	// volSum[i] is the sum of volumes of bars 0 … i−1 and volPos[i] the
	// count of those with non-zero volume (filled by VolumeSurge.Prepare).
	volSum []float64
	volPos []int
}

// NewContext aligns a series with its benchmark (which may be nil).
func NewContext(s, bench *market.Series) *Context {
	idx := make([]int, len(s.Bars))
	j := 0
	for i, b := range s.Bars {
		idx[i] = -1
		if bench == nil {
			continue
		}
		for j < len(bench.Bars) && bench.Bars[j].Date < b.Date {
			j++
		}
		if j < len(bench.Bars) && bench.Bars[j].Date == b.Date {
			idx[i] = j
		}
	}
	return &Context{Series: s, Bench: bench, BenchIdx: idx}
}

// Load is the one loading path for backtest, scan, report and the API. It
// loads the instrument's daily bars in [from, to] and builds the
// evaluation context on timeframe tf. benchDaily may be nil.
//
// Only the daily timeframe is supported so far; weekly resampling and
// benchmark gap marking arrive with their own tickets.
func Load(ctx context.Context, prov market.Provider, inst market.Instrument, benchDaily *market.Series,
	tf market.Timeframe, from, to market.Date,
) (pctx *Context, daily *market.Series, err error) {
	if tf != market.Daily {
		return nil, nil, fmt.Errorf("timeframe %q is not supported yet", tf)
	}
	bars, err := prov.Daily(ctx, inst.ID, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", inst.Symbol, err)
	}
	daily = market.NewDaily(inst, bars)
	return NewContext(daily, benchDaily), daily, nil
}
