package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
	"kanso/internal/scan"
)

func registerScan(api huma.API, h *handlers) {
	huma.Register(api, huma.Operation{
		OperationID: "list-universes",
		Method:      http.MethodGet,
		Path:        "/api/universes",
		Summary:     "List every stock list",
	}, h.universes)
	huma.Register(api, huma.Operation{
		OperationID: "quick-scan",
		Method:      http.MethodPost,
		Path:        "/api/quickscan",
		Summary:     "Scan a stock list for one pattern or filter",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound},
	}, h.quickscan)
	huma.Register(api, huma.Operation{
		OperationID: "validate-system",
		Method:      http.MethodPost,
		Path:        "/api/validate",
		Summary:     "Validate a system and return it with defaults filled in",
		Errors:      []int{http.StatusBadRequest},
	}, h.validate)
}

// Universe is one stock list.
type Universe struct {
	Name    string `json:"name"`
	Members int    `json:"members"`
	// HasHistory is false when membership is today's list only, so results
	// carry survivorship bias.
	HasHistory bool `json:"has_history" doc:"False when only today's members are known (survivorship bias)"`
}

// UniversesOutput lists the stock lists.
type UniversesOutput struct {
	Body struct {
		Universes []Universe `json:"universes"`
	}
}

func (h *handlers) universes(ctx context.Context, _ *struct{}) (*UniversesOutput, error) {
	names, err := h.prov.Universes(ctx)
	if err != nil {
		return nil, huma.Error500InternalServerError("internal error", err)
	}
	out := &UniversesOutput{}
	out.Body.Universes = make([]Universe, 0, len(names))
	for _, n := range names {
		members, hist, err := h.prov.Universe(ctx, n)
		if err != nil {
			return nil, huma.Error500InternalServerError("internal error", err)
		}
		out.Body.Universes = append(out.Body.Universes, Universe{Name: n, Members: len(members), HasHistory: hist})
	}
	return out, nil
}

// QuickscanRequest is a scan for one pattern or filter (design §7.4).
type QuickscanRequest struct {
	Pattern   string           `json:"pattern" doc:"Pattern or filter type, from /api/patterns"`
	Params    patterns.Params  `json:"params,omitempty" doc:"Param values; missing ones take their defaults"`
	Universe  string           `json:"universe"`
	Timeframe market.Timeframe `json:"timeframe,omitempty" enum:"daily,weekly" default:"daily"`
	AsOf      market.Date      `json:"as_of,omitempty" format:"date" doc:"Scan as of this date (default: the latest benchmark date)"`
	Mode      scan.Mode        `json:"mode,omitempty" enum:"fresh,all" default:"fresh" doc:"fresh: signalled within the last 5 daily / 1 weekly bars; all: qualifying now"`
}

// QuickscanInput is the quick scan request.
type QuickscanInput struct {
	Body QuickscanRequest
}

// CondOutcome is one condition's verdict within an evaluation.
type CondOutcome struct {
	Group     string          `json:"group" enum:"all,any"`
	Index     int             `json:"index" doc:"Position within its group"`
	Condition rules.Condition `json:"condition"`
	Result    Result          `json:"result"`
}

// Outcome is the entry rule's verdict on one bar, with every condition.
type Outcome struct {
	Pass       bool          `json:"pass"`
	Evaluable  bool          `json:"evaluable"`
	Conditions []CondOutcome `json:"conditions"`
	AnyMatched []int         `json:"any_matched" doc:"Indexes of the ANY branches that passed"`
}

// Match is one qualifying stock. Columns are absent when their condition
// is not in the rule or not evaluable.
type Match struct {
	Symbol       string       `json:"symbol"`
	Date         market.Date  `json:"date" doc:"Bar evaluated: the as-of day, or the last complete week"`
	SignalDate   market.Date  `json:"signal_date,omitempty" doc:"Latest signal in the fresh window (fresh mode)"`
	TrueSince    market.Date  `json:"true_since,omitempty" doc:"First bar of the current pass streak (all mode)"`
	Outcome      Outcome      `json:"outcome"`
	Level        *float64     `json:"level,omitempty"`
	LevelDate    *market.Date `json:"level_date,omitempty"`
	LevelAgeBars *int         `json:"level_age_bars,omitempty" doc:"Bars since the level was set"`
	PctAbove     *float64     `json:"pct_above,omitempty" doc:"Close above the level, in percent"`
	VolRatio     *float64     `json:"vol_ratio,omitempty" doc:"Volume over the prior window's average"`
	RSDiff       *float64     `json:"rs_diff,omitempty" doc:"Stock return minus benchmark return, as a fraction"`
	Provisional  *Outcome     `json:"provisional,omitempty" doc:"The rule on the week in progress (weekly only)"`
}

