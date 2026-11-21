package rules

import "kanso/internal/market"

// Defaults from spec §5.4–5.6 and §7.3.
const (
	DefaultStopPct       = 8.0
	DefaultTrendSMAWeeks = 30
	DefaultPerSidePct    = 0.20
	DefaultSlippagePct   = 0.10
	DefaultCapital       = 1_000_000.0
	DefaultMaxPositions  = 10
)

// ApplyDefaults fills the spec defaults into the zero-valued parts of s.
// Exits and costs are filled only when the whole block is zero, because a
// zero field inside them is meaningful (no trend exit, no slippage). The
// portfolio is filled field by field, and the timeframe defaults to daily.
func ApplyDefaults(s *System) {
	if s.Timeframe == "" {
		s.Timeframe = market.Daily
	}
	if s.Exits == (Exits{}) {
		s.Exits = Exits{StopPct: DefaultStopPct, TrendSMAWeeks: DefaultTrendSMAWeeks}
	}
	if s.Costs == (Costs{}) {
		s.Costs = Costs{PerSidePct: DefaultPerSidePct, SlippagePct: DefaultSlippagePct}
	}
	if s.Portfolio.Capital == 0 {
		s.Portfolio.Capital = DefaultCapital
	}
	if s.Portfolio.MaxPositions == 0 {
		s.Portfolio.MaxPositions = DefaultMaxPositions
	}
}
