package report

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
)

func date(i int) market.Date {
	return market.Date(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(market.DateLayout))
}

// provider holds BREAK, 800 bars with flat highs of 100 except 130 at bar
// 300 and a close of 131 (volume 3) on the last bar; NEW, 10 bars only;
// and the ^NSEI benchmark, flat at 100 then 110 on the last bar.
func provider() market.Provider {
	brk := make([]market.Bar, 800)
	nsei := make([]market.Bar, 800)
	for i := range brk {
		brk[i] = market.Bar{Date: date(i), Open: 99, High: 100, Low: 98, Close: 99, Volume: 1}
		nsei[i] = market.Bar{Date: date(i), Open: 100, High: 100, Low: 100, Close: 100}
	}
	brk[300].High = 130
	brk[799].Close, brk[799].High, brk[799].Volume = 131, 132, 3
	nsei[799].Close = 110
	return market.NewMemProvider(
		[]market.Instrument{
			{ID: 1, Symbol: "BREAK", Type: market.Equity},
			{ID: 2, Symbol: "NEW", Type: market.Equity},
			{ID: 3, Symbol: "^NSEI", Type: market.Index},
		},
		map[string][]market.Bar{"BREAK": brk, "NEW": brk[:10], "^NSEI": nsei},
		nil,
	)
}

func TestReportAllPatternsDefaultOrder(t *testing.T) {
	r, err := Run(context.Background(), provider(), "BREAK", "", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, l := range r.Lines {
		types = append(types, l.Type)
		if !l.Result.Pass || !l.Result.Evaluable || l.Label == "" {
			t.Errorf("%s: %+v", l.Type, l)
		}
	}
	want := []string{"n_year_high", "level_age", "not_extended", "relative_strength", "volume_surge"}
	if diff := cmp.Diff(want, types); diff != "" {
		t.Fatalf("line types (-want +got):\n%s", diff)
	}
	if r.Lines[0].Params.Float("years") != 3 {
		t.Fatalf("n_year_high params = %v", r.Lines[0].Params)
	}
	ev := r.Lines[0].Result.Evidence.(patterns.NYearHighEvidence)
	if ev.Level != 130 || ev.LevelDate != date(300) || ev.BarsSince != 499 {
		t.Fatalf("evidence = %+v", ev)
	}
	if age := r.Lines[1].Result.Evidence.(patterns.LevelAgeEvidence); age.BarsSince != 499 || age.LevelDate != date(300) {
		t.Fatalf("level age = %+v", age)
	}
	rs := r.Lines[3].Result.Evidence.(patterns.RSEvidence)
	if math.Abs(rs.BenchReturn-0.10) > 1e-9 || math.Abs(rs.StockReturn-(131.0/99-1)) > 1e-9 {
		t.Fatalf("rs = %+v", rs)
	}
	if vol := r.Lines[4].Result.Evidence.(patterns.VolumeEvidence); vol.Ratio != 3 || vol.AvgVolume != 1 {
		t.Fatalf("volume = %+v", vol)
	}
	if r.Symbol != "BREAK" || r.AsOf != date(799) || r.Timeframe != market.Daily {
		t.Fatalf("header = %s %s %s", r.Symbol, r.AsOf, r.Timeframe)
	}
	if r.Level == nil || *r.Level != 130 || r.LevelDate == nil || *r.LevelDate != date(300) {
		t.Fatalf("level = %v %v", r.Level, r.LevelDate)
	}
}

func TestReportWithoutBenchmarkStillEvaluatesOthers(t *testing.T) {
	brk, err := provider().Daily(context.Background(), 1, "", "9999-12-31")
	if err != nil {
		t.Fatal(err)
	}
	prov := market.NewMemProvider([]market.Instrument{{ID: 1, Symbol: "BREAK", Type: market.Equity}},
		map[string][]market.Bar{"BREAK": brk}, nil)
	r, err := Run(context.Background(), prov, "BREAK", "", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range r.Lines {
		if l.Type == "relative_strength" {
			if ne, ok := l.Result.Evidence.(patterns.NotEvaluable); !ok || ne.Reason != patterns.ReasonBenchmarkMissing {
				t.Fatalf("rs = %+v", l.Result)
			}
		} else if !l.Result.Evaluable {
			t.Fatalf("%s not evaluable without a benchmark: %+v", l.Type, l.Result)
		}
	}
}

func TestReportWeeklyEvaluatesLastCompleteWeek(t *testing.T) {
	// BREAK's last bar, Thu 2022-03-10 (bar 799), is in a forming week; the
	// last complete week ends Sun 2022-03-06 (bar 795). Its 800 calendar
	// days are ~114 weeks, short of the 156 weeks 3 years needs.
	r, err := Run(context.Background(), provider(), "BREAK", "", market.Weekly)
	if err != nil {
		t.Fatal(err)
	}
	if r.Timeframe != market.Weekly || r.AsOf != date(795) {
		t.Fatalf("header = %s %s, want weekly as of %s", r.Timeframe, r.AsOf, date(795))
	}
	res := r.Lines[0].Result
	if ne, ok := res.Evidence.(patterns.NotEvaluable); !ok || ne.Reason != patterns.ReasonInsufficientHistory {
		t.Fatalf("result = %+v, want insufficient_history", res)
	}
}

func TestReportAsOfUsesLastBarOnOrBefore(t *testing.T) {
	// As of bar 500 the breakout on the last bar hasn't happened yet.
	r, err := Run(context.Background(), provider(), "BREAK", date(500), market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	if r.AsOf != date(500) || r.Lines[0].Result.Pass {
		t.Fatalf("as of %s pass=%v", r.AsOf, r.Lines[0].Result.Pass)
	}
}

func TestReportInsufficientHistoryShown(t *testing.T) {
	r, err := Run(context.Background(), provider(), "NEW", "", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	res := r.Lines[0].Result
	ne, ok := res.Evidence.(patterns.NotEvaluable)
	if res.Evaluable || res.Pass || !ok || ne.Reason != patterns.ReasonInsufficientHistory {
		t.Fatalf("result = %+v", res)
	}
	if r.Level != nil || r.LevelDate != nil {
		t.Fatal("level should be absent without enough history")
	}
}

func TestReportBeforeFirstBarIsInsufficientHistory(t *testing.T) {
	r, err := Run(context.Background(), provider(), "NEW", "2000-01-01", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	if r.AsOf != "" || r.Lines[0].Result.Evidence.(patterns.NotEvaluable).Reason != patterns.ReasonInsufficientHistory {
		t.Fatalf("report = %+v", r)
	}
}

func TestReportUnknownSymbol(t *testing.T) {
	_, err := Run(context.Background(), provider(), "NOPE", "", market.Daily)
	if !errors.Is(err, market.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
