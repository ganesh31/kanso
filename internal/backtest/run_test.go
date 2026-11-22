package backtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/rules"
)

// datedFrom returns bars with their dates rewritten to consecutive
// weekdays from start.
func datedFrom(start market.Date, bars []market.Bar) []market.Bar {
	d := start.Time()
	out := make([]market.Bar, len(bars))
	for i := range bars {
		for d.Weekday() == 0 || d.Weekday() == 6 {
			d = d.AddDate(0, 0, 1)
		}
		out[i] = bars[i]
		out[i].Date = market.Date(d.Format(market.DateLayout))
		d = d.AddDate(0, 0, 1)
	}
	return out
}

// runProvider has RW (a 2,000-bar walk that signals on the default
// system), EMPTY (no bars) and the benchmark, which starts later than RW.
func runProvider(benchStart market.Date) *market.MemProvider {
	r := rand.New(rand.NewPCG(7, 11))
	stock := randomWalk(r, 2000, 0.0015)
	bench := datedFrom(benchStart, randomWalk(r, 2000, 0.0003))
	insts := []market.Instrument{
		{ID: 1, Symbol: "RW", Type: market.Equity},
		{ID: 2, Symbol: "EMPTY", Type: market.Equity},
		{ID: 3, Symbol: market.Benchmark, Type: market.Index},
	}
	return market.NewMemProvider(insts,
		map[string][]market.Bar{"RW": stock, market.Benchmark: bench},
		map[string][]string{"TEST": {"RW", "EMPTY"}})
}

func testSystem() rules.System {
	s := rules.DefaultSystem()
	s.Universe = "TEST"
	return s
}

