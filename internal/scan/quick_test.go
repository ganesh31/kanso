package scan

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// breakout serves BREAK: 300 flat bars, the last closing at 105 (5% over
// its 1-year high) on triple volume.
func breakout() market.Provider {
	bars := withVolume(flat(300), 299, 299, 300)
	bars[299].Close, bars[299].High = 105, 105
	return provider(300, map[string][]market.Bar{"BREAK": bars, "FLAT": flat(300)})
}

func quick(t *testing.T, q QuickRequest) *ScanResult {
	t.Helper()
	res, err := Quick(context.Background(), breakout(), q)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestQuickAutoInsertsNYearHigh(t *testing.T) {
	res := quick(t, QuickRequest{Pattern: patterns.TypeNotExtended, Universe: "U", Mode: All})
	if diff := cmp.Diff([]string{patterns.TypeNYearHigh}, res.AutoInserted); diff != "" {
		t.Fatalf("auto-inserted (-want +got):\n%s", diff)
	}
	// The inserted n_year_high has its default 3 years, more history than
	// 300 bars, so nothing has a level to be "not extended" from.
	if len(res.Matches) != 0 {
		t.Fatalf("matches %v", symbols(res.Matches))
	}
}

func TestQuickAutoInsertedConditionComesFirst(t *testing.T) {
	sys, inserted, err := QuickSystem(QuickRequest{Pattern: patterns.TypeLevelAge, Universe: "U"}, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, c := range sys.Entry.All {
		types = append(types, c.Type)
	}
	if diff := cmp.Diff([]string{patterns.TypeNYearHigh, patterns.TypeLevelAge}, types); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{patterns.TypeNYearHigh}, inserted); diff != "" {
		t.Fatal(diff)
	}
	if sys.Timeframe != market.Daily {
		t.Fatalf("timeframe %q, want daily by default", sys.Timeframe)
	}
}

func TestQuickVolumeOnlyNoInsert(t *testing.T) {
	res := quick(t, QuickRequest{Pattern: patterns.TypeVolumeSurge, Universe: "U", Mode: Fresh})
	if len(res.AutoInserted) != 0 {
		t.Fatalf("auto-inserted %v", res.AutoInserted)
	}
	if diff := cmp.Diff([]string{"BREAK"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
}

func TestQuickUsesEditedParams(t *testing.T) {
	// A 1-year high finds BREAK; the default 3 years needs more history.
	res := quick(t, QuickRequest{Pattern: patterns.TypeNYearHigh, Params: patterns.Params{"years": 1.0}, Universe: "U", Mode: Fresh})
	if diff := cmp.Diff([]string{"BREAK"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
	if res := quick(t, QuickRequest{Pattern: patterns.TypeNYearHigh, Universe: "U"}); len(res.Matches) != 0 {
		t.Fatalf("default params matched %v", symbols(res.Matches))
	}
}

func TestQuickRejectsUnknownPattern(t *testing.T) {
	_, err := Quick(context.Background(), breakout(), QuickRequest{Pattern: "cup_and_handle", Universe: "U"})
	var ve *rules.ValidationError
	if !asValidation(err, &ve) {
		t.Fatalf("got %v, want *rules.ValidationError", err)
	}
	if len(ve.Errors) != 1 || ve.Errors[0].Path != "pattern" {
		t.Fatalf("got %+v, want one error at pattern", ve.Errors)
	}
}

func TestQuickBadParamsPointAtParams(t *testing.T) {
	_, err := Quick(context.Background(), breakout(), QuickRequest{Pattern: patterns.TypeNotExtended, Params: patterns.Params{"pct": 99.0}, Universe: "U"})
	var ve *rules.ValidationError
	if !asValidation(err, &ve) || len(ve.Errors) != 1 || ve.Errors[0].Path != "params" {
		t.Fatalf("got %v, want one error at params", err)
	}
}
