package rules

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
)

// stub is a fake pattern whose verdict on bar t is pass[t] / evaluable[t]
// (evaluable defaults to true). It records the order it was prepared in.
type stub struct {
	typ       string
	pass      []bool
	evaluable []bool
	provides  []string
	requires  []string
	prepared  *[]string
}

func (s stub) Meta() patterns.Meta {
	return patterns.Meta{Type: s.typ, Label: s.typ, Provides: s.provides, Requires: s.requires}
}

func (s stub) Prepare(*patterns.Context, patterns.Params) error {
	if s.prepared != nil {
		*s.prepared = append(*s.prepared, s.typ)
	}
	return nil
}

func (s stub) Eval(_ *patterns.Context, _ patterns.Params, t int) patterns.Result {
	if s.evaluable != nil && !s.evaluable[t] {
		return patterns.NotEvaluableResult(patterns.ReasonInsufficientHistory)
	}
	return patterns.Result{Pass: s.pass[t], Evaluable: true}
}

func flatContext(n int) *patterns.Context {
	bars := make([]market.Bar, n)
	for i := range bars {
		bars[i] = market.Bar{Date: market.Date("2024-01-01"), Open: 1, High: 1, Low: 1, Close: 1}
	}
	return patterns.NewContext(market.NewDaily(market.Instrument{Symbol: "T"}, bars), nil)
}

func compile(t *testing.T, e Entry, ps ...patterns.Pattern) *Compiled {
	t.Helper()
	c, err := Compile(e, patterns.NewRegistry(ps...))
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Prepare(flatContext(1)); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEvalAllAndAny(t *testing.T) {
	e := Entry{All: []Condition{{Type: "a"}}, Any: []Condition{{Type: "b"}, {Type: "c"}}}
	c := compile(t, e, stub{typ: "a", pass: []bool{true}}, stub{typ: "b", pass: []bool{false}}, stub{typ: "c", pass: []bool{true}})
	out := c.Eval(flatContext(1), 0)
	if !out.Pass || !out.Evaluable {
		t.Fatalf("got %+v, want pass", out)
	}
	if diff := cmp.Diff([]int{1}, out.AnyMatched); diff != "" {
		t.Fatalf("AnyMatched (-want +got):\n%s", diff)
	}
	var where []string
	for _, co := range out.Conditions {
		where = append(where, co.Group+":"+co.Condition.Type)
	}
	if diff := cmp.Diff([]string{"all:a", "any:b", "any:c"}, where); diff != "" {
		t.Fatalf("conditions (-want +got):\n%s", diff)
	}
	if out.Conditions[2].Index != 1 {
		t.Fatalf("any[1] has index %d", out.Conditions[2].Index)
	}
}

func TestEvalAnyAllFail(t *testing.T) {
	e := Entry{All: []Condition{{Type: "a"}}, Any: []Condition{{Type: "b"}, {Type: "c"}}}
	c := compile(t, e, stub{typ: "a", pass: []bool{true}}, stub{typ: "b", pass: []bool{false}}, stub{typ: "c", pass: []bool{false}})
	out := c.Eval(flatContext(1), 0)
	if out.Pass || !out.Evaluable || len(out.AnyMatched) != 0 {
		t.Fatalf("got %+v, want evaluable fail with no ANY match", out)
	}
	if len(out.Conditions) != 3 {
		t.Fatalf("got %d condition outcomes, want all 3 for the explanation", len(out.Conditions))
	}
}

func TestEvalRequiredFailIsNotPass(t *testing.T) {
	e := Entry{All: []Condition{{Type: "a"}, {Type: "b"}}}
	c := compile(t, e, stub{typ: "a", pass: []bool{false}}, stub{typ: "b", pass: []bool{true}})
	out := c.Eval(flatContext(1), 0)
	if out.Pass || !out.Evaluable || len(out.Conditions) != 2 {
		t.Fatalf("got %+v", out)
	}
}

func TestEvalNotEvaluableRequiredIsNotPass(t *testing.T) {
	e := Entry{All: []Condition{{Type: "a"}, {Type: "b"}}}
	c := compile(t, e, stub{typ: "a", pass: []bool{true}, evaluable: []bool{false}}, stub{typ: "b", pass: []bool{true}})
	out := c.Eval(flatContext(1), 0)
	if out.Pass || out.Evaluable {
		t.Fatalf("got %+v, want not evaluable", out)
	}
	if len(out.Conditions) != 2 || !out.Conditions[1].Result.Pass {
		t.Fatalf("every condition must still be evaluated: %+v", out.Conditions)
	}
}

func TestEvalAnyUndecidableIsNotEvaluable(t *testing.T) {
	// No ANY branch passes and one could not be evaluated, so the rule's
	// verdict is unknown.
	e := Entry{All: []Condition{{Type: "a"}}, Any: []Condition{{Type: "b"}, {Type: "c"}}}
	c := compile(t, e, stub{typ: "a", pass: []bool{true}},
		stub{typ: "b", pass: []bool{false}, evaluable: []bool{false}}, stub{typ: "c", pass: []bool{false}})
	if out := c.Eval(flatContext(1), 0); out.Pass || out.Evaluable {
		t.Fatalf("got %+v, want not evaluable", out)
	}
	// One passing branch decides it, whatever the other says.
	c = compile(t, e, stub{typ: "a", pass: []bool{true}},
		stub{typ: "b", pass: []bool{false}, evaluable: []bool{false}}, stub{typ: "c", pass: []bool{true}})
	if out := c.Eval(flatContext(1), 0); !out.Pass || !out.Evaluable {
		t.Fatalf("got %+v, want pass", out)
	}
}

func TestPrepareRunsProvidersFirst(t *testing.T) {
	var order []string
	e := Entry{All: []Condition{{Type: "needs"}, {Type: "gives"}}}
	reg := patterns.NewRegistry(
		stub{typ: "needs", pass: []bool{true}, requires: []string{"x"}, prepared: &order},
		stub{typ: "gives", pass: []bool{true}, provides: []string{"x"}, prepared: &order},
	)
	c, err := Compile(e, reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Prepare(flatContext(1)); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"gives", "needs"}, order); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestCompileRejectsUnknownType(t *testing.T) {
	if _, err := Compile(Entry{All: []Condition{{Type: "nope"}}}, patterns.Default()); err == nil {
		t.Fatal("want error")
	}
}

func TestEvalRealPatterns(t *testing.T) {
	// 300 flat bars at 100, then a close of 120: a new 1-year high that is
	// 20% above the level, so not_extended (8%) fails.
	bars := make([]market.Bar, 301)
	for i := range bars {
		bars[i] = market.Bar{Date: market.Date("2024-01-01"), Open: 100, High: 100, Low: 100, Close: 100, Volume: 1}
	}
	bars[300].Close, bars[300].High = 120, 120
	ctx := patterns.NewContext(market.NewDaily(market.Instrument{Symbol: "T"}, bars), nil)
	e := Entry{All: []Condition{{Type: "n_year_high", Params: patterns.Params{"years": 1.0}}, {Type: "not_extended"}}}
	c, err := Compile(e, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	out := c.Eval(ctx, 300)
	if out.Pass || !out.Evaluable || !out.Conditions[0].Result.Pass || out.Conditions[1].Result.Pass {
		t.Fatalf("got %+v", out)
	}
}