func TestRunUsesBenchmarkStart(t *testing.T) {
	res, err := Run(context.Background(), runProvider("2015-06-01"), testSystem(), "2010-01-01", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Honesty.DataStart != "2015-06-01" {
		t.Fatalf("DataStart = %s, want the benchmark's first date 2015-06-01", res.Honesty.DataStart)
	}
	if len(res.AllTrades) == 0 {
		t.Fatal("no trades")
	}
	for _, tr := range res.AllTrades {
		if tr.SignalDate < "2015-06-01" {
			t.Fatalf("trade signalled on %s, before the data start", tr.SignalDate)
		}
	}
}

func TestRunUsesLaterRequestedStart(t *testing.T) {
	res, err := Run(context.Background(), runProvider("2015-06-01"), testSystem(), "2018-01-01", "2021-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if res.Honesty.DataStart != "2018-01-01" || res.Honesty.DataEnd != "2021-12-31" {
		t.Fatalf("window %s → %s, want 2018-01-01 → 2021-12-31", res.Honesty.DataStart, res.Honesty.DataEnd)
	}
	for _, tr := range res.AllTrades {
		if tr.SignalDate < "2018-01-01" || tr.ExitDate > "2021-12-31" {
			t.Fatalf("trade %s → %s is outside the window", tr.SignalDate, tr.ExitDate)
		}
	}
}

func TestRunSkipsInstrumentWithNoBars(t *testing.T) {
	res, err := Run(context.Background(), runProvider("2015-01-05"), testSystem(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	want := []SkippedInstrument{{Symbol: "EMPTY", Reason: "no_data"}}
	if diff := cmp.Diff(want, res.Honesty.Skipped); diff != "" {
		t.Fatalf("Skipped (-want +got):\n%s", diff)
	}
}

func TestRunTradesAreHonest(t *testing.T) {
	res, err := Run(context.Background(), runProvider("2015-01-05"), testSystem(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	h := res.Honesty
	if h.FillModel != "next-day open" || h.PerSidePct != 0.20 || h.SlippagePct != 0.10 || !h.Survivorship {
		t.Fatalf("honesty = %+v", h)
	}
	if res.TradeStats.Trades != len(res.AllTrades) {
		t.Fatalf("stats over %d trades, have %d", res.TradeStats.Trades, len(res.AllTrades))
	}
	for i, tr := range res.AllTrades {
		if tr.EntryDate <= tr.SignalDate {
			t.Fatalf("trade %d entered %s, not after its signal %s", i, tr.EntryDate, tr.SignalDate)
		}
		if i > 0 && tr.EntryDate < res.AllTrades[i-1].ExitDate {
			t.Fatalf("trade %d entered %s while the previous trade was open until %s", i, tr.EntryDate, res.AllTrades[i-1].ExitDate)
		}
	}
}

// breakoutBars are 800 days that make a new 3-year high on bar 795 and
// hold it to the end. The close 126 bars (6 months) before it is base, so
// a lower base means a stronger 6-month return.
func breakoutBars(base float64) []market.Bar {
	bars := make([]market.Bar, 800)
	for i := range bars {
		bars[i] = market.Bar{Open: 99, High: 100, Low: 98, Close: 99, Volume: 1}
	}
	bars[300].High = 130
	bars[669] = market.Bar{Open: base, High: base, Low: base, Close: base, Volume: 1}
	for i := 795; i < 800; i++ {
		bars[i] = market.Bar{Open: 131, High: 132, Low: 130, Close: 131, Volume: 1}
	}
	return datedFrom("2020-01-01", bars)
}

func TestRankKeyIgnoresRule(t *testing.T) {
	flat := breakoutBars(99)
	bench := make([]market.Bar, len(flat))
	for i, b := range flat {
		bench[i] = market.Bar{Date: b.Date, Open: 100, High: 100, Low: 100, Close: 100}
	}
	prov := market.NewMemProvider([]market.Instrument{
		{ID: 1, Symbol: "AAA", Type: market.Equity},
		{ID: 2, Symbol: "ZZZ", Type: market.Equity},
		{ID: 3, Symbol: market.Benchmark, Type: market.Index},
	}, map[string][]market.Bar{"AAA": flat, "ZZZ": breakoutBars(60), market.Benchmark: bench},
		map[string][]string{"TEST": {"AAA", "ZZZ"}})
	s := rules.System{Name: "no rs", Universe: "TEST", Entry: rules.Entry{All: []rules.Condition{{Type: "n_year_high"}}},
		Portfolio: rules.Portfolio{MaxPositions: 1}}
	res, err := Run(context.Background(), prov, s, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.AllTrades) != 2 || res.AllTrades[0].EntryDate != res.AllTrades[1].EntryDate {
		t.Fatalf("want AAA and ZZZ entering the same day, got %+v", res.AllTrades)
	}
	if len(res.Taken) != 1 || res.Taken[0].Symbol != "ZZZ" {
		t.Fatalf("taken %+v, want ZZZ: its 6-month RS beats AAA's, though RS is not in the rule", res.Taken)
	}
}

func TestRunPortfolioAndBenchmark(t *testing.T) {
	res, err := Run(context.Background(), runProvider("2015-01-05"), testSystem(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Honesty.RankingRule != "crowded days ranked by 6-month RS, then symbol" {
		t.Fatalf("RankingRule = %q", res.Honesty.RankingRule)
	}
	if len(res.Taken) == 0 || len(res.Equity) == 0 || len(res.Equity) != len(res.Benchmark) {
		t.Fatalf("taken %d, equity %d, benchmark %d points", len(res.Taken), len(res.Equity), len(res.Benchmark))
	}
	if res.Equity[0].Date != res.Honesty.DataStart || !near(res.Equity[0].Value, 1_000_000) || !near(res.Benchmark[0].Value, 1_000_000) {
		t.Fatalf("curves start %+v / %+v, want 1,000,000 on %s", res.Equity[0], res.Benchmark[0], res.Honesty.DataStart)
	}
	p := res.Portfolio
	if p.FinalEquity != res.Equity[len(res.Equity)-1].Value || p.CAGR != CAGR(res.Equity) ||
		p.MaxDrawdown != MaxDrawdown(res.Equity) || p.BenchCAGR != CAGR(res.Benchmark) ||
		p.BenchMaxDrawdown != MaxDrawdown(res.Benchmark) || p.Exposure <= 0 || p.Exposure > 1 {
		t.Fatalf("portfolio stats %+v don't match the curves", p)
	}
}

func TestRunDeterministic(t *testing.T) {
	var out [2][]byte
	for i := range out {
		res, err := Run(context.Background(), runProvider("2015-01-05"), testSystem(), "", "")
		if err != nil {
			t.Fatal(err)
		}
		if out[i], err = json.Marshal(res); err != nil {
			t.Fatal(err)
		}
	}
	if !bytes.Equal(out[0], out[1]) {
		t.Fatal("two runs on the same data gave different JSON")
	}
}

func TestRunInvalidSystem(t *testing.T) {
	s := testSystem()
	s.Exits.StopPct = 0
	_, err := Run(context.Background(), runProvider("2015-01-05"), s, "", "")
	var ve *rules.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want a *rules.ValidationError", err)
	}
}

func TestRunUnknownUniverse(t *testing.T) {
	s := testSystem()
	s.Universe = "NOPE"
	if _, err := Run(context.Background(), runProvider("2015-01-05"), s, "", ""); !errors.Is(err, market.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestRunBenchmarkWithoutBarsUsesStockDates(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	insts := []market.Instrument{
		{ID: 1, Symbol: "RW", Type: market.Equity},
		{ID: 3, Symbol: market.Benchmark, Type: market.Index},
	}
	prov := market.NewMemProvider(insts,
		map[string][]market.Bar{"RW": randomWalk(r, 600, 0.0015)},
		map[string][]string{"TEST": {"RW"}})
	res, err := Run(context.Background(), prov, testSystem(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Equity) == 0 {
		t.Fatal("no equity curve: the calendar ignored the stock's dates")
	}
	if res.Honesty.DataStart == "" || res.Honesty.DataStart != res.Equity[0].Date {
		t.Fatalf("DataStart = %q, want the first simulated date %s", res.Honesty.DataStart, res.Equity[0].Date)
	}
}

// historyProvider is a provider whose stock lists have historical
// membership.
type historyProvider struct{ *market.MemProvider }

func (p historyProvider) Universe(ctx context.Context, name string) ([]market.Instrument, bool, error) {
	m, _, err := p.MemProvider.Universe(ctx, name)
	return m, true, err
}

func TestRunSurvivorshipFlag(t *testing.T) {
	res, err := Run(context.Background(), runProvider("2015-01-05"), testSystem(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Honesty.Survivorship {
		t.Fatal("Survivorship = false for a stock list with no history")
	}
	res, err = Run(context.Background(), historyProvider{runProvider("2015-01-05")}, testSystem(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if res.Honesty.Survivorship {
		t.Fatal("Survivorship = true for a stock list with historical membership")
	}
}

func TestRunHonestyFields(t *testing.T) {
	sys := testSystem()
	sys.Costs.PerSidePct, sys.Costs.SlippagePct = 0.3, 0.05
	sys.Portfolio.Capital = 1000 // too small for RW's whole shares: skipped signals
	res, err := Run(context.Background(), runProvider("2015-06-01"), sys, "2016-01-01", "2020-12-31")
	if err != nil {
		t.Fatal(err)
	}
	h := res.Honesty
	if len(h.SkippedSignals) == 0 {
		t.Fatal("no skipped signals with capital too small for one share")
	}
	for _, s := range h.SkippedSignals {
		if s.Symbol != "RW" || s.Reason == "" || s.EntryDate < "2016-01-01" {
			t.Fatalf("skipped signal %+v", s)
		}
	}
	h.SkippedSignals = nil
	want := Honesty{
		FillModel: "next-day open", PerSidePct: 0.3, SlippagePct: 0.05, Survivorship: true,
		DataSource: "Yahoo Finance", Adjustment: "split-adjusted, not dividend-adjusted",
		DataStart: "2016-01-01", DataEnd: "2020-12-31",
		RankingRule: "crowded days ranked by 6-month RS, then symbol",
		Skipped:     []SkippedInstrument{{Symbol: "EMPTY", Reason: "no_data"}},
	}
	if diff := cmp.Diff(want, h); diff != "" {
		t.Fatalf("Honesty (-want +got):\n%s", diff)
	}
}
