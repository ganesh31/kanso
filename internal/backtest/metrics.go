package backtest

import (
	"math"
	"slices"

	"kanso/internal/market"
)

// PortfolioStats are the portfolio metrics (spec §5.7). Returns and
// drawdowns are fractions: 0.12 is 12%, a drawdown is ≤ 0.
type PortfolioStats struct {
	CAGR        float64
	MaxDrawdown float64
	FinalEquity float64
	// Exposure is the fraction of trading days with a position held, 0–1.
	Exposure float64
	// BenchCAGR and BenchMaxDrawdown are buy-and-hold Nifty's.
	BenchCAGR        float64
	BenchMaxDrawdown float64
}

// CAGR is the curve's compound annual growth rate from its first point to
// its last, or 0 for fewer than two points, a zero span or a non-positive
// start or end.
func CAGR(eq []EquityPoint) float64 {
	if len(eq) < 2 {
		return 0
	}
	first, last := eq[0], eq[len(eq)-1]
	y := years(first.Date, last.Date)
	if y <= 0 || first.Value <= 0 || last.Value <= 0 {
		return 0
	}
	return math.Pow(last.Value/first.Value, 1/y) - 1
}

// years is the time from a to b in years: whole calendar years, plus the
// remaining days over the length of the year they fall in.
func years(a, b market.Date) float64 {
	t, end := a.Time(), b.Time()
	n := 0
	for !t.AddDate(n+1, 0, 0).After(end) {
		n++
	}
	from, to := t.AddDate(n, 0, 0), t.AddDate(n+1, 0, 0)
	return float64(n) + end.Sub(from).Hours()/to.Sub(from).Hours()
}

// MaxDrawdown is the deepest fall from a running peak, as a fraction ≤ 0.
func MaxDrawdown(eq []EquityPoint) float64 {
	peak, mdd := math.Inf(-1), 0.0
	for _, p := range eq {
		peak = max(peak, p.Value)
		if peak > 0 {
			mdd = min(mdd, p.Value/peak-1)
		}
	}
	return mdd
}

// TradeStats are the trade-level metrics (spec §5.7), over single-stock
// trades with no slot limits.
type TradeStats struct {
	Trades int
	// WinRate is the fraction of trades with a positive return, 0–1.
	WinRate float64
	// AvgWinPct and AvgLossPct are mean returns in percent of the winners
	// and of the rest (flat trades count as losses); 0 when there are none.
	AvgWinPct  float64
	AvgLossPct float64
	// ExpectancyPct is the mean return per trade, in percent.
	ExpectancyPct float64
	// MedianHoldingDays is in trading days; with an even count it is the
	// two middle values' mean, rounded down.
	MedianHoldingDays int
}

// ComputeTradeStats summarizes trades.
func ComputeTradeStats(ts []Trade) TradeStats {
	n := len(ts)
	if n == 0 {
		return TradeStats{}
	}
	var wins, losses int
	var winSum, lossSum float64
	days := make([]int, n)
	for i, t := range ts {
		if t.ReturnPct > 0 {
			wins++
			winSum += t.ReturnPct
		} else {
			losses++
			lossSum += t.ReturnPct
		}
		days[i] = t.HoldingDays
	}
	st := TradeStats{Trades: n, WinRate: float64(wins) / float64(n), ExpectancyPct: (winSum + lossSum) / float64(n)}
	if wins > 0 {
		st.AvgWinPct = winSum / float64(wins)
	}
	if losses > 0 {
		st.AvgLossPct = lossSum / float64(losses)
	}
	slices.Sort(days)
	if n%2 == 1 {
		st.MedianHoldingDays = days[n/2]
	} else {
		st.MedianHoldingDays = (days[n/2-1] + days[n/2]) / 2
	}
	return st
}
