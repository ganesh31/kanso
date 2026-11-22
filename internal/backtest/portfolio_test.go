package backtest

import (
	"testing"

	"kanso/internal/market"
	"kanso/internal/rules"
)

// simDates is a calendar of n consecutive weekdays from 2024-01-01.
func simDates(n int) []market.Date {
	bars := flatDays(n)
	out := make([]market.Date, n)
	for i, b := range bars {
		out[i] = b.Date
	}
	return out
}

// closesAt is a daily series for sym on cal with the given closes (one per
// date; open = close).
func closesAt(sym string, cal []market.Date, closes ...float64) *market.Series {
	bars := make([]market.Bar, len(closes))
	for i, c := range closes {
		bars[i] = market.Bar{Date: cal[i], Open: c, High: c, Low: c, Close: c}
	}
	return market.NewDaily(market.Instrument{Symbol: sym, Type: market.Equity}, bars)
}

// flatCloses is sym at price on every date of cal.
func flatCloses(sym string, cal []market.Date, price float64) *market.Series {
	cs := make([]float64, len(cal))
	for i := range cs {
		cs[i] = price
	}
	return closesAt(sym, cal, cs...)
}

// cand is a candidate in sym entered on cal[entry] and exited on cal[exit]
// (ExitOpen when exit is the last date), both at price.
func cand(sym string, cal []market.Date, entry, exit int, price, rank float64) Trade {
	reason := ExitStop
	if exit == len(cal)-1 {
		reason = ExitOpen
	}
	return Trade{
		Symbol: sym, SignalDate: cal[entry-1], EntryDate: cal[entry], ExitDate: cal[exit],
		EntryPrice: price, ExitPrice: price, ExitReason: reason, HoldingDays: exit - entry, RankKey: rank,
	}
}

// simulate runs Simulate with no costs or slippage.
func simulate(cands []Trade, cal []market.Date, capital float64, maxPos int, series ...*market.Series) Simulation {
	closes := map[string]*market.Series{}
	for _, s := range series {
		closes[s.Instrument.Symbol] = s
	}
	return Simulate(cands, closes, cal, rules.Portfolio{Capital: capital, MaxPositions: maxPos}, EquityExecutor{}, PercentCost{})
}

func takenSymbols(sim Simulation) []string {
	out := make([]string, len(sim.Taken))
	for i, t := range sim.Taken {
		out[i] = t.Symbol + "@" + string(t.EntryDate)
	}
	return out
}

func TestSimulateRespectsMaxPositions(t *testing.T) {
	cal := simDates(10)
	sim := simulate([]Trade{
		cand("AAA", cal, 2, 9, 100, 0.1),
		cand("BBB", cal, 2, 9, 100, 0.3),
		cand("CCC", cal, 2, 9, 100, 0.2),
	}, cal, 1000, 2,
		flatCloses("AAA", cal, 100), flatCloses("BBB", cal, 100), flatCloses("CCC", cal, 100))
	got := takenSymbols(sim)
	want := []string{"BBB@" + string(cal[2]), "CCC@" + string(cal[2])}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("taken %v, want %v (the two highest RankKeys)", got, want)
	}
	for _, tr := range sim.Taken {
		if tr.Quantity != 5 {
			t.Fatalf("%s quantity %d, want 5 (1000/2 slots at 100)", tr.Symbol, tr.Quantity)
		}
	}
}

func TestSimulateTieBreakBySymbol(t *testing.T) {
	cal := simDates(10)
	sim := simulate([]Trade{
		cand("BBB", cal, 2, 9, 100, 0.5),
		cand("AAA", cal, 2, 9, 100, 0.5),
	}, cal, 1000, 1, flatCloses("AAA", cal, 100), flatCloses("BBB", cal, 100))
	if got := takenSymbols(sim); len(got) != 1 || got[0] != "AAA@"+string(cal[2]) {
		t.Fatalf("taken %v, want AAA alone (equal RankKey, symbol ascending)", got)
	}
}

