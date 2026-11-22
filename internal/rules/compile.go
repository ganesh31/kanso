package rules

import (
	"fmt"

	"kanso/internal/patterns"
)

// Groups a condition can sit in.
const (
	GroupAll = "all"
	GroupAny = "any"
)

// CondOutcome is one condition's verdict within an evaluation.
type CondOutcome struct {
	Group     string          `json:"group" enum:"all,any"`
	Index     int             `json:"index"`
	Condition Condition       `json:"condition"`
	Result    patterns.Result `json:"result"`
}

// Outcome is the entry rule's verdict on one bar, with every condition's
// result (the explanation) and the indexes of the ANY branches that
// passed.
type Outcome struct {
	Pass       bool          `json:"pass"`
	Evaluable  bool          `json:"evaluable"`
	Conditions []CondOutcome `json:"conditions"`
	AnyMatched []int         `json:"any_matched"`
}

type compiledCond struct {
	group  string
	index  int
	cond   Condition
	pat    patterns.Pattern
	params patterns.Params
}

// Compiled is an entry rule bound to its patterns, ready to evaluate. It
// is read-only after Compile, so one Compiled may serve many contexts in
// parallel.
type Compiled struct {
	conds []compiledCond
	// prepareOrder lists conds indexes with context providers first.
	prepareOrder []int
	hasAny       bool
}

// Compile binds an entry rule to the registry, normalizing params. Callers
// should Validate the system first for field-level errors.
func Compile(e Entry, reg *patterns.Registry) (*Compiled, error) {
	c := &Compiled{hasAny: len(e.Any) > 0}
	add := func(group string, conds []Condition) error {
		for i, cond := range conds {
			pat, ok := reg.Get(cond.Type)
			if !ok {
				return fmt.Errorf("entry.%s[%d]: unknown condition type %q", group, i, cond.Type)
			}
			params, err := patterns.Normalize(pat.Meta(), cond.Params)
			if err != nil {
				return fmt.Errorf("entry.%s[%d]: %w", group, i, err)
			}
			cond.Params = params
			c.conds = append(c.conds, compiledCond{group: group, index: i, cond: cond, pat: pat, params: params})
		}
		return nil
	}
	if err := add(GroupAll, e.All); err != nil {
		return nil, err
	}
	if err := add(GroupAny, e.Any); err != nil {
		return nil, err
	}
	for pass := range 2 {
		for i, cc := range c.conds {
			if isProvider := len(cc.pat.Meta().Provides) > 0; isProvider == (pass == 0) {
				c.prepareOrder = append(c.prepareOrder, i)
			}
		}
	}
	return c, nil
}

// Prepare prepares every condition on ctx, context providers (the breakout
// level) before the conditions that read them.
func (c *Compiled) Prepare(ctx *patterns.Context) error {
	for _, i := range c.prepareOrder {
		cc := c.conds[i]
		if err := cc.pat.Prepare(ctx, cc.params); err != nil {
			return fmt.Errorf("prepare %s: %w", cc.cond.Type, err)
		}
	}
	return nil
}

// NotEvaluable is the outcome when there is no bar to evaluate: every
// condition is not evaluable for reason.
func (c *Compiled) NotEvaluable(reason patterns.Reason) Outcome {
	out := Outcome{Conditions: make([]CondOutcome, len(c.conds)), AnyMatched: []int{}}
	for i, cc := range c.conds {
		out.Conditions[i] = CondOutcome{Group: cc.group, Index: cc.index, Condition: cc.cond, Result: patterns.NotEvaluableResult(reason)}
	}
	return out
}

// Eval evaluates the rule on bar t of a prepared context. Every condition
// is evaluated, so the explanation is complete. The rule is not evaluable
// when a required condition is not evaluable, or when no ANY branch passes
// and some branch is not evaluable (its verdict is then unknown).
func (c *Compiled) Eval(ctx *patterns.Context, t int) Outcome {
	out := Outcome{Evaluable: true, Conditions: make([]CondOutcome, len(c.conds)), AnyMatched: []int{}}
	allPass := true
	anyPass, anyUnknown := false, false
	for i, cc := range c.conds {
		r := cc.pat.Eval(ctx, cc.params, t)
		out.Conditions[i] = CondOutcome{Group: cc.group, Index: cc.index, Condition: cc.cond, Result: r}
		switch cc.group {
		case GroupAll:
			if !r.Evaluable {
				out.Evaluable = false
			}
			allPass = allPass && r.Pass
		case GroupAny:
			if r.Pass {
				anyPass = true
				out.AnyMatched = append(out.AnyMatched, cc.index)
			} else if !r.Evaluable {
				anyUnknown = true
			}
		}
	}
	if c.hasAny && !anyPass && anyUnknown {
		out.Evaluable = false
	}
	out.Pass = out.Evaluable && allPass && (!c.hasAny || anyPass)
	return out
}
