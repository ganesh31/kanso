// Package patterns holds the pattern framework (metadata, params, a shared
// evaluation context and a registry) and every V1 pattern and filter.
package patterns

// BreakoutLevel is the context value a breakout pattern provides: the level
// and the index of the bar that set it at every t (spec §5.2).
const BreakoutLevel = "breakout_level"

// Meta describes a pattern for validation, the builder UI and the report.
type Meta struct {
	Type        string      `json:"type"`
	Label       string      `json:"label"`
	Description string      `json:"description"`
	Params      []ParamSpec `json:"params"`
	// Provides and Requires name context values such as BreakoutLevel.
	Provides []string `json:"provides"`
	Requires []string `json:"requires"`
}

// Result is one condition's verdict on one bar. A not-evaluable result
// never passes and carries NotEvaluable evidence.
type Result struct {
	Pass      bool     `json:"pass"`
	Evaluable bool     `json:"evaluable"`
	Evidence  Evidence `json:"evidence"`
}

// Pattern is one condition. Prepare runs once per context (O(n)), then
// Eval answers for any bar index in O(1). Params must be normalized.
type Pattern interface {
	Meta() Meta
	Prepare(ctx *Context, p Params) error
	Eval(ctx *Context, p Params, t int) Result
}

func notEvaluable(reason Reason) Result {
	return Result{Evidence: NotEvaluable{Type: TypeNotEvaluable, Reason: reason}}
}
