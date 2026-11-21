package backtest

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func trades(returns []float64, days []int) []Trade {
	out := make([]Trade, len(returns))
	for i := range returns {
		out[i] = Trade{ReturnPct: returns[i], HoldingDays: days[i]}
	}
	return out
}

func TestTradeStatsValues(t *testing.T) {
	got := ComputeTradeStats(trades([]float64{10, -5, 20, -5}, []int{3, 10, 5, 7}))
	want := TradeStats{Trades: 4, WinRate: 0.5, AvgWinPct: 15, AvgLossPct: -5, ExpectancyPct: 5, MedianHoldingDays: 6}
	if diff := cmp.Diff(want, got, cmpopts.EquateApprox(1e-9, 0)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestTradeStatsOddCountMedianAndNoLosses(t *testing.T) {
	got := ComputeTradeStats(trades([]float64{4, 2, 6}, []int{9, 1, 4}))
	want := TradeStats{Trades: 3, WinRate: 1, AvgWinPct: 4, AvgLossPct: 0, ExpectancyPct: 4, MedianHoldingDays: 4}
	if diff := cmp.Diff(want, got, cmpopts.EquateApprox(1e-9, 0)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestTradeStatsEmpty(t *testing.T) {
	if got := ComputeTradeStats(nil); got != (TradeStats{}) {
		t.Fatalf("got %+v, want zeros", got)
	}
}

func TestTradeStatsZeroReturnIsALoss(t *testing.T) {
	got := ComputeTradeStats(trades([]float64{0, 10}, []int{1, 1}))
	if got.WinRate != 0.5 || got.AvgLossPct != 0 || got.AvgWinPct != 10 {
		t.Fatalf("got %+v, want a flat trade counted as a non-win", got)
	}
}