func TestSimulateExitsReleaseSlotSameDay(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason ExitReason
		gapped bool
	}{
		{"trend exit", ExitTrend, false},
		{"gapped stop", ExitStop, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cal := simDates(10)
			aaa := cand("AAA", cal, 1, 4, 100, 0.9)
			aaa.ExitReason, aaa.GappedStop = tc.reason, tc.gapped
			sim := simulate([]Trade{aaa, cand("BBB", cal, 4, 9, 100, 0.1)},
				cal, 1000, 1, flatCloses("AAA", cal, 100), flatCloses("BBB", cal, 100))
			got := takenSymbols(sim)
			if len(got) != 2 || got[1] != "BBB@"+string(cal[4]) {
				t.Fatalf("taken %v, want BBB entered on AAA's exit day %s", got, cal[4])
			}
			if sim.Taken[1].Quantity != 10 {
				t.Fatalf("BBB quantity %d, want 10: AAA's sale at the open returned the cash first", sim.Taken[1].Quantity)
			}
		})
	}
}

func TestSimulateIntradayStopReleasesAfterEntries(t *testing.T) {
	cal := simDates(10)
	aaa := cand("AAA", cal, 1, 4, 100, 0.9) // stop hit after the open on day 4
	sim := simulate([]Trade{aaa, cand("BBB", cal, 4, 9, 100, 0.1)},
		cal, 1000, 1, flatCloses("AAA", cal, 100), flatCloses("BBB", cal, 100))
	if got := takenSymbols(sim); len(got) != 1 || got[0] != "AAA@"+string(cal[1]) {
		t.Fatalf("taken %v, want AAA alone: at day 4's open AAA still holds the only slot", got)
	}
	if got := sim.Equity[4].Value; !near(got, 1000) {
		t.Fatalf("equity on day 4 = %v, want 1000 (AAA sold, all cash)", got)
	}
}

func TestSimulateCashCap(t *testing.T) {
	cal := simDates(10)
	// AAA triples on its entry day: equity 2,000, so the next slot would be
	// 1,000, but only 500 is cash.
	aaa := closesAt("AAA", cal, 100, 300, 300, 300, 300, 300, 300, 300, 300, 300)
	sim := simulate([]Trade{
		cand("AAA", cal, 1, 9, 100, 0.9),
		cand("BBB", cal, 2, 9, 100, 0.1),
	}, cal, 1000, 2, aaa, flatCloses("BBB", cal, 100))
	if len(sim.Taken) != 2 || sim.Taken[1].Quantity != 5 {
		t.Fatalf("taken %+v, want BBB with 5 shares (500 cash, not the 1,000 slot)", sim.Taken)
	}
	if got := sim.Equity[2].Value; !near(got, 2000) {
		t.Fatalf("equity on day 2 = %v, want 2000 (no cash left, 5×300 + 5×100)", got)
	}
}

func TestSimulateGapUpEntryStillFills(t *testing.T) {
	bars := flatDays(10)
	setBar(bars, 1, 105, 105, 105) // signal closed at 100 on bar 0, opens 5% higher
	tr := path(t, bars, 0, sys(0))
	daily := market.NewDaily(market.Instrument{Symbol: "T", Type: market.Equity}, bars)
	sim := Simulate([]Trade{tr}, map[string]*market.Series{"T": daily}, simDates(10),
		rules.Portfolio{Capital: 1_000_000, MaxPositions: 10},
		EquityExecutor{SlippagePct: 0.10}, PercentCost{PerSidePct: 0.20})
	if len(sim.Skipped) != 0 || len(sim.Taken) != 1 {
		t.Fatalf("taken %d, skipped %+v: a gap-up entry must still fill", len(sim.Taken), sim.Skipped)
	}
	// 100,000 slot ÷ (105 × 1.001 slippage × 1.002 cost) = 949.5 shares.
	if q := sim.Taken[0].Quantity; q != 949 {
		t.Fatalf("quantity %d, want 949", q)
	}
	if !near(sim.Taken[0].EntryPrice, 105.105) {
		t.Fatalf("entry fill %v, want 105.105", sim.Taken[0].EntryPrice)
	}
	// Cash 1,000,000 − 949 × 105.105 × 1.002, plus 949 shares at the 105 close.
	if got, want := sim.Equity[1].Value, 1_000_000-949*105.105*1.002+949*105; !near(got, want) {
		t.Fatalf("equity at entry close %v, want %v", got, want)
	}
}

