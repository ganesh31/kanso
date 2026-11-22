package backtest

import (
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"kanso/internal/market"
)

func curve(dates []market.Date, values ...float64) []EquityPoint {
	out := make([]EquityPoint, len(values))
	for i, v := range values {
		out[i] = EquityPoint{Date: dates[i], Value: v}
	}
	return out
}

func TestMaxDrawdownAndCAGR(t *testing.T) {
	eq := curve([]market.Date{"2020-01-01", "2021-01-01", "2022-01-01", "2023-01-01"}, 100, 120, 90, 130)
	if got := MaxDrawdown(eq); !near(got, -0.25) {
		t.Fatalf("MaxDrawdown = %v, want -0.25 (120 → 90)", got)
	}
	if got, want := CAGR(eq), math.Pow(1.3, 1.0/3)-1; !near(got, want) {
		t.Fatalf("CAGR = %v, want %v (130/100 over exactly 3 years)", got, want)
	}
}

func TestCAGRPartYear(t *testing.T) {
	// 2023-01-01 → 2024-07-01 is 1 year and 182 days, 182/366 of leap 2024.
	eq := curve([]market.Date{"2023-01-01", "2024-07-01"}, 100, 150)
	want := math.Pow(1.5, 1/(1+182.0/366)) - 1
	if got := CAGR(eq); !near(got, want) {
		t.Fatalf("CAGR = %v, want %v", got, want)
	}
}

func TestMetricsDegenerateCurves(t *testing.T) {
	if CAGR(nil) != 0 || MaxDrawdown(nil) != 0 {
		t.Fatal("empty curve must give zeros")
	}
	one := curve([]market.Date{"2020-01-01"}, 100)
	if CAGR(one) != 0 || MaxDrawdown(one) != 0 {
		t.Fatal("a one-point curve must give zeros")
	}
	if got := MaxDrawdown(curve([]market.Date{"2020-01-01", "2020-01-02"}, 100, 110)); got != 0 {
		t.Fatalf("rising curve MaxDrawdown = %v, want 0", got)
	}
}

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
