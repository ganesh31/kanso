package patterns

// allTimeMinYears is the minimum prior history for the all-time mode, so
// newly listed stocks don't flood results (spec §5.2).
const allTimeMinYears = 2

// NYearHigh is the new N-year high (spec §5.2). The breakout level at bar
// t is the highest high of bars t−L … t−1 (today excluded); it passes when
// close(t) > level(t). It provides BreakoutLevel to other conditions.
type NYearHigh struct{}

// Meta implements Pattern.
func (NYearHigh) Meta() Meta {
	return Meta{
		Type:        TypeNYearHigh,
		Label:       "New N-year high",
		Description: "Close above the highest high of the previous N years (today's high excluded).",
		Params: []ParamSpec{
			{Name: "years", Label: "Years", Kind: KindNumber, Unit: Years, Default: 3.0, Min: 1, Max: 20, Step: 1},
			{Name: "all_time", Label: "All-time (at least 2 years of history)", Kind: KindBool, Default: false},
		},
		Provides: []string{BreakoutLevel},
		Requires: []string{},
	}
}

// Prepare fills ctx.Level and ctx.LevelIdx for every bar in O(n): a rolling
// max over the prior L highs with a monotonic deque, or a running max for
// all-time. On equal highs the earliest bar is the one that set the level.
func (NYearHigh) Prepare(ctx *Context, p Params) error {
	bars := ctx.Series.Bars
	n := len(bars)
	tf := ctx.Series.Timeframe
	ctx.Level = make([]float64, n)
	ctx.LevelIdx = make([]int, n)

	if p.Bool("all_time") {
		minBars := ToBars(allTimeMinYears, Years, tf)
		best := -1
		for t := range n {
			ctx.LevelIdx[t] = -1
			if best >= 0 && t >= minBars {
				ctx.Level[t], ctx.LevelIdx[t] = bars[best].High, best
			}
			if best < 0 || bars[t].High > bars[best].High {
				best = t
			}
		}
		return nil
	}

	l := ToBars(p.Float("years"), Years, tf)
	// deque holds indexes of the window t-L … t-1 with strictly decreasing
	// highs, oldest first; its front is the earliest maximum.
	deque := make([]int, 0, l)
	for t := range n {
		ctx.LevelIdx[t] = -1
		if t >= l && l > 0 {
			for len(deque) > 0 && deque[0] < t-l {
				deque = deque[1:]
			}
			ctx.Level[t], ctx.LevelIdx[t] = bars[deque[0]].High, deque[0]
		}
		for len(deque) > 0 && bars[deque[len(deque)-1]].High < bars[t].High {
			deque = deque[:len(deque)-1]
		}
		deque = append(deque, t)
	}
	return nil
}

// Eval implements Pattern in O(1) after Prepare. A data gap inside the
// lookback window (or anywhere in the history for all-time) makes the bar
// not evaluable.
func (NYearHigh) Eval(ctx *Context, p Params, t int) Result {
	s := ctx.Series
	if t < 0 || t >= s.Complete {
		return NotEvaluableResult(ReasonIncompleteBar)
	}
	idx := ctx.LevelIdx[t]
	if idx < 0 {
		return NotEvaluableResult(ReasonInsufficientHistory)
	}
	start := 0
	if !p.Bool("all_time") {
		start = t - ToBars(p.Float("years"), Years, s.Timeframe)
	}
	if s.GapsIn(start, t) > 0 {
		return NotEvaluableResult(ReasonDataGap)
	}
	level := ctx.Level[t]
	if level <= 0 {
		return NotEvaluableResult(ReasonZeroDenominator)
	}
	c := s.Bars[t].Close
	return Result{
		Pass:      c > level,
		Evaluable: true,
		Evidence: NYearHighEvidence{
			Type:      TypeNYearHigh,
			Level:     level,
			LevelDate: s.Bars[idx].Date,
			BarsSince: t - idx,
			Close:     c,
			PctAbove:  (c/level - 1) * 100,
		},
	}
}

var _ Pattern = NYearHigh{}
