package backtest

import (
	"math"
	"testing"
	"time"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// flatDays returns n daily bars on consecutive weekdays from Monday
// 2024-01-01, each with open = high = low = close = 100.
func flatDays(n int) []market.Bar {
	bars := make([]market.Bar, 0, n)
	d := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for len(bars) < n {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			bars = append(bars, market.Bar{Date: market.Date(d.Format(market.DateLayout)), Open: 100, High: 100, Low: 100, Close: 100, Volume: 1000})
		}
		d = d.AddDate(0, 0, 1)
	}
	return bars
}

// setBar overwrites bar i's open, low and close (high = max of the three).
func setBar(bars []market.Bar, i int, open, low, close float64) {
	bars[i].Open, bars[i].Low, bars[i].Close = open, low, close
	bars[i].High = max(open, low, close)
}

// sys is the default exits and costs with the given trend SMA weeks.
func sys(trendWeeks int) *rules.System {
	return &rules.System{
		Timeframe: market.Daily,
		Exits:     rules.Exits{StopPct: 8, TrendSMAWeeks: trendWeeks},
		Costs:     rules.Costs{PerSidePct: 0.20, SlippagePct: 0.10},
	}
}

// path runs TradePath on daily bars for a signal at sig, with default
// costs and slippage.
func path(t *testing.T, bars []market.Bar, sig int, s *rules.System) Trade {
	t.Helper()
	daily := market.NewDaily(market.Instrument{Symbol: "T", Type: market.Equity}, bars)
	ctx := patterns.NewContext(daily, nil)
	tr, ok := TradePath(sig, ctx, daily, s, EquityExecutor{SlippagePct: s.Costs.SlippagePct}, PercentCost{PerSidePct: s.Costs.PerSidePct})
	if !ok {
		t.Fatal("TradePath returned no trade")
	}
	return tr
}

func near(a, b float64) bool { return math.Abs(a-b) <= 1e-9*max(1, math.Abs(b)) }

func TestEntryNextDayOpen(t *testing.T) {
	bars := flatDays(30)
	bars[11].Open = 104
	tr := path(t, bars, 10, sys(0))
	if tr.SignalDate != bars[10].Date || tr.EntryDate != bars[11].Date {
		t.Fatalf("signal %s entry %s, want %s and %s", tr.SignalDate, tr.EntryDate, bars[10].Date, bars[11].Date)
	}
	if !near(tr.EntryPrice, 104*1.001) {
		t.Fatalf("EntryPrice = %v, want %v", tr.EntryPrice, 104*1.001)
	}
}

func TestNoTradeWithoutNextBar(t *testing.T) {
	bars := flatDays(5)
	daily := market.NewDaily(market.Instrument{Symbol: "T"}, bars)
	s := sys(0)
	if _, ok := TradePath(4, patterns.NewContext(daily, nil), daily, s, EquityExecutor{}, PercentCost{}); ok {
		t.Fatal("signal on the last bar must not trade")
	}
}

func TestWeeklyEntryFirstBarOfNextWeek(t *testing.T) {
	// Week of Mon 2024-01-08: Friday 2024-01-12 is a holiday, so the week
	// ends on Thursday. The next bar is Monday 2024-01-15.
	var bars []market.Bar
	for _, b := range flatDays(25) {
		if b.Date != "2024-01-12" {
			bars = append(bars, b)
		}
	}
	daily := market.NewDaily(market.Instrument{Symbol: "T"}, bars)
	weekly := market.ToWeekly(daily)
	sig := 1
	if weekly.Bars[sig].Date != "2024-01-11" {
		t.Fatalf("week 1 dated %s, want Thursday 2024-01-11", weekly.Bars[sig].Date)
	}
	s := sys(0)
	s.Timeframe = market.Weekly
	tr, ok := TradePath(sig, patterns.NewContext(weekly, nil), daily, s, EquityExecutor{SlippagePct: 0.1}, PercentCost{PerSidePct: 0.2})
	if !ok {
		t.Fatal("no trade")
	}
	if tr.SignalDate != "2024-01-11" || tr.EntryDate != "2024-01-15" {
		t.Fatalf("signal %s entry %s, want 2024-01-11 and Monday 2024-01-15", tr.SignalDate, tr.EntryDate)
	}
}

