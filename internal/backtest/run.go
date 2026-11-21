package backtest

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sort"

	"golang.org/x/sync/errgroup"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// Honesty-panel wording (spec §5.8).
const (
	FillModel  = "next-day open"
	DataSource = "Yahoo Finance"
	Adjustment = "split-adjusted, not dividend-adjusted"
)

// Skip reasons for instruments.
const (
	SkipNoData = "no_data"
)

// rankMonths is the RS window that ranks crowded entry days (spec §5.6).
const rankMonths = 6

// SkippedInstrument is a stock list member the backtest could not use.
type SkippedInstrument struct {
	Symbol string
	Reason string
}

// Honesty is what the honesty panel shows next to the results (spec §5.8).
type Honesty struct {
	FillModel   string
	PerSidePct  float64
	SlippagePct float64
	// Survivorship is true when the stock list is today's members only.
	Survivorship bool
	DataSource   string
	Adjustment   string
	// DataStart and DataEnd are the window actually used: the start is
	// the later of the requested start and the benchmark's first date.
	DataStart market.Date
	DataEnd   market.Date
	Skipped   []SkippedInstrument
}

// Result is a backtest's output.
type Result struct {
	// System is the system run, with defaults and params filled in.
	System rules.System
	// AllTrades are single-stock trades (no slot limits) for every
	// instrument, by symbol then entry date.
	AllTrades  []Trade
	TradeStats TradeStats
	Honesty    Honesty
}

// Run backtests sys over its stock list between from and to (either may
// be "": all data). Bars before the start are still loaded for lookback.
// An invalid system returns a *rules.ValidationError and an unknown stock
// list wraps market.ErrNotFound.
func Run(ctx context.Context, prov market.Provider, sys rules.System, from, to market.Date) (*Result, error) {
	rules.ApplyDefaults(&sys)
	reg := patterns.Default()
	if err := rules.Validate(&sys, reg); err != nil {
		return nil, err
	}
	compiled, err := rules.Compile(sys.Entry, reg)
	if err != nil {
		return nil, err
	}
	members, hasHistory, err := prov.Universe(ctx, sys.Universe)
	if err != nil {
		return nil, err
	}
	if to == "" {
		to = "9999-12-31"
	}
	bench, err := loadBench(ctx, prov, benchmarkOf(sys.Entry), to)
	if err != nil {
		return nil, err
	}
	start := from
	if bench != nil && len(bench.Bars) > 0 && bench.Bars[0].Date > start {
		start = bench.Bars[0].Date
	}

	ex := EquityExecutor{SlippagePct: sys.Costs.SlippagePct}
	cm := PercentCost{PerSidePct: sys.Costs.PerSidePct}
	outs := make([]instOut, len(members))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.GOMAXPROCS(0))
	for i, inst := range members {
		g.Go(func() error {
			o, err := runOne(gctx, prov, inst, bench, compiled, &sys, start, to, ex, cm)
			outs[i] = o
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	sort.SliceStable(outs, func(i, j int) bool { return outs[i].symbol < outs[j].symbol })

	res := &Result{System: sys, AllTrades: []Trade{}, Honesty: Honesty{
		FillModel: FillModel, PerSidePct: sys.Costs.PerSidePct, SlippagePct: sys.Costs.SlippagePct,
		Survivorship: !hasHistory, DataSource: DataSource, Adjustment: Adjustment,
		DataStart: start, Skipped: []SkippedInstrument{},
	}}
	for _, o := range outs {
		if o.skip != "" {
			res.Honesty.Skipped = append(res.Honesty.Skipped, SkippedInstrument{Symbol: o.symbol, Reason: o.skip})
			continue
		}
		res.AllTrades = append(res.AllTrades, o.trades...)
		res.Honesty.DataEnd = max(res.Honesty.DataEnd, o.last)
	}
	if bench != nil && len(bench.Bars) > 0 {
		res.Honesty.DataEnd = bench.Bars[len(bench.Bars)-1].Date
	}
	res.TradeStats = ComputeTradeStats(res.AllTrades)
	return res, nil
}

// instOut is one instrument's part of a run.
type instOut struct {
	symbol string
	// skip is the reason the instrument was not used, or "".
	skip   string
	trades []Trade
	// last is the date of its last bar.
	last market.Date
}

// runOne loads one instrument with all its history up to to and follows
// its signals dated from start on, single-stock style.
func runOne(ctx context.Context, prov market.Provider, inst market.Instrument, bench *market.Series,
	c *rules.Compiled, sys *rules.System, start, to market.Date, ex Executor, cm CostModel,
) (instOut, error) {
	out := instOut{symbol: inst.Symbol}
	pctx, daily, err := patterns.Load(ctx, prov, inst, bench, sys.Timeframe, "", to)
	if err != nil {
		return out, err
	}
	if len(daily.Bars) == 0 {
		out.skip = SkipNoData
		return out, nil
	}
	out.last = daily.Bars[len(daily.Bars)-1].Date
	if err := c.Prepare(pctx); err != nil {
		return out, fmt.Errorf("%s: %w", inst.Symbol, err)
	}
	var sigs []int
	for _, s := range Signals(c, pctx) {
		if pctx.Series.Bars[s].Date >= start {
			sigs = append(sigs, s)
		}
	}
	out.trades = singleStockTrades(sigs, pctx, daily, sys, ex, cm)
	return out, nil
}

// rankKey is the 6-month RS diff at signal bar sig, whether or not RS is
// in the rule, or -Inf when it is not evaluable (spec §5.6).
func rankKey(pctx *patterns.Context, sig int) float64 {
	if diff, ok := patterns.RSDiff(pctx, rankMonths, sig); ok {
		return diff
	}
	return math.Inf(-1)
}

// singleStockTrades follows signals in order, skipping any whose entry
// would come before the previous trade's exit (spec §5.5). Each trade
// carries its RankKey.
func singleStockTrades(sigs []int, ctx *patterns.Context, daily *market.Series, sys *rules.System, ex Executor, cm CostModel) []Trade {
	out := []Trade{}
	for _, s := range sigs {
		tr, ok := TradePath(s, ctx, daily, sys, ex, cm)
		if !ok {
			continue
		}
		if n := len(out); n > 0 && tr.EntryDate < out[n-1].ExitDate {
			continue
		}
		tr.RankKey = rankKey(ctx, s)
		out = append(out, tr)
	}
	return out
}

// benchmarkOf is the benchmark the rule's relative strength uses, or the
// default (Nifty 50).
func benchmarkOf(e rules.Entry) string {
	for _, c := range append(append([]rules.Condition{}, e.All...), e.Any...) {
		if c.Type == patterns.TypeRelativeStrength {
			if b := c.Params.String("benchmark"); b != "" {
				return b
			}
		}
	}
	return patterns.DefaultBenchmark
}

// loadBench loads the benchmark's daily bars up to to, or nil when the
// provider doesn't have it.
func loadBench(ctx context.Context, prov market.Provider, symbol string, to market.Date) (*market.Series, error) {
	inst, err := prov.Instrument(ctx, symbol)
	if errors.Is(err, market.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("benchmark %s: %w", symbol, err)
	}
	bars, err := prov.Daily(ctx, inst.ID, "", to)
	if err != nil {
		return nil, fmt.Errorf("benchmark %s: %w", symbol, err)
	}
	return market.NewDaily(inst, bars), nil
}
