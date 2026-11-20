package patterns

import (
	"fmt"
	"math"
	"sort"
)

// ParamKind is the value type of a pattern param.
type ParamKind string

// Param kinds.
const (
	KindNumber ParamKind = "number"
	KindBool   ParamKind = "bool"
	KindSymbol ParamKind = "symbol"
)

// ParamSpec describes one pattern param for validation and for the UI.
// Min, Max and Step apply to numbers only; Step 0 means any value.
type ParamSpec struct {
	Name    string    `json:"name"`
	Label   string    `json:"label"`
	Kind    ParamKind `json:"kind" enum:"number,bool,symbol"`
	Unit    Unit      `json:"unit" enum:",weeks,months,years"`
	Default any       `json:"default"`
	Min     float64   `json:"min"`
	Max     float64   `json:"max"`
	Step    float64   `json:"step"`
}

// Params are a condition's param values by name. After Normalize, numbers
// are float64, bools are bool and symbols are string.
type Params map[string]any

// Float returns a number param, or 0 if it is missing or not a number.
func (p Params) Float(name string) float64 {
	v, _ := p[name].(float64)
	return v
}

// Bool returns a bool param, or false if it is missing.
func (p Params) Bool(name string) bool {
	v, _ := p[name].(bool)
	return v
}

// String returns a symbol param, or "" if it is missing.
func (p Params) String(name string) string {
	v, _ := p[name].(string)
	return v
}

// stepTolerance absorbs float error when checking a value is a whole
// number of steps from Min (e.g. 1.75 with step 0.25).
const stepTolerance = 1e-9

// Normalize returns a copy of p with defaults filled in. It rejects unknown
// names, wrong kinds and out-of-range or off-step numbers; errors name the
// offending param.
func Normalize(m Meta, p Params) (Params, error) {
	specs := make(map[string]ParamSpec, len(m.Params))
	for _, s := range m.Params {
		specs[s.Name] = s
	}
	names := make([]string, 0, len(p))
	for n := range p {
		names = append(names, n)
	}
	sort.Strings(names) // deterministic first error
	for _, n := range names {
		if _, ok := specs[n]; !ok {
			return nil, fmt.Errorf("%s: unknown param %q", m.Type, n)
		}
	}

	out := make(Params, len(m.Params))
	for _, s := range m.Params {
		v, ok := p[s.Name]
		if !ok {
			out[s.Name] = s.Default
			continue
		}
		nv, err := checkParam(s, v)
		if err != nil {
			return nil, fmt.Errorf("%s: param %q %w", m.Type, s.Name, err)
		}
		out[s.Name] = nv
	}
	return out, nil
}

func checkParam(s ParamSpec, v any) (any, error) {
	switch s.Kind {
	case KindNumber:
		f, ok := toFloat(v)
		if !ok {
			return nil, fmt.Errorf("must be a number")
		}
		if math.IsNaN(f) || f < s.Min || f > s.Max {
			return nil, fmt.Errorf("must be between %g and %g", s.Min, s.Max)
		}
		if s.Step > 0 {
			steps := (f - s.Min) / s.Step
			if math.Abs(steps-math.Round(steps)) > stepTolerance {
				return nil, fmt.Errorf("must be in steps of %g", s.Step)
			}
		}
		return f, nil
	case KindBool:
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("must be true or false")
		}
		return b, nil
	case KindSymbol:
		str, ok := v.(string)
		if !ok || str == "" {
			return nil, fmt.Errorf("must be a symbol")
		}
		return str, nil
	}
	return nil, fmt.Errorf("has unknown kind %q", s.Kind)
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	}
	return 0, false
}