// entryAt100 makes bar e open so the entry fill (0.1% slippage) is exactly
// 100, putting the 8% stop at 92.
func entryAt100(bars []market.Bar, e int) {
	bars[e].Open = 100 / 1.001
}

func TestStopGapDownFillsAtOpen(t *testing.T) {
	bars := flatDays(30)
	entryAt100(bars, 11)
	setBar(bars, 12, 90, 88, 89)
	tr := path(t, bars, 10, sys(0))
	if tr.ExitReason != ExitStop || tr.ExitDate != bars[12].Date || !tr.GappedStop {
		t.Fatalf("got %s on %s gapped=%v, want a gapped stop on %s", tr.ExitReason, tr.ExitDate, tr.GappedStop, bars[12].Date)
	}
	if !near(tr.ExitPrice, 90*0.999) {
		t.Fatalf("ExitPrice = %v, want %v (the open, less slippage)", tr.ExitPrice, 90*0.999)
	}
}

func TestStopIntradayFillsAtStop(t *testing.T) {
	bars := flatDays(30)
	entryAt100(bars, 11)
	setBar(bars, 12, 95, 91, 93)
	tr := path(t, bars, 10, sys(0))
	if tr.ExitReason != ExitStop || tr.ExitDate != bars[12].Date || tr.GappedStop {
		t.Fatalf("got %s on %s gapped=%v, want an intraday stop on %s", tr.ExitReason, tr.ExitDate, tr.GappedStop, bars[12].Date)
	}
	if !near(tr.ExitPrice, 92*0.999) {
		t.Fatalf("ExitPrice = %v, want %v (the stop, less slippage)", tr.ExitPrice, 92*0.999)
	}
}

func TestStopCheckedOnEntryDay(t *testing.T) {
	bars := flatDays(30)
	setBar(bars, 11, 100/1.001, 91, 93)
	tr := path(t, bars, 10, sys(0))
	if tr.ExitReason != ExitStop || tr.ExitDate != bars[11].Date || tr.HoldingDays != 0 {
		t.Fatalf("got %s on %s after %d days, want a stop on the entry day %s", tr.ExitReason, tr.ExitDate, tr.HoldingDays, bars[11].Date)
	}
	if !near(tr.ExitPrice, 92*0.999) {
		t.Fatalf("ExitPrice = %v, want %v", tr.ExitPrice, 92*0.999)
	}
}

func TestStopNotTouched(t *testing.T) {
	bars := flatDays(30)
	entryAt100(bars, 11)
	setBar(bars, 12, 95, 92.0001, 93) // low stays above the stop
	if tr := path(t, bars, 10, sys(0)); tr.ExitReason != ExitOpen {
		t.Fatalf("got %s on %s, want the trade still open", tr.ExitReason, tr.ExitDate)
	}
}

// trendDays is 12 flat weeks (60 bars, Mon–Fri) where week 7 (bars
// 35–39) closes at 98: below its 5-week SMA of 99.6, so a 5-week trend
// exit fires at bar 40's open, the Monday after.
func trendDays() []market.Bar {
	bars := flatDays(60)
	setBar(bars, 39, 100, 98, 98)
	return bars
}

func TestTrendExitNextWeekOpen(t *testing.T) {
	bars := trendDays()
	bars[40].Open = 101
	tr := path(t, bars, 26, sys(5))
	if tr.ExitReason != ExitTrend || tr.ExitDate != bars[40].Date {
		t.Fatalf("got %s on %s, want trend on %s (first trading day after the week)", tr.ExitReason, tr.ExitDate, bars[40].Date)
	}
	if !near(tr.ExitPrice, 101*0.999) || tr.HoldingDays != 13 {
		t.Fatalf("ExitPrice %v after %d days, want %v after 13", tr.ExitPrice, tr.HoldingDays, 101*0.999)
	}
}

func TestTrendExitIgnoresWeeksBeforeEntry(t *testing.T) {
	bars := flatDays(60)
	setBar(bars, 24, 100, 98, 98) // week 4 closes low; entry is bar 26, in week 5
	if tr := path(t, bars, 25, sys(5)); tr.ExitReason != ExitOpen {
		t.Fatalf("got %s on %s, want the trade still open", tr.ExitReason, tr.ExitDate)
	}
}

