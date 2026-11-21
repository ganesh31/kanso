package patterns

// LevelAge is the old-high-age filter (spec §5.3, V0 finding): it passes
// when the shared breakout level was set at least min_age_years ago, so
// continuation highs (an old high only days old) are skipped and only base
// breakouts remain. Optional: not part of the default system.
type LevelAge struct{}

// Meta implements Pattern.
func (LevelAge) Meta() Meta {
	return Meta{
		Type:        TypeLevelAge,
		Label:       "Old high age",
		Description: "The breakout level was set at least N years ago (a base breakout, not a continuation high).",
		Params: []ParamSpec{
			{Name: "min_age_years", Label: "Min age (years)", Kind: KindNumber, Unit: Years, Default: 1.0, Min: 0.25, Max: 10, Step: 0.25},
		},
		Provides: []string{},
		Requires: []string{BreakoutLevel},
	}
}

// Prepare implements Pattern; it reads the level a provider published.
func (LevelAge) Prepare(*Context, Params) error { return nil }

// Eval implements Pattern.
func (LevelAge) Eval(ctx *Context, p Params, t int) Result {
	s := ctx.Series
	if t < 0 || t >= s.Complete {
		return NotEvaluableResult(ReasonIncompleteBar)
	}
	idx, ok := levelAt(ctx, t)
	if !ok {
		return NotEvaluableResult(ReasonNoBreakoutLevel)
	}
	since := t - idx
	return Result{
		Pass:      since >= ToBars(p.Float("min_age_years"), Years, s.Timeframe),
		Evaluable: true,
		Evidence:  LevelAgeEvidence{Type: TypeLevelAge, BarsSince: since, LevelDate: s.Bars[idx].Date},
	}
}

var _ Pattern = LevelAge{}
