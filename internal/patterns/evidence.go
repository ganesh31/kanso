package patterns

import "kanso/internal/market"

// Evidence is the typed set of numbers behind one condition's verdict on
// one bar. Each condition type has its own struct; the API exposes them as
// a union discriminated by the "type" field (ADR 0003).
type Evidence interface {
	EvidenceType() string
}

// Evidence type discriminators.
const (
	TypeNotEvaluable = "not_evaluable"
	TypeNYearHigh    = "n_year_high"
)

// Reason says why a condition could not be evaluated on a bar.
type Reason string

// Not-evaluable reasons.
const (
	ReasonInsufficientHistory Reason = "insufficient_history"
	ReasonDataGap             Reason = "data_gap"
	ReasonBenchmarkMissing    Reason = "benchmark_missing"
	ReasonNoBreakoutLevel     Reason = "no_breakout_level"
	ReasonZeroDenominator     Reason = "zero_denominator"
	ReasonIncompleteBar       Reason = "incomplete_bar"
)

// NotEvaluable is the evidence of a result that could not be evaluated.
type NotEvaluable struct {
	Type   string `json:"type" enum:"not_evaluable"`
	Reason Reason `json:"reason" enum:"insufficient_history,data_gap,benchmark_missing,no_breakout_level,zero_denominator,incomplete_bar"`
}

// EvidenceType implements Evidence.
func (NotEvaluable) EvidenceType() string { return TypeNotEvaluable }

// NYearHighEvidence backs an n_year_high verdict (spec §5.2).
type NYearHighEvidence struct {
	Type string `json:"type" enum:"n_year_high"`
	// Level is the highest high of the prior lookback bars.
	Level float64 `json:"level"`
	// LevelDate is the date of the bar that set the level.
	LevelDate market.Date `json:"level_date"`
	// BarsSince is how many bars ago the level was set.
	BarsSince int     `json:"bars_since"`
	Close     float64 `json:"close"`
	// PctAbove is how far the close is above the level, in percent.
	PctAbove float64 `json:"pct_above"`
}

// EvidenceType implements Evidence.
func (NYearHighEvidence) EvidenceType() string { return TypeNYearHigh }

// Evidences lists one zero value of every evidence variant, in a fixed
// order. The API builds its discriminated-union schema from it.
func Evidences() []Evidence {
	return []Evidence{
		NYearHighEvidence{Type: TypeNYearHigh},
		NotEvaluable{Type: TypeNotEvaluable},
	}
}
