package rules

import (
	"fmt"
	"slices"
	"strings"

	"kanso/internal/market"
	"kanso/internal/patterns"
)

// FieldError is one problem in a system, at a JSON path such as
// "entry.any" or "entry.all[0].params".
type FieldError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ValidationError lists every problem found in a system.
type ValidationError struct {
	Errors []FieldError
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, fe := range e.Errors {
		msgs[i] = fe.Path + ": " + fe.Message
	}
	return "invalid system: " + strings.Join(msgs, "; ")
}

func (e *ValidationError) add(path, format string, args ...any) {
	e.Errors = append(e.Errors, FieldError{Path: path, Message: fmt.Sprintf(format, args...)})
}

// Ranges for exits, costs and the portfolio.
const (
	maxStopPct       = 50
	minTrendSMAWeeks = 5
	maxTrendSMAWeeks = 100
	maxCostPct       = 5
	maxMaxPositions  = 50
)

// Validate checks s against the registry and normalizes every condition's
// params in place (defaults filled, numbers as float64). It returns a
// *ValidationError listing every problem, or nil.
func Validate(s *System, reg *patterns.Registry) error {
	ve := &ValidationError{}
	if s.Timeframe != market.Daily && s.Timeframe != market.Weekly {
		ve.add("timeframe", "must be daily or weekly")
	}
	validateEntry(&s.Entry, reg, ve)

	if s.Exits.StopPct <= 0 || s.Exits.StopPct > maxStopPct {
		ve.add("exits.stop_pct", "must be more than 0 and at most %d", maxStopPct)
	}
	if n := s.Exits.TrendSMAWeeks; n != 0 && (n < minTrendSMAWeeks || n > maxTrendSMAWeeks) {
		ve.add("exits.trend_sma_weeks", "must be 0 (off) or between %d and %d", minTrendSMAWeeks, maxTrendSMAWeeks)
	}
	if c := s.Costs.PerSidePct; c < 0 || c > maxCostPct {
		ve.add("costs.per_side_pct", "must be between 0 and %d", maxCostPct)
	}
	if c := s.Costs.SlippagePct; c < 0 || c > maxCostPct {
		ve.add("costs.slippage_pct", "must be between 0 and %d", maxCostPct)
	}
	if s.Portfolio.Capital <= 0 {
		ve.add("portfolio.capital", "must be more than 0")
	}
	if n := s.Portfolio.MaxPositions; n < 1 || n > maxMaxPositions {
		ve.add("portfolio.max_positions", "must be between 1 and %d", maxMaxPositions)
	}
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}

// validateEntry checks the entry rule and normalizes its params.
func validateEntry(e *Entry, reg *patterns.Registry, ve *ValidationError) {
	if len(e.All) == 0 && len(e.Any) == 0 {
		ve.add("entry.all", "needs at least one condition")
	}
	if len(e.Any) == 1 {
		ve.add("entry.any", "needs at least 2 conditions (a single condition belongs in all)")
	}

	provided := map[string]bool{}
	type requirer struct {
		path string
		typ  string
		need []string
	}
	var requirers []requirer
	// providerSeen maps a context value to the path of the condition that
	// provides it: each value (the breakout level) has one provider.
	providerSeen := map[string]string{}

	check := func(group string, conds []Condition) {
		for i := range conds {
			c := &conds[i]
			path := fmt.Sprintf("entry.%s[%d]", group, i)
			pat, ok := reg.Get(c.Type)
			if !ok {
				ve.add(path+".type", "unknown condition type %q", c.Type)
				continue
			}
			m := pat.Meta()
			if len(m.Provides) > 0 {
				if group != "all" {
					ve.add(path+".type", "%s provides %s, so it may appear only in all", c.Type, strings.Join(m.Provides, ", "))
				} else {
					for _, p := range m.Provides {
						if first, dup := providerSeen[p]; dup {
							ve.add(path+".type", "%s may be provided only once (already by %s)", p, first)
							continue
						}
						providerSeen[p] = path
						provided[p] = true
					}
				}
			}
			if len(m.Requires) > 0 {
				requirers = append(requirers, requirer{path: path + ".type", typ: c.Type, need: m.Requires})
			}
			np, err := patterns.Normalize(m, c.Params)
			if err != nil {
				ve.add(path+".params", "%s", err.Error())
				continue
			}
			c.Params = np
		}
	}
	check("all", e.All)
	check("any", e.Any)
	checkOneBenchmark(e, ve)

	for _, r := range requirers {
		for _, need := range r.need {
			if !provided[need] {
				ve.add(r.path, "%s needs a %s; add a provider such as %s to all", r.typ, need, providersOf(reg, need))
			}
		}
	}
}

// providersOf names the registered patterns that provide a context value.
func providersOf(reg *patterns.Registry, value string) string {
	var out []string
	for _, m := range reg.All() {
		if slices.Contains(m.Provides, value) {
			out = append(out, m.Type)
		}
	}
	return strings.Join(out, " or ")
}

// checkOneBenchmark rejects relative-strength conditions that name
// different benchmarks: report, scan and backtest load one benchmark per
// system (Entry.Benchmark), so a second one would be silently ignored.
// It runs after normalization, so an omitted benchmark is the default.
func checkOneBenchmark(e *Entry, ve *ValidationError) {
	first := ""
	for _, g := range []struct {
		name  string
		conds []Condition
	}{{"all", e.All}, {"any", e.Any}} {
		for i, c := range g.conds {
			if c.Type != patterns.TypeRelativeStrength {
				continue
			}
			b := c.Params.String("benchmark")
			if b == "" {
				b = patterns.DefaultBenchmark
			}
			if first == "" {
				first = b
			} else if b != first {
				ve.add(fmt.Sprintf("entry.%s[%d].params.benchmark", g.name, i),
					"every relative strength condition must use the same benchmark (%s)", first)
			}
		}
	}
}
