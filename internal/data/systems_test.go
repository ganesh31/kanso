package data

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

func sampleSystem(name string) rules.System {
	return rules.System{
		Name: name, Universe: "NIFTY500", Timeframe: market.Weekly,
		Entry: rules.Entry{
			All: []rules.Condition{{Type: "n_year_high", Params: patterns.Params{"years": 3.0}}},
			Any: []rules.Condition{
				{Type: "volume_surge", Params: patterns.Params{"k": 1.5, "window_weeks": 10.0}},
				{Type: "relative_strength", Params: patterns.Params{"rs_months": 6.0, "benchmark": "^NSEI"}},
			},
		},
		Exits:     rules.Exits{StopPct: 8, TrendSMAWeeks: 0},
		Costs:     rules.Costs{PerSidePct: 0.2, SlippagePct: 0},
		Portfolio: rules.Portfolio{Capital: 500000, MaxPositions: 5},
	}
}

func TestSystemsCRUD(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)

	want := sampleSystem("Breakout")
	id, err := s.SaveSystem(ctx, want)
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 36 {
		t.Fatalf("id %q is not a UUID", id)
	}
	want.ID = id
	got, err := s.GetSystem(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("round trip (-want +got):\n%s", diff)
	}

	// Saving with the id updates in place.
	want.Name = "Breakout v2"
	if again, err := s.SaveSystem(ctx, want); err != nil || again != id {
		t.Fatalf("update: id %q err %v", again, err)
	}
	other, err := s.SaveSystem(ctx, sampleSystem("Another"))
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.ListSystems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, sys := range list {
		names = append(names, sys.Name)
	}
	if diff := cmp.Diff([]string{"Another", "Breakout v2"}, names); diff != "" {
		t.Fatalf("list by name (-want +got):\n%s", diff)
	}

	if err := s.DeleteSystem(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetSystem(ctx, other); !errors.Is(err, market.ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	if err := s.DeleteSystem(ctx, other); !errors.Is(err, market.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
}

func TestSystemsEmptyListAndReadOnly(t *testing.T) {
	ctx := context.Background()
	s, path := openTemp(t)
	list, err := s.ListSystems(ctx)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("list %v err %v", list, err)
	}
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.SaveSystem(ctx, sampleSystem("x")); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("read-only save: %v", err)
	}
}
