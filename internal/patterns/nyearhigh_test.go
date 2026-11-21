package patterns

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"kanso/internal/market"
)

// date returns a distinct trading date for bar i (consecutive calendar
// days from 2020-01-01; weekends don't matter to these patterns).
func date(i int) market.Date {
	return market.Date(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(market.DateLayout))
}

// dailySeries builds a daily series with the given highs and closes
// (open and low are set to the close).
func dailySeries(highs, closes []float64) *market.Series {
	bars := make([]market.Bar, len(highs))
	for i := range highs {
		bars[i] = market.Bar{Date: date(i), Open: closes[i], High: highs[i], Low: closes[i], Close: closes[i], Volume: 1000}
	}
	return market.NewDaily(market.Instrument{ID: 1, Symbol: "TEST"}, bars)
}

// flat returns n bars with high 100 and close 99.
func flat(n int) (highs, closes []float64) {
	highs = make([]float64, n)
	closes = make([]float64, n)
	for i := range highs {
		highs[i], closes[i] = 100, 99
	}
	return highs, closes
}

func evalNYH(t *testing.T, s *market.Series, p Params, at int) (Result, *Context) {
	t.Helper()
	pat := NYearHigh{}
	np, err := Normalize(pat.Meta(), p)
	if err != nil {
		t.Fatal(err)
	}
	ctx := NewContext(s, nil)
	if err := pat.Prepare(ctx, np); err != nil {
		t.Fatal(err)
	}
	return pat.Eval(ctx, np, at), ctx
}

// oneYearWithHigh returns 253 bars (L=252 prior bars + today) whose prior
// max high is 110 at index 100, with today's close set by the caller.
func oneYearWithHigh(todayHigh, todayClose float64) *market.Series {
	highs, closes := flat(253)
	highs[100] = 110
	highs[252], closes[252] = todayHigh, todayClose
	return dailySeries(highs, closes)
}

