package report

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

func breakoutSystem(tf market.Timeframe) *rules.System {
	return &rules.System{
		Name: "Breakout", Universe: "NIFTY50", Timeframe: tf,
		Entry: rules.Entry{
			All: []rules.Condition{{Type: "n_year_high"}},
			Any: []rules.Condition{
				{Type: "volume_surge", Params: patterns.Params{"k": 5.0}},
				{Type: "relative_strength"},
			},
		},
	}
}

func TestReportWithSystemIncludesOutcome(t *testing.T) {
	r, err := Run(context.Background(), provider(), "BREAK", breakoutSystem(""), "", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, l := range r.Lines {
		types = append(types, l.Type)
	}
	if diff := cmp.Diff([]string{"n_year_high", "volume_surge", "relative_strength"}, types); diff != "" {
		t.Fatalf("line types follow the system (-want +got):\n%s", diff)
	}
	if r.Lines[0].Params.Float("years") != 3 || r.Lines[0].Label == "" {
		t.Fatalf("n_year_high line = %+v", r.Lines[0])
	}
	if r.Lines[1].Result.Pass || !r.Lines[2].Result.Pass {
		t.Fatalf("volume k=5 should fail and RS pass: %+v", r.Lines)
	}
	o := r.SystemOutcome
	if o == nil || !o.Pass || !o.Evaluable || !cmp.Equal(o.AnyMatched, []int{1}) || len(o.Conditions) != 3 {
		t.Fatalf("outcome = %+v", o)
	}
	if r.Level == nil || *r.Level != 130 || r.AsOf != date(799) {
		t.Fatalf("header = %v %s", r.Level, r.AsOf)
	}
}

func TestReportWithoutSystemHasNoOutcome(t *testing.T) {
	r, err := Run(context.Background(), provider(), "BREAK", nil, "", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	if r.SystemOutcome != nil {
		t.Fatalf("outcome = %+v", r.SystemOutcome)
	}
}

func TestReportSystemTimeframeWins(t *testing.T) {
	r, err := Run(context.Background(), provider(), "BREAK", breakoutSystem(market.Weekly), "", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	if r.Timeframe != market.Weekly || r.AsOf != date(795) {
		t.Fatalf("header = %s %s", r.Timeframe, r.AsOf)
	}
	if o := r.SystemOutcome; o == nil || o.Pass || o.Evaluable {
		t.Fatalf("outcome = %+v, want not evaluable (114 weeks < 3 years)", o)
	}
}

func TestReportSystemBeforeFirstBar(t *testing.T) {
	r, err := Run(context.Background(), provider(), "NEW", breakoutSystem(""), "2000-01-01", market.Daily)
	if err != nil {
		t.Fatal(err)
	}
	o := r.SystemOutcome
	if o == nil || o.Pass || o.Evaluable || len(o.Conditions) != 3 || len(r.Lines) != 3 {
		t.Fatalf("outcome = %+v lines = %+v", o, r.Lines)
	}
	if ne, ok := r.Lines[0].Result.Evidence.(patterns.NotEvaluable); !ok || ne.Reason != patterns.ReasonInsufficientHistory {
		t.Fatalf("line = %+v", r.Lines[0])
	}
}

func TestReportInvalidSystem(t *testing.T) {
	sys := breakoutSystem("")
	sys.Entry.Any = sys.Entry.Any[:1]
	_, err := Run(context.Background(), provider(), "BREAK", sys, "", market.Daily)
	var ve *rules.ValidationError
	if !errors.As(err, &ve) || ve.Errors[0].Path != "entry.any" {
		t.Fatalf("err = %v", err)
	}
}
