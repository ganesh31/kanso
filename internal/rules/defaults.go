package rules

import (
	"kanso/internal/market"
	"kanso/internal/patterns"
)

// Defaults from spec §5.4–5.6 and §7.3.
const (
	DefaultStopPct       = 8.0
	DefaultTrendSMAWeeks = 30
	DefaultPerSidePct    = 0.20
	DefaultSlippagePct   = 0.10
	DefaultCapital       = 1_000_000.0
	DefaultMaxPositions  = 10
)

// DefaultUniverse is the stock list the default system runs on.
const DefaultUniverse = "NIFTY50"

// DefaultSystem is the system the Builder opens on (spec §7.5): a new
// 3-year high with every default filter required (volume surge, relative
// strength, not extended; not the optional level age), on daily bars,
// with the spec's exits, costs and portfolio. Each condition carries all
// of its default params. Every call returns a fresh copy.
func DefaultSystem() System {
	reg := patterns.Default()
	types := []string{patterns.TypeNYearHigh, patterns.TypeVolumeSurge, patterns.TypeRelativeStrength, patterns.TypeNotExtended}
	all := make([]Condition, len(types))
	for i, typ := range types {
		pat, _ := reg.Get(typ)
		params, err := patterns.Normalize(pat.Meta(), nil)
		if err != nil {
			panic("rules: default params of " + typ + ": " + err.Error())
		}
		all[i] = Condition{Type: typ, Params: params}
	}
	s := System{Name: "3-year breakout", Universe: DefaultUniverse, Entry: Entry{All: all}}
	ApplyDefaults(&s)
	return s
}

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
