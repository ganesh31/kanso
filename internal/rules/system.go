// Package rules is the System model (design §7.3): an entry rule of
// required conditions and one optional ANY group, exits, costs and a
// portfolio. It validates systems, compiles entry rules and evaluates them
// with a full explanation.
package rules

import (
	"kanso/internal/market"
	"kanso/internal/patterns"
)

// Condition is one pattern or filter with its params.
type Condition struct {
	Type   string          `json:"type"`
	Params patterns.Params `json:"params,omitempty"`
}

// Entry is the entry rule: every condition in All, and at least one in
// Any when Any is non-empty (spec §5.5).
type Entry struct {
	All []Condition `json:"all"`
	Any []Condition `json:"any,omitempty"`
}

// Exits are the exit rules (spec §5.4). TrendSMAWeeks 0 disables the
// trend exit.
type Exits struct {
	StopPct       float64 `json:"stop_pct" doc:"Stop-loss, % below the entry fill"`
	TrendSMAWeeks int     `json:"trend_sma_weeks" doc:"Exit when a weekly close is below this SMA of weekly closes; 0 disables"`
}

// Costs are charged on every fill, per side, in percent (spec §5.5).
type Costs struct {
	PerSidePct  float64 `json:"per_side_pct" doc:"Brokerage and statutory charges, % per side"`
	SlippagePct float64 `json:"slippage_pct" doc:"Slippage, % per side"`
}

// Portfolio sizes positions (spec §5.6).
type Portfolio struct {
	Capital      float64 `json:"capital" doc:"Starting capital in rupees"`
	MaxPositions int     `json:"max_positions" doc:"Maximum concurrent positions"`
}

// System is a complete trading system, the canonical JSON of design §7.3.
type System struct {
	ID        string           `json:"id,omitempty"`
	Name      string           `json:"name"`
	Universe  string           `json:"universe"`
	Timeframe market.Timeframe `json:"timeframe,omitempty" enum:"daily,weekly"`
	Entry     Entry            `json:"entry"`
	Exits     Exits            `json:"exits,omitempty"`
	Costs     Costs            `json:"costs,omitempty"`
	Portfolio Portfolio        `json:"portfolio,omitempty"`
}
