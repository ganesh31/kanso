package patterns

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"kanso/internal/market"
)

// eval normalizes p for pat, prepares ctx and evaluates bar at.
func eval(t *testing.T, pat Pattern, ctx *Context, p Params, at int) Result {
	t.Helper()
	np, err := Normalize(pat.Meta(), p)
	if err != nil {
		t.Fatal(err)
	}
	if err := pat.Prepare(ctx, np); err != nil {
		t.Fatal(err)
	}
	return pat.Eval(ctx, np, at)
}

// volumeSeries is a daily series with the given volumes (prices flat 100).
func volumeSeries(vols []float64) *market.Series {
	bars := make([]market.Bar, len(vols))
	for i, v := range vols {
		bars[i] = market.Bar{Date: date(i), Open: 100, High: 100, Low: 100, Close: 100, Volume: v}
	}
	return market.NewDaily(market.Instrument{ID: 1, Symbol: "TEST"}, bars)
}

// priorThenToday returns n prior volumes of prior followed by today's.
func priorThenToday(n int, prior, today float64) []float64 {
	v := make([]float64, n+1)
	for i := range n {
		v[i] = prior
	}
	v[n] = today
	return v
}

func TestVolumeSurgeExcludesCurrentBarFromAverage(t *testing.T) {
	r := eval(t, VolumeSurge{}, NewContext(volumeSeries(priorThenToday(50, 100, 200)), nil), nil, 50)
	want := Result{Pass: true, Evaluable: true, Evidence: VolumeEvidence{Type: TypeVolumeSurge, Ratio: 2, AvgVolume: 100}}
	if diff := cmp.Diff(want, r, cmpopts.EquateApprox(1e-9, 0)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestVolumeSurgeBelowK(t *testing.T) {
	r := eval(t, VolumeSurge{}, NewContext(volumeSeries(priorThenToday(50, 100, 199)), nil), nil, 50)
	if r.Pass || !r.Evaluable {
		t.Fatalf("got %+v, want evaluable fail", r)
	}
}

func TestVolumeSurgeWindowWeeksPerTimeframe(t *testing.T) {
	// An old 1000 at bar 0 is outside the 50-bar daily window of bar 59.
	vols := priorThenToday(59, 100, 200)
	vols[0] = 1000
	r := eval(t, VolumeSurge{}, NewContext(volumeSeries(vols), nil), Params{"window_weeks": 10.0}, 59)
	if ev := r.Evidence.(VolumeEvidence); ev.AvgVolume != 100 {
		t.Fatalf("daily avg = %v, want 100 over 50 bars", ev.AvgVolume)
	}

	// The same bars read as weekly: 10 weeks is 10 bars.
	weekly := volumeSeries(vols)
	weekly.Timeframe = market.Weekly
	ctx := NewContext(weekly, nil)
	if r := eval(t, VolumeSurge{}, ctx, nil, 9); r.Evaluable {
		t.Fatalf("weekly bar 9: got %+v, want insufficient history", r)
	}
	if r := eval(t, VolumeSurge{}, ctx, nil, 10); r.Evidence.(VolumeEvidence).AvgVolume != 190 {
		t.Fatalf("weekly bar 10: got %+v, want avg (1000 + 9×100)/10", r.Evidence)
	}
	if r := eval(t, VolumeSurge{}, ctx, nil, 11); r.Evidence.(VolumeEvidence).AvgVolume != 100 {
		t.Fatalf("weekly bar 11: got %+v, want avg 100", r.Evidence)
	}
}

// closeSeries is a daily series with the given closes, dated from bar
// offset on (so a benchmark can start later than the stock).
func closeSeries(symbol string, offset int, closes []float64) *market.Series {
	bars := make([]market.Bar, len(closes))
	for i, c := range closes {
		bars[i] = market.Bar{Date: date(offset + i), Open: c, High: c, Low: c, Close: c, Volume: 1000}
	}
	return market.NewDaily(market.Instrument{ID: 1, Symbol: symbol}, bars)
}

// ramp returns n+1 closes of 100, except 100×(1+ret) on the last bar.
func ramp(n int, ret float64) []float64 {
	c := make([]float64, n+1)
	for i := range c {
		c[i] = 100
	}
	c[n] = 100 * (1 + ret)
	return c
}

func TestRelativeStrengthPositiveDiff(t *testing.T) {
	ctx := NewContext(closeSeries("TEST", 0, ramp(126, 0.20)), closeSeries("^NSEI", 0, ramp(126, 0.10)))
	r := eval(t, RelativeStrength{}, ctx, nil, 126)
	want := Result{Pass: true, Evaluable: true, Evidence: RSEvidence{
		Type: TypeRelativeStrength, StockReturn: 0.20, BenchReturn: 0.10, Diff: 0.10,
	}}
	if diff := cmp.Diff(want, r, cmpopts.EquateApprox(1e-9, 0)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	d, ok := RSDiff(ctx, 6, 126)
	if !ok || math.Abs(d-0.10) > 1e-9 {
		t.Fatalf("RSDiff = %v, %v", d, ok)
	}
}

func TestRelativeStrengthEqualReturnsFail(t *testing.T) {
	ctx := NewContext(closeSeries("TEST", 0, ramp(126, 0.10)), closeSeries("^NSEI", 0, ramp(126, 0.10)))
	if r := eval(t, RelativeStrength{}, ctx, nil, 126); r.Pass || !r.Evaluable {
		t.Fatalf("got %+v, want evaluable fail (diff must be > 0)", r)
	}
}

func TestRelativeStrengthMissingBenchmarkBar(t *testing.T) {
	bench := closeSeries("^NSEI", 0, ramp(127, 0.10))
	bench.Bars = append(bench.Bars[:126:126], bench.Bars[127])
	ctx := NewContext(closeSeries("TEST", 0, ramp(127, 0.20)), bench)
	if ctx.BenchIdx[126] != -1 {
		t.Fatalf("setup: BenchIdx[126] = %d", ctx.BenchIdx[126])
	}
	r := eval(t, RelativeStrength{}, ctx, nil, 126)
	want := Result{Evidence: NotEvaluable{Type: TypeNotEvaluable, Reason: ReasonBenchmarkMissing}}
	if diff := cmp.Diff(want, r); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	// Other conditions still evaluate on that bar.
	if v := eval(t, VolumeSurge{}, ctx, nil, 126); !v.Evaluable {
		t.Fatalf("volume on the same bar: %+v", v)
	}
	if _, ok := RSDiff(ctx, 6, 126); ok {
		t.Fatal("RSDiff ok on a missing benchmark bar")
	}
}

func TestRelativeStrengthBeforeBenchmarkStart(t *testing.T) {
	// The benchmark starts at bar 10, after t−126 = 0.
	ctx := NewContext(closeSeries("TEST", 0, ramp(126, 0.20)), closeSeries("^NSEI", 10, ramp(116, 0.10)))
	r := eval(t, RelativeStrength{}, ctx, nil, 126)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonBenchmarkMissing {
		t.Fatalf("got %+v, want benchmark_missing", r)
	}
}

func TestRelativeStrengthWithoutBenchmark(t *testing.T) {
	r := eval(t, RelativeStrength{}, NewContext(closeSeries("TEST", 0, ramp(126, 0.20)), nil), nil, 126)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonBenchmarkMissing {
		t.Fatalf("got %+v, want benchmark_missing", r)
	}
}

func TestRelativeStrengthInsufficientHistory(t *testing.T) {
	ctx := NewContext(closeSeries("TEST", 0, ramp(126, 0.20)), closeSeries("^NSEI", 0, ramp(126, 0.10)))
	r := eval(t, RelativeStrength{}, ctx, nil, 125)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonInsufficientHistory {
		t.Fatalf("got %+v, want insufficient_history", r)
	}
}

// levelCtx is a context over closes whose shared breakout level at bar t
// is level, set at bar levelIdx (−1 for none), as a provider would leave it.
func levelCtx(closes []float64, t int, level float64, levelIdx int) *Context {
	ctx := NewContext(closeSeries("TEST", 0, closes), nil)
	ctx.Level = make([]float64, len(closes))
	ctx.LevelIdx = make([]int, len(closes))
	for i := range ctx.LevelIdx {
		ctx.LevelIdx[i] = -1
	}
	ctx.Level[t], ctx.LevelIdx[t] = level, levelIdx
	return ctx
}

func TestNotExtendedUsesSharedLevel(t *testing.T) {
	r := eval(t, NotExtended{}, levelCtx([]float64{90, 108}, 1, 100, 0), nil, 1)
	want := Result{Pass: true, Evaluable: true, Evidence: NotExtendedEvidence{Type: TypeNotExtended, PctAbove: 8}}
	if diff := cmp.Diff(want, r, cmpopts.EquateApprox(1e-9, 0)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	if r := eval(t, NotExtended{}, levelCtx([]float64{90, 108.01}, 1, 100, 0), nil, 1); r.Pass || !r.Evaluable {
		t.Fatalf("close 108.01: got %+v, want evaluable fail", r)
	}
}

func TestNotExtendedWithoutLevel(t *testing.T) {
	for name, ctx := range map[string]*Context{
		"no level at t":   levelCtx([]float64{90, 108}, 1, 0, -1),
		"no provider ran": NewContext(closeSeries("TEST", 0, []float64{90, 108}), nil),
	} {
		r := eval(t, NotExtended{}, ctx, nil, 1)
		want := Result{Evidence: NotEvaluable{Type: TypeNotEvaluable, Reason: ReasonNoBreakoutLevel}}
		if diff := cmp.Diff(want, r); diff != "" {
			t.Fatalf("%s (-want +got):\n%s", name, diff)
		}
	}
}

func TestFiltersZeroDenominators(t *testing.T) {
	zero := Result{Evidence: NotEvaluable{Type: TypeNotEvaluable, Reason: ReasonZeroDenominator}}
	cases := map[string]Result{
		"avg volume 0": eval(t, VolumeSurge{}, NewContext(volumeSeries(priorThenToday(50, 0, 500)), nil), nil, 50),
		"stock close[t-n] 0": eval(t, RelativeStrength{},
			NewContext(closeSeries("TEST", 0, append([]float64{0}, ramp(125, 0.2)...)), closeSeries("^NSEI", 0, ramp(126, 0.1))), nil, 126),
		"bench close[t-n] 0": eval(t, RelativeStrength{},
			NewContext(closeSeries("TEST", 0, ramp(126, 0.2)), closeSeries("^NSEI", 0, append([]float64{0}, ramp(125, 0.1)...))), nil, 126),
		"level 0": eval(t, NotExtended{}, levelCtx([]float64{0, 5}, 1, 0, 0), nil, 1),
	}
	for name, r := range cases {
		if diff := cmp.Diff(zero, r); diff != "" {
			t.Errorf("%s (-want +got):\n%s", name, diff)
		}
	}
}

func TestLevelAgeBlocksContinuationHigh(t *testing.T) {
	closes := make([]float64, 800)
	r := eval(t, LevelAge{}, levelCtx(closes, 799, 100, 799-3), nil, 799)
	want := Result{Pass: false, Evaluable: true, Evidence: LevelAgeEvidence{Type: TypeLevelAge, BarsSince: 3, LevelDate: date(796)}}
	if diff := cmp.Diff(want, r); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestLevelAgePassesOldCeiling(t *testing.T) {
	closes := make([]float64, 800)
	r := eval(t, LevelAge{}, levelCtx(closes, 799, 100, 799-715), nil, 799)
	want := Result{Pass: true, Evaluable: true, Evidence: LevelAgeEvidence{Type: TypeLevelAge, BarsSince: 715, LevelDate: date(799 - 715)}}
	if diff := cmp.Diff(want, r); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	// Exactly min_age_years (252 daily bars) passes; one bar less fails.
	if r := eval(t, LevelAge{}, levelCtx(closes, 799, 100, 799-252), nil, 799); !r.Pass {
		t.Fatalf("252 bars: got %+v, want pass (≥)", r)
	}
	if r := eval(t, LevelAge{}, levelCtx(closes, 799, 100, 799-251), nil, 799); r.Pass {
		t.Fatalf("251 bars: got %+v, want fail", r)
	}
}

func TestLevelAgeWithoutLevel(t *testing.T) {
	r := eval(t, LevelAge{}, levelCtx(make([]float64, 10), 9, 0, -1), nil, 9)
	want := Result{Evidence: NotEvaluable{Type: TypeNotEvaluable, Reason: ReasonNoBreakoutLevel}}
	if diff := cmp.Diff(want, r); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestLevelAgeNotInDefaultSystemButRegistered(t *testing.T) {
	m := LevelAge{}.Meta()
	p, err := Normalize(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Float("min_age_years") != 1 || len(m.Requires) != 1 || m.Requires[0] != BreakoutLevel {
		t.Fatalf("meta = %+v params = %v", m, p)
	}
	if _, err := Normalize(m, Params{"min_age_years": 0.3}); err == nil {
		t.Fatal("0.3 years accepted; step is 0.25")
	}
}

func TestDefaultRegistersAllFiveV1Conditions(t *testing.T) {
	var got []string
	for _, m := range Default().All() {
		got = append(got, m.Type)
	}
	want := []string{"level_age", "n_year_high", "not_extended", "relative_strength", "volume_surge"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestNoDurationParamIsInBars(t *testing.T) {
	durations := map[string]Unit{"years": Years, "window_weeks": Weeks, "rs_months": Months, "min_age_years": Years}
	for _, m := range Default().All() {
		for _, s := range m.Params {
			if u, ok := durations[s.Name]; ok && s.Unit != u {
				t.Errorf("%s.%s unit = %q, want %q", m.Type, s.Name, s.Unit, u)
			}
		}
	}
}

func TestEvidencesListsEveryType(t *testing.T) {
	var got []string
	for _, ev := range Evidences() {
		got = append(got, ev.EvidenceType())
	}
	want := []string{"n_year_high", "level_age", "not_extended", "relative_strength", "volume_surge", "not_evaluable"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestFiltersNeverEmitNaN(t *testing.T) {
	// Random-ish walk with zeros mixed in: no evaluable evidence may hold
	// NaN or Inf on any bar.
	n := 400
	closes, vols := make([]float64, n), make([]float64, n)
	for i := range n {
		closes[i] = float64((i*37)%11) * 10 // includes zeros
		vols[i] = float64((i * 13) % 5)     // includes zeros
	}
	s := closeSeries("TEST", 0, closes)
	for i := range s.Bars {
		s.Bars[i].Volume = vols[i]
	}
	ctx := NewContext(s, closeSeries("^NSEI", 0, closes))
	pats := []Pattern{NYearHigh{}, VolumeSurge{}, RelativeStrength{}, NotExtended{}, LevelAge{}}
	params := make([]Params, len(pats))
	for i, pat := range pats {
		p, err := Normalize(pat.Meta(), Params{})
		if err != nil {
			t.Fatal(err)
		}
		if pat.Meta().Type == TypeNYearHigh {
			p["years"] = 1.0
		}
		params[i] = p
		if err := pat.Prepare(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	for tt := range n {
		for i, pat := range pats {
			r := pat.Eval(ctx, params[i], tt)
			for _, f := range evidenceFloats(r.Evidence) {
				if math.IsNaN(f) || math.IsInf(f, 0) {
					t.Fatalf("%s at %d: %+v", pat.Meta().Type, tt, r)
				}
			}
		}
	}
}

func evidenceFloats(ev Evidence) []float64 {
	switch e := ev.(type) {
	case NYearHighEvidence:
		return []float64{e.Level, e.Close, e.PctAbove}
	case VolumeEvidence:
		return []float64{e.Ratio, e.AvgVolume}
	case RSEvidence:
		return []float64{e.StockReturn, e.BenchReturn, e.Diff}
	case NotExtendedEvidence:
		return []float64{e.PctAbove}
	}
	return nil
}

func TestRelativeStrengthDataGapInsideWindow(t *testing.T) {
	stock := markGapBefore(closeSeries("TEST", 0, ramp(126, 0.20)), 60)
	bench := closeSeries("^NSEI", 0, ramp(132, 0.10))
	r := eval(t, RelativeStrength{}, NewContext(stock, bench), nil, 126)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonDataGap {
		t.Fatalf("got %+v, want not evaluable: data gap", r)
	}
	if _, ok := RSDiff(NewContext(stock, bench), 6, 126); ok {
		t.Fatal("RSDiff available across a data gap")
	}
}

func TestRelativeStrengthNeedsCompleteBenchmarkWeek(t *testing.T) {
	weekly := func(symbol string, ret float64) *market.Series {
		s := closeSeries(symbol, 0, ramp(26, ret))
		for i := range s.Bars {
			s.Bars[i].Date = date(7 * i) // one bar per ISO week
		}
		s.Timeframe = market.Weekly
		return s
	}
	stock, bench := weekly("TEST", 0.20), weekly("^NSEI", 0.10)
	if r := eval(t, RelativeStrength{}, NewContext(stock, bench), nil, 26); !r.Pass {
		t.Fatalf("complete benchmark week: got %+v, want pass", r)
	}
	bench.Complete = len(bench.Bars) - 1 // the benchmark's last week is still in progress
	r := eval(t, RelativeStrength{}, NewContext(stock, bench), nil, 26)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonBenchmarkMissing {
		t.Fatalf("got %+v, want not evaluable: benchmark week incomplete", r)
	}
}

func TestVolumeSurgeDataGapInsideWindow(t *testing.T) {
	s := markGapBefore(volumeSeries(priorThenToday(50, 100, 300)), 30)
	r := eval(t, VolumeSurge{}, NewContext(s, nil), nil, 50)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonDataGap {
		t.Fatalf("got %+v, want not evaluable: data gap", r)
	}
}
