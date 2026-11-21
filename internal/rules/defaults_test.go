package rules

import (
	"testing"

	"kanso/internal/market"
	"kanso/internal/patterns"
)

func TestDefaultSystemIsValidWithSpecDefaults(t *testing.T) {
	s := DefaultSystem()
	if err := Validate(&s, patterns.Default()); err != nil {
		t.Fatal(err)
	}
	if s.Timeframe != market.Daily || s.Exits.StopPct != 8 || s.Exits.TrendSMAWeeks != 30 ||
		s.Costs.PerSidePct != 0.20 || s.Costs.SlippagePct != 0.10 ||
		s.Portfolio.Capital != 1_000_000 || s.Portfolio.MaxPositions != 10 {
		t.Fatalf("defaults differ from spec §5: %+v", s)
	}
	want := []string{patterns.TypeNYearHigh, patterns.TypeVolumeSurge, patterns.TypeRelativeStrength, patterns.TypeNotExtended}
	if len(s.Entry.All) != len(want) || len(s.Entry.Any) != 0 {
		t.Fatalf("entry = %+v, want all of %v", s.Entry, want)
	}
	for i, c := range s.Entry.All {
		if c.Type != want[i] {
			t.Fatalf("entry.all[%d] = %s, want %s", i, c.Type, want[i])
		}
	}
	if y := s.Entry.All[0].Params.Float("years"); y != 3 {
		t.Fatalf("years = %v, want 3", y)
	}
}

func TestDefaultSystemIsAFreshCopy(t *testing.T) {
	a := DefaultSystem()
	a.Entry.All[0].Params["years"] = 5.0
	if b := DefaultSystem(); b.Entry.All[0].Params.Float("years") != 3 {
		t.Fatal("editing one default system changed the next")
	}
}
