package backtest

import (
	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// ExitReason says why a trade ended.
type ExitReason string

// Exit reasons (spec §5.4).
const (
	ExitStop  ExitReason = "stop"
	ExitTrend ExitReason = "trend"
	// ExitOpen marks a trade still open at the end of data, valued at the
	// last close.
	ExitOpen ExitReason = "open"
)

// Trade is one round trip in one stock.
type Trade struct {
	Symbol string
	// SignalDate is the signal bar: a day, or on weekly the last trading
	// day of the signal week.
	SignalDate market.Date
	EntryDate  market.Date
	ExitDate   market.Date
	// EntryPrice and ExitPrice are fills, after slippage but before costs.
	EntryPrice float64
	ExitPrice  float64
	ExitReason ExitReason
	// GappedStop is true when the day opened at or below the stop, so the
	// stop filled at the open.
	GappedStop bool
	// ReturnPct is the per-share return in percent, net of costs and
	// slippage on both sides (no exit cost on an open trade).
	ReturnPct float64
	// HoldingDays is the number of trading days from entry to exit.
	HoldingDays int
	// RankKey orders crowded entry days: the 6-month RS diff at the signal
	// (spec §5.6), or -Inf when not evaluable. Set by Run, not by
	// TradePath. It is not serialized: JSON has no -Inf.
	RankKey float64 `json:"-"`
	// Level and LevelDate are the breakout level at the signal bar and
	// the date it was set, for the trade chart; nil when the rule has no
	// breakout level. Set by Run, not by TradePath.
	Level     *float64
	LevelDate *market.Date
	// Quantity is the whole shares the portfolio bought; 0 outside it.
	Quantity int64
}

// TradePath follows one signal at bar sig of ctx.Series through its daily
// bars (spec §5.4, §5.5). It returns false when there is no next daily
// bar to enter on.
//
//   - Entry: the next daily bar's open (weekly: the first bar of the next
//     week, the bar that proves the signal week complete), plus slippage.
//   - Stop: stop_pct below the entry fill, checked from the entry day on.
//     On a day whose low reaches it, the exit reference is min(open, stop)
//     and GappedStop is set when the open was at or below the stop.
//   - Trend exit: when a complete week ending on or after the entry day
//     closes below its trend_sma_weeks SMA of weekly closes, exit at the
//     next trading day's open. It comes before that day's stop check.
//   - End of data: reason "open" at the last close, with no exit costs.
func TradePath(sig int, ctx *patterns.Context, daily *market.Series, sys *rules.System, ex Executor, cm CostModel) (Trade, bool) {
	e := sig + 1
	if ctx.Series.Timeframe == market.Weekly {
		e = ctx.Series.LastDailyIdx[sig] + 1
	}
	bars := daily.Bars
	if sig < 0 || e >= len(bars) {
		return Trade{}, false
	}
	entry := ex.FillPrice(bars[e].Open, Buy)
	tr := Trade{
		Symbol:     daily.Instrument.Symbol,
		SignalDate: ctx.Series.Bars[sig].Date,
		EntryDate:  bars[e].Date,
		EntryPrice: entry,
	}

	trendDay := trendExitDay(daily, e, sys.Exits.TrendSMAWeeks)
	stop := entry * (1 - sys.Exits.StopPct/100)
	for d := e; d < len(bars); d++ {
		b := bars[d]
		if d == trendDay {
			return exit(tr, d, b, b.Open, ExitTrend, e, ex, cm), true
		}
		if sys.Exits.StopPct > 0 && b.Low <= stop {
			tr.GappedStop = b.Open <= stop
			return exit(tr, d, b, min(b.Open, stop), ExitStop, e, ex, cm), true
		}
	}

	last := len(bars) - 1
	tr.ExitDate, tr.ExitPrice, tr.ExitReason = bars[last].Date, bars[last].Close, ExitOpen
	tr.HoldingDays = last - e
	tr.ReturnPct = returnPct(entry, tr.ExitPrice, cm, false)
	return tr, true
}

// trendExitDay is the daily index whose open the trend exit fills at, for
// a trade entered on daily bar e, or -1 when it never fires (or n is 0).
// It is the day after the first complete week, ending on or after e, whose
// close is below the SMA of the last n weekly closes including its own.
func trendExitDay(daily *market.Series, e, n int) int {
	if n <= 0 {
		return -1
	}
	w := market.ToWeekly(daily)
	sum := 0.0
	for i := 0; i < w.Complete; i++ {
		sum += w.Bars[i].Close
		if i >= n {
			sum -= w.Bars[i-n].Close
		}
		if i < n-1 || w.LastDailyIdx[i] < e {
			continue
		}
		if w.Bars[i].Close < sum/float64(n) {
			// A complete week always has a later daily bar.
			return w.LastDailyIdx[i] + 1
		}
	}
	return -1
}

// exit closes tr on daily bar d (bar b) at reference price ref, entered on
// daily bar e.
func exit(tr Trade, d int, b market.Bar, ref float64, reason ExitReason, e int, ex Executor, cm CostModel) Trade {
	tr.ExitDate, tr.ExitReason = b.Date, reason
	tr.ExitPrice = ex.FillPrice(ref, Sell)
	tr.HoldingDays = d - e
	tr.ReturnPct = returnPct(tr.EntryPrice, tr.ExitPrice, cm, true)
	return tr
}

// returnPct is the per-share return in percent: what one share's exit
// brings in after costs over what it cost with costs. exitCost is false
// for a trade still open.
func returnPct(entry, exit float64, cm CostModel, exitCost bool) float64 {
	paid := entry + cm.Cost(entry, Buy)
	got := exit
	if exitCost {
		got -= cm.Cost(exit, Sell)
	}
	return (got/paid - 1) * 100
}
