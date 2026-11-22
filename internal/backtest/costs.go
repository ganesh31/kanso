package backtest

import (
	"math"

	"kanso/internal/market"
)

// Side is the direction of a fill.
type Side int

// Sides.
const (
	Buy Side = iota
	Sell
)

func (s Side) String() string {
	if s == Sell {
		return "sell"
	}
	return "buy"
}

// CostModel is what a fill costs in charges (brokerage and statutory),
// on top of slippage (spec §5.5).
type CostModel interface {
	// Cost is the charge, in rupees, on a fill of notional rupees.
	Cost(notional float64, side Side) float64
}

// PercentCost charges one combined percentage per side.
type PercentCost struct {
	PerSidePct float64
}

// Cost implements CostModel.
func (c PercentCost) Cost(notional float64, _ Side) float64 {
	return notional * c.PerSidePct / 100
}

// Executor turns a reference price into a fill and sizes positions. V1
// trades equities; futures and options are seams for later (spec §9).
type Executor interface {
	// FillPrice is the price paid (Buy) or received (Sell) for ref.
	FillPrice(ref float64, side Side) float64
	// Quantity is how many units capital buys at price, which already
	// includes the per-side cost. 0 means not even one unit is affordable.
	Quantity(capital, price float64, inst market.Instrument) int64
}

// EquityExecutor fills equities with slippage on both sides and sizes in
// whole shares. LotSize is ignored for equities.
type EquityExecutor struct {
	SlippagePct float64
}

// FillPrice implements Executor: buy = ref×(1+s), sell = ref×(1−s).
func (e EquityExecutor) FillPrice(ref float64, side Side) float64 {
	s := e.SlippagePct / 100
	if side == Sell {
		return ref * (1 - s)
	}
	return ref * (1 + s)
}

// Quantity implements Executor: floor(capital / price) whole shares.
func (EquityExecutor) Quantity(capital, price float64, _ market.Instrument) int64 {
	if price <= 0 || capital <= 0 {
		return 0
	}
	return int64(math.Floor(capital / price))
}

var (
	_ Executor  = EquityExecutor{}
	_ CostModel = PercentCost{}
)
