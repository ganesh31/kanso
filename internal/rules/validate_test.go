package rules

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
)

// specExample is the System JSON of design §7.3.
const specExample = `{
  "name": "3y breakout, volume or RS",
  "universe": "NIFTY500",
  "timeframe": "weekly",
  "entry": {
    "all": [ {"type": "n_year_high", "params": {"years": 3}},
             {"type": "not_extended", "params": {"pct": 8}} ],
    "any": [ {"type": "volume_surge", "params": {"k": 2, "window_weeks": 10}},
             {"type": "relative_strength", "params": {"rs_months": 6, "benchmark": "^NSEI"}} ]
  },
  "exits": { "stop_pct": 8, "trend_sma_weeks": 30 },
  "costs": { "per_side_pct": 0.20, "slippage_pct": 0.10 },
  "portfolio": { "capital": 1000000, "max_positions": 10 }
}`

func cond(typ string, p patterns.Params) Condition { return Condition{Type: typ, Params: p} }

// valid returns a default system with the given required conditions.
func valid(all ...Condition) System {
	s := System{Name: "t", Universe: "ALL", Entry: Entry{All: all}}
	ApplyDefaults(&s)
	return s
}

// fieldErrors validates s and returns its field errors (nil when valid).
func fieldErrors(t *testing.T, s System) []FieldError {
	t.Helper()
	err := Validate(&s, patterns.Default())
	if err == nil {
		return nil
	}
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error %v is not a *ValidationError", err)
	}
	return ve.Errors
}

