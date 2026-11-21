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
	// Series bar i (daily) or in the same ISO week (weekly), or -1.
	BenchIdx []int
	// Level and LevelIdx are filled in by the BreakoutLevel provider.
	// LevelIdx[t] == -1 means there is no level at t.
	Level    []float64
	LevelIdx []int
}

// NewContext aligns a series with its benchmark (which may be nil). Both
// must be on the same timeframe: daily bars align by date, weekly bars by
// ISO week (a stock's week may end on a different day than the bench's).
func NewContext(s, bench *market.Series) *Context {
	key := func(d market.Date) market.Date { return d }
	if s.Timeframe == market.Weekly {
		key = isoWeekKey
	}
	idx := make([]int, len(s.Bars))
	j := 0
	for i, b := range s.Bars {
		idx[i] = -1
		if bench == nil {
			continue
		}
		k := key(b.Date)
		for j < len(bench.Bars) && key(bench.Bars[j].Date) < k {
			j++
		}
		if j < len(bench.Bars) && key(bench.Bars[j].Date) == k {
			idx[i] = j
		}
	}
	return &Context{Series: s, Bench: bench, BenchIdx: idx}
}

// isoWeekKey maps a date to a sortable "YYYY-Www" ISO-week key.
func isoWeekKey(d market.Date) market.Date {
	y, w := d.ISOWeek()
	return market.Date(fmt.Sprintf("%04d-W%02d", y, w))
}

// Load is the one loading path for backtest, scan, report and the API. It
// loads the instrument's daily bars in [from, to] and builds the
// evaluation context on timeframe tf. benchDaily may be nil. On weekly,
// both the stock and the bench are resampled to ISO weeks; daily is always
// the daily series (fills and exits run on it).
func Load(ctx context.Context, prov market.Provider, inst market.Instrument, benchDaily *market.Series,
	tf market.Timeframe, from, to market.Date,
) (pctx *Context, daily *market.Series, err error) {
	if tf != market.Daily && tf != market.Weekly {
		return nil, nil, fmt.Errorf("unknown timeframe %q", tf)
	}
	bars, err := prov.Daily(ctx, inst.ID, from, to)
	if err != nil {
		return nil, nil, fmt.Errorf("load %s: %w", inst.Symbol, err)
	}
	daily = market.NewDaily(inst, bars)
	if tf == market.Daily {
		return NewContext(daily, benchDaily), daily, nil
	}
	var bench *market.Series
	if benchDaily != nil {
		bench = market.ToWeekly(benchDaily)
	}
	return NewContext(market.ToWeekly(daily), bench), daily, nil
}
