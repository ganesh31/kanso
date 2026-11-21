package patterns

// NotExtended passes when the close is at most pct percent above the shared
// breakout level, so a breakout isn't bought after it has already run
// (spec §5.3).
type NotExtended struct{}

// Meta implements Pattern.
func (NotExtended) Meta() Meta {
	return Meta{
		Type:        TypeNotExtended,
		Label:       "Not extended",
		Description: "Close at most x% above the breakout level.",
		Params: []ParamSpec{
			{Name: "pct", Label: "Max % above level", Kind: KindNumber, Default: 8.0, Min: 0, Max: 50},
		},
		Provides: []string{},
		Requires: []string{BreakoutLevel},
	}
}

// Prepare implements Pattern; it reads the level a provider published.
func (NotExtended) Prepare(*Context, Params) error { return nil }

// Eval implements Pattern.
func (NotExtended) Eval(ctx *Context, p Params, t int) Result {
	s := ctx.Series
	if t < 0 || t >= s.Complete {
		return NotEvaluableResult(ReasonIncompleteBar)
	}
	if _, ok := levelAt(ctx, t); !ok {
		return NotEvaluableResult(ReasonNoBreakoutLevel)
	}
	level := ctx.Level[t]
	if level <= 0 {
		return NotEvaluableResult(ReasonZeroDenominator)
	}
	c := s.Bars[t].Close
	return Result{
		Pass:      c <= level*(1+p.Float("pct")/100),
		Evaluable: true,
		Evidence:  NotExtendedEvidence{Type: TypeNotExtended, PctAbove: (c/level - 1) * 100},
	}
}

// levelAt returns the index of the bar that set the shared breakout level
// at t, or false when no provider published one there.
func levelAt(ctx *Context, t int) (int, bool) {
	if t >= len(ctx.LevelIdx) || ctx.LevelIdx[t] < 0 {
		return -1, false
	}
	return ctx.LevelIdx[t], true
}

var _ Pattern = NotExtended{}