// hasPath reports whether errs has an error at path whose message
// contains substr.
func hasPath(errs []FieldError, path, substr string) bool {
	for _, e := range errs {
		if e.Path == path && strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

func TestSystemJSONRoundTripMatchesSpecExample(t *testing.T) {
	var s System
	if err := json.Unmarshal([]byte(specExample), &s); err != nil {
		t.Fatal(err)
	}
	want := System{
		Name: "3y breakout, volume or RS", Universe: "NIFTY500", Timeframe: market.Weekly,
		Entry: Entry{
			All: []Condition{cond("n_year_high", patterns.Params{"years": 3.0}), cond("not_extended", patterns.Params{"pct": 8.0})},
			Any: []Condition{
				cond("volume_surge", patterns.Params{"k": 2.0, "window_weeks": 10.0}),
				cond("relative_strength", patterns.Params{"rs_months": 6.0, "benchmark": "^NSEI"}),
			},
		},
		Exits:     Exits{StopPct: 8, TrendSMAWeeks: 30},
		Costs:     Costs{PerSidePct: 0.20, SlippagePct: 0.10},
		Portfolio: Portfolio{Capital: 1000000, MaxPositions: 10},
	}
	if diff := cmp.Diff(want, s); diff != "" {
		t.Fatalf("unmarshal (-want +got):\n%s", diff)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var gotMap, wantMap map[string]any
	if err := json.Unmarshal(b, &gotMap); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(specExample), &wantMap); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantMap, gotMap); diff != "" {
		t.Fatalf("marshal (-want +got):\n%s", diff)
	}
}

func TestApplyDefaultsMatchesSpec(t *testing.T) {
	var s System
	ApplyDefaults(&s)
	want := System{
		Timeframe: market.Daily,
		Exits:     Exits{StopPct: 8, TrendSMAWeeks: 30},
		Costs:     Costs{PerSidePct: 0.20, SlippagePct: 0.10},
		Portfolio: Portfolio{Capital: 1000000, MaxPositions: 10},
	}
	if diff := cmp.Diff(want, s); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestApplyDefaultsKeepsSetValues(t *testing.T) {
	s := System{Timeframe: market.Weekly, Exits: Exits{StopPct: 10}, Portfolio: Portfolio{Capital: 5}}
	ApplyDefaults(&s)
	if s.Timeframe != market.Weekly || s.Exits != (Exits{StopPct: 10}) || s.Portfolio != (Portfolio{Capital: 5, MaxPositions: 10}) {
		t.Fatalf("got %+v", s)
	}
}

func TestValidateSpecExampleIsValid(t *testing.T) {
	var s System
	if err := json.Unmarshal([]byte(specExample), &s); err != nil {
		t.Fatal(err)
	}
	if errs := fieldErrors(t, s); errs != nil {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateNormalizesParams(t *testing.T) {
	s := valid(cond("n_year_high", nil))
	if err := Validate(&s, patterns.Default()); err != nil {
		t.Fatal(err)
	}
	want := patterns.Params{"years": 3.0, "all_time": false}
	if diff := cmp.Diff(want, s.Entry.All[0].Params); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestValidateRejectsUnknownType(t *testing.T) {
	errs := fieldErrors(t, valid(cond("cup_and_handle", nil)))
	if !hasPath(errs, "entry.all[0].type", "cup_and_handle") {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateRejectsBadParams(t *testing.T) {
	errs := fieldErrors(t, valid(cond("n_year_high", patterns.Params{"years": 50.0})))
	if !hasPath(errs, "entry.all[0].params", "between") {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateRejectsSingleItemAny(t *testing.T) {
	s := valid(cond("n_year_high", nil))
	s.Entry.Any = []Condition{cond("volume_surge", nil)}
	if errs := fieldErrors(t, s); !hasPath(errs, "entry.any", "at least 2") {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateRejectsEmptyEntry(t *testing.T) {
	if errs := fieldErrors(t, valid()); !hasPath(errs, "entry.all", "") {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateNYearHighOnlyInAll(t *testing.T) {
	s := valid(cond("volume_surge", nil))
	s.Entry.Any = []Condition{cond("n_year_high", nil), cond("relative_strength", nil)}
	if errs := fieldErrors(t, s); !hasPath(errs, "entry.any[0].type", "only in") {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateNYearHighAtMostOnce(t *testing.T) {
	errs := fieldErrors(t, valid(cond("n_year_high", nil), cond("n_year_high", patterns.Params{"years": 5.0})))
	if !hasPath(errs, "entry.all[1].type", "once") {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateRequiresProvider(t *testing.T) {
	errs := fieldErrors(t, valid(cond("not_extended", nil)))
	if !hasPath(errs, "entry.all[0].type", "breakout_level") {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateVolumeOnlySystemIsValid(t *testing.T) {
	if errs := fieldErrors(t, valid(cond("volume_surge", nil))); errs != nil {
		t.Fatalf("got %+v", errs)
	}
}

func TestValidateRejectsBadExitsAndPortfolio(t *testing.T) {
	cases := []struct {
		name string
		edit func(*System)
		path string
	}{
		{"stop 0", func(s *System) { s.Exits.StopPct = 0 }, "exits.stop_pct"},
		{"stop 51", func(s *System) { s.Exits.StopPct = 51 }, "exits.stop_pct"},
		{"trend 4", func(s *System) { s.Exits.TrendSMAWeeks = 4 }, "exits.trend_sma_weeks"},
		{"trend 101", func(s *System) { s.Exits.TrendSMAWeeks = 101 }, "exits.trend_sma_weeks"},
		{"positions 0", func(s *System) { s.Portfolio.MaxPositions = 0 }, "portfolio.max_positions"},
		{"positions 51", func(s *System) { s.Portfolio.MaxPositions = 51 }, "portfolio.max_positions"},
		{"capital 0", func(s *System) { s.Portfolio.Capital = 0 }, "portfolio.capital"},
		{"cost -1", func(s *System) { s.Costs.PerSidePct = -1 }, "costs.per_side_pct"},
		{"slippage 6", func(s *System) { s.Costs.SlippagePct = 6 }, "costs.slippage_pct"},
		{"timeframe", func(s *System) { s.Timeframe = "monthly" }, "timeframe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := valid(cond("volume_surge", nil))
			c.edit(&s)
			if errs := fieldErrors(t, s); !hasPath(errs, c.path, "") {
				t.Fatalf("got %+v, want error at %s", errs, c.path)
			}
		})
	}
	ok := []func(*System){
		func(s *System) { s.Exits.TrendSMAWeeks = 0 },
		func(s *System) { s.Exits.StopPct = 50 },
		func(s *System) { s.Costs = Costs{} },
	}
	for i, edit := range ok {
		s := valid(cond("volume_surge", nil))
		edit(&s)
		if errs := fieldErrors(t, s); errs != nil {
			t.Fatalf("ok case %d: got %+v", i, errs)
		}
	}
}

func TestValidateRejectsMixedBenchmarks(t *testing.T) {
	s := valid(
		cond(patterns.TypeRelativeStrength, nil),
		cond(patterns.TypeRelativeStrength, patterns.Params{"benchmark": "^NSEBANK"}),
	)
	errs := fieldErrors(t, s)
	if !hasPath(errs, "entry.all[1].params.benchmark", "same benchmark (^NSEI)") {
		t.Fatalf("errors %+v, want a mixed-benchmark error at entry.all[1]", errs)
	}
	same := valid(
		cond(patterns.TypeRelativeStrength, patterns.Params{"benchmark": "^NSEBANK"}),
		cond(patterns.TypeRelativeStrength, patterns.Params{"benchmark": "^NSEBANK", "rs_months": 3.0}),
	)
	if errs := fieldErrors(t, same); errs != nil {
		t.Fatalf("one benchmark twice: %+v, want valid", errs)
	}
}