func TestSimulateTakesLaterSignalAfterRejectedOne(t *testing.T) {
	cal := simDates(12)
	sim := simulate([]Trade{
		cand("AAA", cal, 1, 5, 100, 0.9),
		cand("TCS", cal, 1, 8, 100, 0.1),  // S1: no slot, rejected
		cand("TCS", cal, 6, 11, 100, 0.1), // S2: inside S1's would-be trade
	}, cal, 1000, 1, flatCloses("AAA", cal, 100), flatCloses("TCS", cal, 100))
	got := takenSymbols(sim)
	want := []string{"AAA@" + string(cal[1]), "TCS@" + string(cal[6])}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("taken %v, want %v", got, want)
	}
	if len(sim.Skipped) != 0 {
		t.Fatalf("skipped %+v: a signal rejected for lack of slots is not a skipped signal", sim.Skipped)
	}
}

func TestSimulateSkipsHeldSymbol(t *testing.T) {
	cal := simDates(12)
	sim := simulate([]Trade{
		cand("TCS", cal, 1, 8, 100, 0.1),
		cand("TCS", cal, 4, 11, 100, 0.1), // TCS is still held
	}, cal, 1000, 5, flatCloses("TCS", cal, 100))
	if got := takenSymbols(sim); len(got) != 1 {
		t.Fatalf("taken %v, want only the first TCS trade", got)
	}
}

func TestSimulateSkipsZeroShareTrade(t *testing.T) {
	cal := simDates(10)
	sim := simulate([]Trade{
		cand("MRF", cal, 1, 9, 600, 0.9), // one share is more than the 500 slot
		cand("BBB", cal, 1, 9, 100, 0.5),
		cand("CCC", cal, 1, 9, 100, 0.1),
	}, cal, 1000, 2, flatCloses("MRF", cal, 600), flatCloses("BBB", cal, 100), flatCloses("CCC", cal, 100))
	want := SkippedSignal{Symbol: "MRF", EntryDate: cal[1], Reason: "price_above_slot"}
	if len(sim.Skipped) != 1 || sim.Skipped[0] != want {
		t.Fatalf("skipped %+v, want %+v", sim.Skipped, want)
	}
	if got := takenSymbols(sim); len(got) != 2 || got[0][:3] != "BBB" || got[1][:3] != "CCC" {
		t.Fatalf("taken %v, want BBB and CCC: MRF's slot stays free", got)
	}
}

func TestSimulateSkipsWhenCashIsShortOfOneShare(t *testing.T) {
	cal := simDates(10)
	// AAA: 5 shares at 100 use 500 of the 1,000 capital, then the stock
	// triples, so equity is 2,000 and the slot (equity ÷ 2) is 1,000. One BBB
	// share (600) fits the slot but not the 500 cash left.
	aaa := closesAt("AAA", cal, 100, 100, 300, 300, 300, 300, 300, 300, 300, 300)
	sim := simulate([]Trade{
		cand("AAA", cal, 1, 9, 100, 0.9),
		cand("BBB", cal, 4, 9, 600, 0.5),
	}, cal, 1000, 2, aaa, flatCloses("BBB", cal, 600))
	want := SkippedSignal{Symbol: "BBB", EntryDate: cal[4], Reason: "insufficient_cash"}
	if len(sim.Skipped) != 1 || sim.Skipped[0] != want {
		t.Fatalf("skipped %+v, want %+v", sim.Skipped, want)
	}
}

func TestSimulateSameDayStopAndExposure(t *testing.T) {
	cal := simDates(10)
	sim := simulate([]Trade{
		cand("AAA", cal, 2, 2, 100, 0.9), // stopped out on its entry day
		cand("BBB", cal, 5, 9, 100, 0.1),
	}, cal, 1000, 1, flatCloses("AAA", cal, 100), flatCloses("BBB", cal, 100))
	if len(sim.Taken) != 2 {
		t.Fatalf("taken %v, want both", takenSymbols(sim))
	}
	// Held at the close on dates 5..9 only.
	if !near(sim.Exposure, 0.5) {
		t.Fatalf("exposure %v, want 0.5", sim.Exposure)
	}
	if !near(sim.Equity[2].Value, 1000) {
		t.Fatalf("equity after the same-day round trip %v, want 1000", sim.Equity[2].Value)
	}
}
