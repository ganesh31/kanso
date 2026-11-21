package backtest

import "slices"

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
