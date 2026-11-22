package patterns

import "kanso/internal/market"

// DefaultBenchmark is the benchmark symbol relative strength is measured
// against by default: Nifty 50 (spec §5.3).
const DefaultBenchmark = market.Benchmark

// RelativeStrength passes when the stock's return over the last rs_months
// beats the benchmark's over the same bars (spec §5.3). The benchmark is
// ctx.Bench: whoever builds the context loads the series named by the
// benchmark param.
type RelativeStrength struct{}

// Meta implements Pattern.
func (RelativeStrength) Meta() Meta {
	return Meta{
		Type:        TypeRelativeStrength,
		Label:       "Relative strength",
		Description: "Return over the last N months beats the benchmark's.",
		Params: []ParamSpec{
			{Name: "rs_months", Label: "Over (months)", Kind: KindNumber, Unit: Months, Default: 6.0, Min: 1, Max: 24, Step: 1},
			{Name: "benchmark", Label: "Benchmark", Kind: KindSymbol, Default: DefaultBenchmark},
		},
		Provides: []string{},
		Requires: []string{},
	}
}

// Prepare implements Pattern; relative strength needs no precomputation.
func (RelativeStrength) Prepare(*Context, Params) error { return nil }

// Eval implements Pattern.
func (RelativeStrength) Eval(ctx *Context, p Params, t int) Result {
	ev, reason := rs(ctx, p.Float("rs_months"), t)
	if reason != "" {
		return NotEvaluableResult(reason)
	}
	return Result{Pass: ev.Diff > 0, Evaluable: true, Evidence: ev}
}

// RSDiff is the stock's return minus the benchmark's over the last months
// at bar t, for ranking (spec §5.6). ok is false when it is not evaluable.
func RSDiff(ctx *Context, months float64, t int) (diff float64, ok bool) {
	ev, reason := rs(ctx, months, t)
	return ev.Diff, reason == ""
}

// rs computes the evidence, or the reason it can't. Both ends of the
// window need a complete benchmark bar on the same date (or ISO week) as
// the stock's bar, and the stock's window must have no data gap.
func rs(ctx *Context, months float64, t int) (RSEvidence, Reason) {
	s := ctx.Series
	if t < 0 || t >= s.Complete {
		return RSEvidence{}, ReasonIncompleteBar
	}
	n := ToBars(months, Months, s.Timeframe)
	if n < 1 || t < n {
		return RSEvidence{}, ReasonInsufficientHistory
	}
	if s.GapsIn(t-n, t) > 0 {
		return RSEvidence{}, ReasonDataGap
	}
	// A provisional benchmark week can share an ISO week with a complete
	// stock week that ends later: its close isn't the week's close yet.
	if ctx.Bench == nil || ctx.BenchIdx[t] < 0 || ctx.BenchIdx[t-n] < 0 || ctx.BenchIdx[t] >= ctx.Bench.Complete {
		return RSEvidence{}, ReasonBenchmarkMissing
	}
	s0, s1 := s.Bars[t-n].Close, s.Bars[t].Close
	b0, b1 := ctx.Bench.Bars[ctx.BenchIdx[t-n]].Close, ctx.Bench.Bars[ctx.BenchIdx[t]].Close
	if s0 <= 0 || b0 <= 0 {
		return RSEvidence{}, ReasonZeroDenominator
	}
	sr, br := s1/s0-1, b1/b0-1
	return RSEvidence{Type: TypeRelativeStrength, StockReturn: sr, BenchReturn: br, Diff: sr - br}, ""
}

var _ Pattern = RelativeStrength{}