func TestNYearHighBreaksAboveLevel(t *testing.T) {
	r, _ := evalNYH(t, oneYearWithHigh(111, 111), Params{"years": 1.0}, 252)
	want := Result{Pass: true, Evaluable: true, Evidence: NYearHighEvidence{
		Type: TypeNYearHigh, Level: 110, LevelDate: date(100), BarsSince: 152, Close: 111,
		PctAbove: 0.909090909090909, // 1/110 = 0.909…%
	}}
	if diff := cmp.Diff(want, r, cmpopts.EquateApprox(1e-9, 0)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestNYearHighEqualToLevelFails(t *testing.T) {
	r, _ := evalNYH(t, oneYearWithHigh(110, 110), Params{"years": 1.0}, 252)
	if r.Pass || !r.Evaluable {
		t.Fatalf("close == level: got %+v, want evaluable fail", r)
	}
}

func TestNYearHighExcludesCurrentBarHigh(t *testing.T) {
	r, _ := evalNYH(t, oneYearWithHigh(200, 111), Params{"years": 1.0}, 252)
	ev := r.Evidence.(NYearHighEvidence)
	if !r.Pass || ev.Level != 110 {
		t.Fatalf("got pass=%v level=%v, want pass at level 110", r.Pass, ev.Level)
	}
}

func TestNYearHighWindowDropsOldestBar(t *testing.T) {
	// The 300 high at bar 0 is outside the 252-bar window of bar 253.
	highs, closes := flat(254)
	highs[0] = 300
	highs[1] = 120
	closes[253] = 121
	r, _ := evalNYH(t, dailySeries(highs, closes), Params{"years": 1.0}, 253)
	ev := r.Evidence.(NYearHighEvidence)
	if !r.Pass || ev.Level != 120 || ev.LevelDate != date(1) {
		t.Fatalf("got %+v", r)
	}
}

// weeklySeries resamples dailyWeeks(n, weekHigh) to weekly.
func weeklySeries(n int, weekHigh map[int]float64) *market.Series {
	return market.ToWeekly(dailyWeeks(n, weekHigh))
}

// dailyWeeks is n weeks of Mon–Fri daily bars from Mon 2024-01-01, plus
// one Monday so the last full week is complete. Every bar has high 100 and
// close 99 except where weekHigh sets a whole week's high.
func dailyWeeks(n int, weekHigh map[int]float64) *market.Series {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var bars []market.Bar
	for w := 0; w <= n; w++ {
		days := 5
		if w == n {
			days = 1
		}
		for d := range days {
			h := 100.0
			if v, ok := weekHigh[w]; ok {
				h = v
			}
			dt := market.Date(start.AddDate(0, 0, 7*w+d).Format(market.DateLayout))
			bars = append(bars, market.Bar{Date: dt, Open: 99, High: h, Low: 99, Close: 99, Volume: 1000})
		}
	}
	return market.NewDaily(market.Instrument{ID: 1, Symbol: "TEST"}, bars)
}

func TestNYearHighWeeklyUses52PerYear(t *testing.T) {
	s := weeklySeries(54, map[int]float64{0: 110})
	if s.Complete != 54 {
		t.Fatalf("Complete = %d, want 54", s.Complete)
	}
	r, _ := evalNYH(t, s, Params{"years": 1.0}, 51)
	if r.Evaluable {
		t.Fatalf("t=51: got %+v, want insufficient history (needs 52 prior weeks)", r)
	}
	r, _ = evalNYH(t, s, Params{"years": 1.0}, 52)
	if ev := r.Evidence.(NYearHighEvidence); ev.Level != 110 || ev.BarsSince != 52 {
		t.Fatalf("t=52: got %+v, want level 110 set 52 weeks ago", r)
	}
	r, _ = evalNYH(t, s, Params{"years": 1.0}, 53)
	if ev := r.Evidence.(NYearHighEvidence); ev.Level != 100 {
		t.Fatalf("t=53: got %+v, want week 0 out of the 52-week window", r)
	}
	r, _ = evalNYH(t, s, Params{"years": 1.0}, 54)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonIncompleteBar {
		t.Fatalf("t=54 (forming week): got %+v, want incomplete_bar", r)
	}
}

func TestNYearHighInsufficientHistory(t *testing.T) {
	r, _ := evalNYH(t, oneYearWithHigh(111, 111), Params{"years": 1.0}, 251)
	want := Result{Evidence: NotEvaluable{Type: TypeNotEvaluable, Reason: ReasonInsufficientHistory}}
	if diff := cmp.Diff(want, r, cmpopts.EquateApprox(1e-9, 0)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestNYearHighAllTimeNeeds504Bars(t *testing.T) {
	highs, closes := flat(600)
	highs[3] = 150 // all-time high, far older than any year window
	closes[503], closes[504] = 151, 151
	s := dailySeries(highs, closes)

	r, _ := evalNYH(t, s, Params{"all_time": true}, 503)
	if r.Evaluable {
		t.Fatalf("t=503: got %+v, want not evaluable", r)
	}
	if ne := r.Evidence.(NotEvaluable); ne.Reason != ReasonInsufficientHistory {
		t.Fatalf("reason = %s", ne.Reason)
	}
	r, ctx := evalNYH(t, s, Params{"all_time": true}, 504)
	ev := r.Evidence.(NYearHighEvidence)
	if !r.Evaluable || !r.Pass || ev.Level != 150 || ev.LevelDate != date(3) {
		t.Fatalf("t=504: got %+v", r)
	}
	if ctx.LevelIdx[504] != 3 || ctx.Level[504] != 150 {
		t.Fatalf("context level = %v@%d", ctx.Level[504], ctx.LevelIdx[504])
	}
}

func TestNYearHighPublishesLevelForEveryBar(t *testing.T) {
	_, ctx := evalNYH(t, oneYearWithHigh(111, 111), Params{"years": 1.0}, 252)
	if len(ctx.LevelIdx) != 253 || ctx.LevelIdx[251] != -1 || ctx.LevelIdx[252] != 100 {
		t.Fatalf("LevelIdx len=%d [251]=%d [252]=%d", len(ctx.LevelIdx), ctx.LevelIdx[251], ctx.LevelIdx[252])
	}
}

func TestNYearHighIncompleteBarNotEvaluable(t *testing.T) {
	s := oneYearWithHigh(111, 111)
	s.Complete = 252 // bar 252 is still forming
	r, _ := evalNYH(t, s, Params{"years": 1.0}, 252)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonIncompleteBar {
		t.Fatalf("got %+v", r)
	}
}

func TestNYearHighZeroLevelNotEvaluable(t *testing.T) {
	highs, closes := make([]float64, 253), make([]float64, 253)
	closes[252] = 1
	r, _ := evalNYH(t, dailySeries(highs, closes), Params{"years": 1.0}, 252)
	if r.Evaluable || r.Evidence.(NotEvaluable).Reason != ReasonZeroDenominator {
		t.Fatalf("got %+v", r)
	}
	if ev, ok := r.Evidence.(NYearHighEvidence); ok && math.IsInf(ev.PctAbove, 0) {
		t.Fatal("Inf in evidence")
	}
}

func TestNYearHighMeta(t *testing.T) {
	m := NYearHigh{}.Meta()
	if m.Type != "n_year_high" || len(m.Provides) != 1 || m.Provides[0] != BreakoutLevel {
		t.Fatalf("meta = %+v", m)
	}
	p, err := Normalize(m, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.Float("years") != 3 || p.Bool("all_time") {
		t.Fatalf("defaults = %v", p)
	}
}

func TestRegistryAllSortedByType(t *testing.T) {
	r := NewRegistry(fakePattern{"zeta"}, NYearHigh{}, fakePattern{"alpha"})
	var got []string
	for _, m := range r.All() {
		got = append(got, m.Type)
	}
	if diff := cmp.Diff([]string{"alpha", "n_year_high", "zeta"}, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	if _, ok := r.Get("n_year_high"); !ok {
		t.Fatal("Get(n_year_high) missing")
	}
	if _, ok := r.Get("nope"); ok {
		t.Fatal("Get(nope) found")
	}
	if _, ok := Default().Get("n_year_high"); !ok {
		t.Fatal("Default() lacks n_year_high")
	}
}

type fakePattern struct{ typ string }

func (f fakePattern) Meta() Meta                      { return Meta{Type: f.typ} }
func (fakePattern) Prepare(*Context, Params) error    { return nil }
func (fakePattern) Eval(*Context, Params, int) Result { return Result{} }

func TestLoadBuildsDailyContextAlignedToBench(t *testing.T) {
	insts := []market.Instrument{{ID: 1, Symbol: "TCS"}, {ID: 2, Symbol: "^NSEI", Type: market.Index}}
	bars := map[string][]market.Bar{
		"TCS":   {{Date: "2024-01-01"}, {Date: "2024-01-02"}, {Date: "2024-01-04"}},
		"^NSEI": {{Date: "2024-01-01"}, {Date: "2024-01-03"}, {Date: "2024-01-04"}},
	}
	prov := market.NewMemProvider(insts, bars, nil)
	bench := market.NewDaily(insts[1], bars["^NSEI"])
	pctx, daily, err := Load(context.Background(), prov, insts[0], bench, market.Daily, "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if daily.Complete != 3 || pctx.Series != daily || pctx.Bench != bench {
		t.Fatalf("daily = %+v", daily)
	}
	if diff := cmp.Diff([]int{0, -1, 2}, pctx.BenchIdx); diff != "" {
		t.Fatalf("BenchIdx (-want +got):\n%s", diff)
	}
}

func TestLoadBuildsWeeklyAlignedByISOWeek(t *testing.T) {
	insts := []market.Instrument{{ID: 1, Symbol: "TCS"}, {ID: 2, Symbol: "^NSEI", Type: market.Index}}
	bars := map[string][]market.Bar{
		// TCS skips Fri 5 Jan; the bench has no bar in the week of 15 Jan.
		"TCS": {
			{Date: "2024-01-01"}, {Date: "2024-01-04"},
			{Date: "2024-01-08"}, {Date: "2024-01-12"},
			{Date: "2024-01-16"},
			{Date: "2024-01-22"},
		},
		"^NSEI": {
			{Date: "2024-01-01"}, {Date: "2024-01-05"},
			{Date: "2024-01-09"}, {Date: "2024-01-12"},
			{Date: "2024-01-22"},
		},
	}
	prov := market.NewMemProvider(insts, bars, nil)
	bench := market.NewDaily(insts[1], bars["^NSEI"])
	pctx, daily, err := Load(context.Background(), prov, insts[0], bench, market.Weekly, "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if daily.Timeframe != market.Daily || len(daily.Bars) != 6 {
		t.Fatalf("daily = %+v", daily)
	}
	s := pctx.Series
	if s.Timeframe != market.Weekly || len(s.Bars) != 4 || s.Complete != 3 {
		t.Fatalf("series: timeframe %q, %d bars, Complete %d; want weekly, 4, 3", s.Timeframe, len(s.Bars), s.Complete)
	}
	if pctx.Bench.Timeframe != market.Weekly || len(pctx.Bench.Bars) != 3 {
		t.Fatalf("bench: timeframe %q, %d bars; want weekly, 3", pctx.Bench.Timeframe, len(pctx.Bench.Bars))
	}
	// Weeks end on different days (TCS Thu 4 Jan, bench Fri 5 Jan) yet align.
	if diff := cmp.Diff([]int{0, 1, -1, 2}, pctx.BenchIdx); diff != "" {
		t.Fatalf("BenchIdx (-want +got):\n%s", diff)
	}
}
