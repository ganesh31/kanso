package backtest

import (
	"math"
	"testing"

	"kanso/internal/market"
)

func TestEquityExecutorWholeShares(t *testing.T) {
	ex := EquityExecutor{SlippagePct: 0.10}
	inst := market.Instrument{Symbol: "MRF", Type: market.Equity, LotSize: 50}
	if got := ex.Quantity(100000, 4105.20, inst); got != 24 {
		t.Errorf("Quantity(100000, 4105.20) = %d, want 24", got)
	}
	if got := ex.Quantity(100000, 131000, inst); got != 0 {
		t.Errorf("Quantity(100000, 131000) = %d, want 0 (can't afford one share)", got)
	}
}

func TestEquityExecutorSlippageBothSides(t *testing.T) {
	ex := EquityExecutor{SlippagePct: 0.10}
	if got := ex.FillPrice(100, Buy); math.Abs(got-100.1) > 1e-9 {
		t.Errorf("buy fill = %v, want 100.1", got)
	}
	if got := ex.FillPrice(100, Sell); math.Abs(got-99.9) > 1e-9 {
		t.Errorf("sell fill = %v, want 99.9", got)
	}
}

func TestPercentCostPerSide(t *testing.T) {
	cm := PercentCost{PerSidePct: 0.20}
	for _, side := range []Side{Buy, Sell} {
		if got := cm.Cost(50000, side); math.Abs(got-100) > 1e-9 {
			t.Errorf("Cost(50000, %v) = %v, want 100", side, got)
		}
	}
}
