package patterns

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
)

func TestToBarsMatchesSpecDefaults(t *testing.T) {
	cases := []struct {
		n      float64
		u      Unit
		daily  int
		weekly int
	}{
		{10, Weeks, 50, 10},
		{6, Months, 126, 26},
		{1, Years, 252, 52},
		{3, Years, 756, 156},
		{2, Years, 504, 104}, // all-time minimum (spec §5.2)
	}
	for _, c := range cases {
		if got := ToBars(c.n, c.u, market.Daily); got != c.daily {
			t.Errorf("ToBars(%v %s, daily) = %d, want %d", c.n, c.u, got, c.daily)
		}
		if got := ToBars(c.n, c.u, market.Weekly); got != c.weekly {
			t.Errorf("ToBars(%v %s, weekly) = %d, want %d", c.n, c.u, got, c.weekly)
		}
	}
}

var testMeta = Meta{
	Type: "test",
	Params: []ParamSpec{
		{Name: "years", Kind: KindNumber, Unit: Years, Default: 3.0, Min: 1, Max: 20, Step: 1},
		{Name: "age", Kind: KindNumber, Unit: Years, Default: 1.0, Min: 0.25, Max: 10, Step: 0.25},
		{Name: "all_time", Kind: KindBool, Default: false},
		{Name: "benchmark", Kind: KindSymbol, Default: "^NSEI"},
	},
}

func TestNormalizeFillsDefaults(t *testing.T) {
	got, err := Normalize(testMeta, Params{"years": 5})
	if err != nil {
		t.Fatal(err)
	}
	want := Params{"years": 5.0, "age": 1.0, "all_time": false, "benchmark": "^NSEI"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestNormalizeRejectsOutOfRange(t *testing.T) {
	_, err := Normalize(testMeta, Params{"years": 25.0})
	if err == nil || !strings.Contains(err.Error(), "years") {
		t.Fatalf("err = %v, want an error naming years", err)
	}
}

func TestNormalizeRejectsBadInput(t *testing.T) {
	cases := map[string]Params{
		"unknown name": {"nope": 1.0},
		"wrong kind":   {"years": "three"},
		"bool kind":    {"all_time": 1.0},
		"off step":     {"age": 0.3},
		"not whole":    {"years": 2.5},
		"below min":    {"age": 0.0},
		"symbol kind":  {"benchmark": 3.0},
		"empty symbol": {"benchmark": ""},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Normalize(testMeta, p); err == nil {
				t.Fatalf("Normalize(%v) = nil error", p)
			}
		})
	}
}

func TestNormalizeAcceptsStepMultiples(t *testing.T) {
	if _, err := Normalize(testMeta, Params{"age": 1.75}); err != nil {
		t.Fatal(err)
	}
}
