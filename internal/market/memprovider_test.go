package market

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func testProvider() *MemProvider {
	insts := []Instrument{
		{ID: 1, Exchange: "NSE", Symbol: "TCS", Type: Equity, LotSize: 1},
		{ID: 2, Exchange: "NSE", Symbol: "^NSEI", Type: Index, LotSize: 1},
	}
	bars := map[string][]Bar{
		"TCS": {
			{Date: "2024-01-01", Close: 1},
			{Date: "2024-01-02", Close: 2},
			{Date: "2024-01-03", Close: 3},
		},
	}
	universes := map[string][]string{"NIFTY50": {"TCS"}, "ALL": {"TCS", "^NSEI"}}
	return NewMemProvider(insts, bars, universes)
}

func TestMemProviderUnknownSymbol(t *testing.T) {
	_, err := testProvider().Instrument(context.Background(), "NOPE")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMemProviderInstrument(t *testing.T) {
	got, err := testProvider().Instrument(context.Background(), "TCS")
	if err != nil {
		t.Fatal(err)
	}
	want := Instrument{ID: 1, Exchange: "NSE", Symbol: "TCS", Type: Equity, LotSize: 1}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestMemProviderDailyRangeInclusive(t *testing.T) {
	got, err := testProvider().Daily(context.Background(), 1, "2024-01-02", "2024-01-03")
	if err != nil {
		t.Fatal(err)
	}
	want := []Bar{{Date: "2024-01-02", Close: 2}, {Date: "2024-01-03", Close: 3}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	if _, err := testProvider().Daily(context.Background(), 99, "", "9999-12-31"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id err = %v, want ErrNotFound", err)
	}
}

func TestMemProviderUniverses(t *testing.T) {
	p := testProvider()
	names, err := p.Universes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"ALL", "NIFTY50"}, names); diff != "" {
		t.Fatalf("names (-want +got):\n%s", diff)
	}
	members, hasHistory, err := p.Universe(context.Background(), "ALL")
	if err != nil {
		t.Fatal(err)
	}
	if hasHistory || len(members) != 2 || members[0].Symbol != "TCS" || members[1].Symbol != "^NSEI" {
		t.Fatalf("members = %+v hasHistory = %v", members, hasHistory)
	}
	if _, _, err := p.Universe(context.Background(), "NOPE"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMemProviderFromBarsAssignsSortedIDsAndTypes(t *testing.T) {
	p := MemProviderFromBars(map[string][]Bar{"TCS": nil, "^NSEI": nil, "INFY": nil})
	want := map[string]Instrument{
		"INFY":  {ID: 1, Exchange: "NSE", Symbol: "INFY", Type: Equity, LotSize: 1},
		"TCS":   {ID: 2, Exchange: "NSE", Symbol: "TCS", Type: Equity, LotSize: 1},
		"^NSEI": {ID: 3, Exchange: "NSE", Symbol: "^NSEI", Type: Index, LotSize: 1},
	}
	for sym, w := range want {
		got, err := p.Instrument(context.Background(), sym)
		if err != nil || got != w {
			t.Fatalf("%s: got %+v, %v; want %+v", sym, got, err, w)
		}
	}
	members, _, err := p.Universe(context.Background(), "ALL")
	if err != nil || len(members) != 2 {
		t.Fatalf("ALL universe should hold the 2 equities, got %+v %v", members, err)
	}
}

func TestNewDailyIsComplete(t *testing.T) {
	s := NewDaily(Instrument{Symbol: "TCS"}, []Bar{{Date: "2024-01-01"}, {Date: "2024-01-02"}})
	if s.Timeframe != Daily || s.Complete != 2 || s.LastDailyIdx != nil {
		t.Fatalf("got %+v", s)
	}
}
