package backtest

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// fake is a pattern whose verdict on bar t is pass[t], evaluable unless
// evaluable[t] is false.
type fake struct {
	pass      []bool
	evaluable []bool
}

func (fake) Meta() patterns.Meta                              { return patterns.Meta{Type: "fake"} }
func (fake) Prepare(*patterns.Context, patterns.Params) error { return nil }
func (f fake) Eval(_ *patterns.Context, _ patterns.Params, t int) patterns.Result {
	if f.evaluable != nil && !f.evaluable[t] {
		return patterns.NotEvaluableResult(patterns.ReasonDataGap)
	}
	return patterns.Result{Pass: f.pass[t], Evaluable: true}
}

// stubRule compiles {all:[fake]} and a context of len(pass) daily bars.
func stubRule(t *testing.T, pass, evaluable []bool) (*rules.Compiled, *patterns.Context) {
	t.Helper()
	c, err := rules.Compile(rules.Entry{All: []rules.Condition{{Type: "fake"}}},
		patterns.NewRegistry(fake{pass: pass, evaluable: evaluable}))
	if err != nil {
		t.Fatal(err)
	}
	bars := make([]market.Bar, len(pass))
	ctx := patterns.NewContext(market.NewDaily(market.Instrument{Symbol: "T"}, bars), nil)
	if err := c.Prepare(ctx); err != nil {
		t.Fatal(err)
	}
	return c, ctx
}

func TestSignalsEdgeTriggered(t *testing.T) {
	c, ctx := stubRule(t, []bool{false, true, true, false, true}, nil)
	if diff := cmp.Diff([]int{1, 4}, Signals(c, ctx)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestNoSignalAfterGap(t *testing.T) {
	c, ctx := stubRule(t, []bool{false, false, false, false, true}, []bool{true, true, false, false, true})
	if got := Signals(c, ctx); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestNoSignalOnFirstEvaluableBar(t *testing.T) {
	c, ctx := stubRule(t, []bool{false, false, true, true}, []bool{false, false, true, true})
	if got := Signals(c, ctx); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestBarZeroNeverSignals(t *testing.T) {
	c, ctx := stubRule(t, []bool{true, true}, nil)
	if got := Signals(c, ctx); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestSignalsOnlyOnCompleteBars(t *testing.T) {
	c, ctx := stubRule(t, []bool{false, false, true}, nil)
	ctx.Series.Complete = 2 // bar 2 is a week in progress
	if got := Signals(c, ctx); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}
