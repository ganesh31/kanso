package patterns

// VolumeSurge passes when today's volume is at least k times the mean
// volume of the prior window (today excluded), spec §5.3.
type VolumeSurge struct{}

// Meta implements Pattern.
func (VolumeSurge) Meta() Meta {
	return Meta{
		Type:        TypeVolumeSurge,
		Label:       "Volume surge",
		Description: "Volume at least k times the average volume of the previous weeks (today excluded).",
		Params: []ParamSpec{
			{Name: "k", Label: "Multiple of average volume", Kind: KindNumber, Default: 2.0, Min: 1, Max: 10},
			{Name: "window_weeks", Label: "Average over (weeks)", Kind: KindNumber, Unit: Weeks, Default: 10.0, Min: 1, Max: 52, Step: 1},
		},
		Provides: []string{},
		Requires: []string{},
	}
}

// Prepare builds prefix sums of volume (and of non-zero-volume bars, so an
// all-zero window is detected exactly) once per context, in O(n). Any
// window's mean is then O(1).
func (VolumeSurge) Prepare(ctx *Context, _ Params) error {
	bars := ctx.Series.Bars
	if len(ctx.volSum) == len(bars)+1 {
		return nil
	}
	ctx.volSum = make([]float64, len(bars)+1)
	ctx.volPos = make([]int, len(bars)+1)
	for i, b := range bars {
		ctx.volSum[i+1] = ctx.volSum[i] + b.Volume
		ctx.volPos[i+1] = ctx.volPos[i]
		if b.Volume > 0 {
			ctx.volPos[i+1]++
		}
	}
	return nil
}

// Eval implements Pattern.
func (VolumeSurge) Eval(ctx *Context, p Params, t int) Result {
	s := ctx.Series
	if t < 0 || t >= s.Complete {
		return NotEvaluableResult(ReasonIncompleteBar)
	}
	w := ToBars(p.Float("window_weeks"), Weeks, s.Timeframe)
	if w < 1 || t < w {
		return NotEvaluableResult(ReasonInsufficientHistory)
	}
	if s.GapsIn(t-w, t) > 0 {
		return NotEvaluableResult(ReasonDataGap)
	}
	if ctx.volPos[t]-ctx.volPos[t-w] == 0 {
		return NotEvaluableResult(ReasonZeroDenominator)
	}
	avg := (ctx.volSum[t] - ctx.volSum[t-w]) / float64(w)
	if avg <= 0 {
		return NotEvaluableResult(ReasonZeroDenominator)
	}
	ratio := s.Bars[t].Volume / avg
	return Result{
		Pass:      ratio >= p.Float("k"),
		Evaluable: true,
		Evidence:  VolumeEvidence{Type: TypeVolumeSurge, Ratio: ratio, AvgVolume: avg},
	}
}

var _ Pattern = VolumeSurge{}
