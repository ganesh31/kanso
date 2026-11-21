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
	TypeNotEvaluable     = "not_evaluable"
	TypeNYearHigh        = "n_year_high"
	TypeLevelAge         = "level_age"
	TypeNotExtended      = "not_extended"
	TypeRelativeStrength = "relative_strength"
	TypeVolumeSurge      = "volume_surge"
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

// VolumeEvidence backs a volume_surge verdict (spec §5.3).
type VolumeEvidence struct {
	Type string `json:"type" enum:"volume_surge"`
	// Ratio is today's volume over the prior window's mean volume.
	Ratio float64 `json:"ratio"`
	// AvgVolume is the mean volume of the prior window, today excluded.
	AvgVolume float64 `json:"avg_volume"`
}

// EvidenceType implements Evidence.
func (VolumeEvidence) EvidenceType() string { return TypeVolumeSurge }

// RSEvidence backs a relative_strength verdict (spec §5.3). Returns are
// fractions (0.1 is +10%).
type RSEvidence struct {
	Type        string  `json:"type" enum:"relative_strength"`
	StockReturn float64 `json:"stock_return"`
	BenchReturn float64 `json:"bench_return"`
	// Diff is StockReturn − BenchReturn; it passes when > 0.
	Diff float64 `json:"diff"`
}

// EvidenceType implements Evidence.
func (RSEvidence) EvidenceType() string { return TypeRelativeStrength }

// NotExtendedEvidence backs a not_extended verdict (spec §5.3).
type NotExtendedEvidence struct {
	Type string `json:"type" enum:"not_extended"`
	// PctAbove is how far the close is above the breakout level, in percent.
	PctAbove float64 `json:"pct_above"`
}

// EvidenceType implements Evidence.
func (NotExtendedEvidence) EvidenceType() string { return TypeNotExtended }

// LevelAgeEvidence backs a level_age verdict (spec §5.3).
type LevelAgeEvidence struct {
	Type string `json:"type" enum:"level_age"`
	// BarsSince is how many bars ago the breakout level was set.
	BarsSince int `json:"bars_since"`
	// LevelDate is the date of the bar that set the level.
	LevelDate market.Date `json:"level_date"`
}

// EvidenceType implements Evidence.
func (LevelAgeEvidence) EvidenceType() string { return TypeLevelAge }

// Evidences lists one zero value of every evidence variant, in report
// order. The API builds its discriminated-union schema from it.
func Evidences() []Evidence {
	return []Evidence{
		NYearHighEvidence{Type: TypeNYearHigh},
		LevelAgeEvidence{Type: TypeLevelAge},
		NotExtendedEvidence{Type: TypeNotExtended},
		RSEvidence{Type: TypeRelativeStrength},
		VolumeEvidence{Type: TypeVolumeSurge},
		NotEvaluable{Type: TypeNotEvaluable},
	}
}