func TestTrendExitNeedsNCompleteWeeks(t *testing.T) {
	bars := flatDays(60)
	setBar(bars, 14, 100, 98, 98) // week 2: only 3 weeks of closes so far
	if tr := path(t, bars, 5, sys(5)); tr.ExitReason != ExitOpen {
		t.Fatalf("got %s on %s, want no trend exit before 5 weeks exist", tr.ExitReason, tr.ExitDate)
	}
}

func TestTrendExitOffWhenZero(t *testing.T) {
	if tr := path(t, trendDays(), 26, sys(0)); tr.ExitReason != ExitOpen {
		t.Fatalf("got %s, want no trend exit with trend_sma_weeks 0", tr.ExitReason)
	}
}

func TestTrendExitBeforeStopSameDay(t *testing.T) {
	bars := trendDays()
	entryAt100(bars, 27)
	setBar(bars, 40, 91, 85, 86) // opens below the 92 stop on the trend-exit day
	tr := path(t, bars, 26, sys(5))
	if tr.ExitReason != ExitTrend || tr.ExitDate != bars[40].Date || tr.GappedStop {
		t.Fatalf("got %s on %s gapped=%v, want trend on %s", tr.ExitReason, tr.ExitDate, tr.GappedStop, bars[40].Date)
	}
	if !near(tr.ExitPrice, 91*0.999) {
		t.Fatalf("ExitPrice = %v, want %v (that day's open)", tr.ExitPrice, 91*0.999)
	}
}

func TestReturnNetOfCosts(t *testing.T) {
	bars := trendDays()
	bars[27].Open = 100
	bars[40].Open = 110
	tr := path(t, bars, 26, sys(5))
	// Buy 100×1.001 = 100.1, paying 100.1×1.002 = 100.3002 with costs.
	// Sell 110×0.999 = 109.89, keeping 109.89×0.998 = 109.67022.
	const want = 9.341975389879575
	if tr.ExitReason != ExitTrend || !near(tr.ReturnPct, want) {
		t.Fatalf("%s ReturnPct = %v, want %v", tr.ExitReason, tr.ReturnPct, want)
	}
}

func TestOpenTradeReturnHasNoExitCost(t *testing.T) {
	bars := flatDays(30)
	bars[11].Open = 100
	bars[29].Close = 110
	tr := path(t, bars, 10, sys(0))
	// 110 / (100.1 × 1.002) − 1
	const want = 9.67076835340308
	if !near(tr.ReturnPct, want) {
		t.Fatalf("ReturnPct = %v, want %v", tr.ReturnPct, want)
	}
}

func TestSingleStockSkipsSignalsWhileTradeOpen(t *testing.T) {
	// Signals at 2, 4 and 8. The trade from 2 enters at 3 and is stopped at
	// 6, so the signal at 4 (entry 5) is skipped and the one at 8 taken.
	pass := make([]bool, 20)
	pass[2], pass[4], pass[8] = true, true, true
	c, ctx := stubRule(t, pass, nil)
	bars := flatDays(20)
	setBar(bars, 6, 95, 80, 85)
	ctx.Series.Bars = bars
	ctx.Series.Instrument.Symbol = "T"
	s := sys(0)
	got := singleStockTrades(Signals(c, ctx), ctx, ctx.Series, s, EquityExecutor{SlippagePct: 0.1}, PercentCost{PerSidePct: 0.2})
	if len(got) != 2 || got[0].SignalDate != bars[2].Date || got[1].SignalDate != bars[8].Date {
		t.Fatalf("got %+v, want trades from the signals at bars 2 and 8", got)
	}
}

func TestOpenTradeAtEndOfData(t *testing.T) {
	bars := flatDays(30)
	bars[29].Close = 120
	tr := path(t, bars, 10, sys(0))
	if tr.ExitReason != ExitOpen || tr.ExitDate != bars[29].Date || tr.ExitPrice != 120 {
		t.Fatalf("got %s on %s at %v, want open on %s at the last close 120", tr.ExitReason, tr.ExitDate, tr.ExitPrice, bars[29].Date)
	}
	if tr.HoldingDays != 18 {
		t.Fatalf("HoldingDays = %d, want 18 (bar 11 to bar 29)", tr.HoldingDays)
	}
}
