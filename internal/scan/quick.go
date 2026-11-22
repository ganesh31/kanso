package scan

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// QuickRequest is a scan for one pattern or filter, no saved system needed
// (design §7.5).
type QuickRequest struct {
	Pattern   string
	Params    patterns.Params
	Universe  string
	Timeframe market.Timeframe
	AsOf      market.Date
	Mode      Mode
}

// Quick scans with the ad-hoc rule {all: [pattern]}. A pattern that needs
// a breakout level gets n_year_high (default params) inserted before it,
// recorded in AutoInserted. Validation errors point at the request's own
// fields ("pattern", "params", "timeframe").
func Quick(ctx context.Context, prov market.Provider, q QuickRequest) (*ScanResult, error) {
	sys, inserted, err := QuickSystem(q, patterns.Default())
	if err != nil {
		return nil, err
	}
	res, err := Run(ctx, prov, Request{System: sys, AsOf: q.AsOf, Mode: q.Mode})
	if err != nil {
		return nil, quickPaths(err, len(inserted))
	}
	res.AutoInserted = inserted
	return res, nil
}

// QuickSystem builds the draft system a quick scan runs (also what "Open
// in System Builder" starts from) and the condition types it inserted.
func QuickSystem(q QuickRequest, reg *patterns.Registry) (rules.System, []string, error) {
	pat, ok := reg.Get(q.Pattern)
	if !ok {
		return rules.System{}, nil, &rules.ValidationError{Errors: []rules.FieldError{
			{Path: "pattern", Message: fmt.Sprintf("unknown pattern %q", q.Pattern)},
		}}
	}
	m := pat.Meta()
	all := []rules.Condition{{Type: q.Pattern, Params: q.Params}}
	inserted := []string{}
	if slices.Contains(m.Requires, patterns.BreakoutLevel) && !slices.Contains(m.Provides, patterns.BreakoutLevel) {
		all = append([]rules.Condition{{Type: patterns.TypeNYearHigh}}, all...)
		inserted = append(inserted, patterns.TypeNYearHigh)
	}
	sys := rules.System{
		Name:      "Quick scan: " + m.Label,
		Universe:  q.Universe,
		Timeframe: q.Timeframe,
		Entry:     rules.Entry{All: all},
	}
	rules.ApplyDefaults(&sys)
	return sys, inserted, nil
}

// quickPaths rewrites system paths to the quick request's fields: the
// pattern's own condition (after any inserted ones) is "pattern" and
// "params".
func quickPaths(err error, offset int) error {
	ve, ok := err.(*rules.ValidationError)
	if !ok {
		return err
	}
	own := fmt.Sprintf("entry.all[%d].", offset)
	out := &rules.ValidationError{}
	for _, fe := range ve.Errors {
		if rest, ok := strings.CutPrefix(fe.Path, own); ok {
			switch {
			case rest == "type":
				fe.Path = "pattern"
			case strings.HasPrefix(rest, "params"):
				fe.Path = "params"
			}
		}
		out.Errors = append(out.Errors, fe)
	}
	return out
}
