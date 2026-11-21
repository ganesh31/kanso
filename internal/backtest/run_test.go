package backtest

import (
	"context"
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