// ScanResult is a scan's matches, sorted by % above (nulls last), then
// symbol.
type ScanResult struct {
	AsOf         market.Date `json:"as_of" doc:"Date scanned, from the data"`
	Matches      []Match     `json:"matches"`
	Stale        []string    `json:"stale" doc:"Stocks whose last bar is before as_of; never matched"`
	Skipped      []string    `json:"skipped" doc:"Stocks with no bars up to as_of"`
	AutoInserted []string    `json:"auto_inserted" doc:"Condition types added because the pattern needs them"`
}

// ScanOutput is a scan response.
type ScanOutput struct {
	Body ScanResult
}

func (h *handlers) quickscan(ctx context.Context, in *QuickscanInput) (*ScanOutput, error) {
	q := in.Body
	res, err := scan.Quick(ctx, h.prov, scan.QuickRequest{
		Pattern: q.Pattern, Params: q.Params, Universe: q.Universe, Timeframe: q.Timeframe, AsOf: q.AsOf, Mode: q.Mode,
	})
	if err != nil {
		return nil, mapScanError(err, q.Universe)
	}
	return &ScanOutput{Body: toScanResult(res)}, nil
}

// ValidateInput is a system to validate.
type ValidateInput struct {
	Body rules.System
}

// ValidateOutput is the valid system with defaults and params filled in.
type ValidateOutput struct {
	Body struct {
		System rules.System `json:"system"`
	}
}

func (h *handlers) validate(_ context.Context, in *ValidateInput) (*ValidateOutput, error) {
	sys := in.Body
	rules.ApplyDefaults(&sys)
	if err := rules.Validate(&sys, patterns.Default()); err != nil {
		return nil, mapScanError(err, sys.Universe)
	}
	out := &ValidateOutput{}
	out.Body.System = sys
	return out, nil
}

// mapScanError maps a validation error to 400 with one body field per
// problem, and an unknown stock list to 404.
func mapScanError(err error, universe string) error {
	if p := validationProblem(err, "body."); p != nil {
		return p
	}
	if errors.Is(err, market.ErrNotFound) {
		return huma.Error404NotFound("unknown stock list " + universe)
	}
	return huma.Error500InternalServerError("internal error", err)
}

func toScanResult(r *scan.ScanResult) ScanResult {
	out := ScanResult{AsOf: r.AsOf, Matches: make([]Match, len(r.Matches)), Stale: r.Stale, Skipped: r.Skipped, AutoInserted: r.AutoInserted}
	for i, m := range r.Matches {
		out.Matches[i] = Match{
			Symbol: m.Symbol, Date: m.Date, SignalDate: m.SignalDate, TrueSince: m.TrueSince,
			Outcome: toOutcome(m.Outcome), Level: m.Level, LevelDate: m.LevelDate, LevelAgeBars: m.LevelAgeBars,
			PctAbove: m.PctAbove, VolRatio: m.VolRatio, RSDiff: m.RSDiff,
		}
		if m.Provisional != nil {
			p := toOutcome(*m.Provisional)
			out.Matches[i].Provisional = &p
		}
	}
	return out
}

func toOutcome(o rules.Outcome) Outcome {
	out := Outcome{Pass: o.Pass, Evaluable: o.Evaluable, Conditions: make([]CondOutcome, len(o.Conditions)), AnyMatched: o.AnyMatched}
	for i, c := range o.Conditions {
		out.Conditions[i] = CondOutcome{
			Group: c.Group, Index: c.Index,
			Condition: c.Condition,
			Result:    Result{Pass: c.Result.Pass, Evaluable: c.Result.Evaluable, Evidence: Evidence{c.Result.Evidence}},
		}
	}
	return out
}
