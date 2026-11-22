package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/report"
	"kanso/internal/rules"
)

type handlers struct {
	prov    market.Provider
	systems SystemStore
	// maxBars caps one /api/bars response.
	maxBars int
}

func register(api huma.API, h *handlers) {
	huma.Register(api, huma.Operation{
		OperationID: "list-patterns",
		Method:      http.MethodGet,
		Path:        "/api/patterns",
		Summary:     "List every registered pattern and its params",
	}, h.patterns)
	huma.Register(api, huma.Operation{
		OperationID: "stock-report",
		Method:      http.MethodPost,
		Path:        "/api/report/{symbol}",
		Summary:     "Evaluate every pattern on one stock",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound},
	}, h.report)
	huma.Register(api, huma.Operation{
		OperationID: "get-bars",
		Method:      http.MethodGet,
		Path:        "/api/bars/{symbol}",
		Summary:     "One symbol's bars for one chart, at most the bar limit",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound},
	}, h.bars)
	registerScan(api, h)
	registerSystems(api, h)
	registerBacktest(api, h)
}

// PatternsOutput lists the pattern registry.
type PatternsOutput struct {
	Body struct {
		Patterns []patterns.Meta `json:"patterns"`
	}
}

func (h *handlers) patterns(_ context.Context, _ *struct{}) (*PatternsOutput, error) {
	out := &PatternsOutput{}
	out.Body.Patterns = patterns.Default().All()
	return out, nil
}

// ReportRequest is the optional body of a stock report.
type ReportRequest struct {
	// System, when present, is reported instead of every pattern: its
	// conditions, its timeframe and its entry rule's outcome. It need not be
	// saved.
	System *rules.System `json:"system,omitempty" doc:"Report this system's conditions and outcome instead of every pattern; it need not be saved"`
	// AsOf evaluates the last bar on or before this date; empty means the
	// latest bar.
	AsOf market.Date `json:"as_of,omitempty" format:"date" doc:"Evaluate the last bar on or before this date (default: latest)"`
	// Timeframe of the bars every condition evaluates on.
	Timeframe market.Timeframe `json:"timeframe,omitempty" enum:"daily,weekly" default:"daily" doc:"Bars to evaluate on; weekly bars are complete ISO weeks built from daily bars. Ignored with a system, which has its own"`
}

// ReportInput is the stock report request.
type ReportInput struct {
	Symbol string `path:"symbol" doc:"NSE symbol, e.g. TCS"`
	Body   *ReportRequest
}

// Result is one condition's verdict with its evidence.
type Result struct {
	Pass      bool     `json:"pass"`
	Evaluable bool     `json:"evaluable"`
	Evidence  Evidence `json:"evidence"`
}

// ReportLine is one condition in a stock report.
type ReportLine struct {
	Type   string          `json:"type"`
	Label  string          `json:"label"`
	Params patterns.Params `json:"params"`
	Result Result          `json:"result"`
}

// Report is the stock report.
type Report struct {
	Symbol    string           `json:"symbol"`
	AsOf      market.Date      `json:"as_of" doc:"Date of the bar evaluated, from the data; empty if there is none"`
	Timeframe market.Timeframe `json:"timeframe" enum:"daily,weekly"`
	Level     *float64         `json:"level,omitempty" doc:"Breakout level at as_of, if any"`
	LevelDate *market.Date     `json:"level_date,omitempty" doc:"Date the breakout level was set"`
	Lines     []ReportLine     `json:"lines"`
	// SystemOutcome is the system's entry rule on as_of, when the request
	// had a system.
	SystemOutcome *Outcome `json:"system_outcome,omitempty" doc:"The system's entry rule on as_of (only with a system)"`
}

// ReportOutput is the stock report response.
type ReportOutput struct {
	Body Report
}

func (h *handlers) report(ctx context.Context, in *ReportInput) (*ReportOutput, error) {
	req := ReportRequest{Timeframe: market.Daily}
	if in.Body != nil {
		req = *in.Body
		if req.Timeframe == "" {
			req.Timeframe = market.Daily
		}
	}
	rep, err := report.Run(ctx, h.prov, in.Symbol, req.System, req.AsOf, req.Timeframe)
	if err != nil {
		if p := validationProblem(err, "body.system."); p != nil {
			return nil, p
		}
		return nil, mapError(err, in.Symbol)
	}
	out := &ReportOutput{Body: Report{
		Symbol: rep.Symbol, AsOf: rep.AsOf, Timeframe: rep.Timeframe,
		Level: rep.Level, LevelDate: rep.LevelDate, Lines: make([]ReportLine, len(rep.Lines)),
	}}
	if rep.SystemOutcome != nil {
		o := toOutcome(*rep.SystemOutcome)
		out.Body.SystemOutcome = &o
	}
	for i, l := range rep.Lines {
		out.Body.Lines[i] = ReportLine{
			Type: l.Type, Label: l.Label, Params: l.Params,
			Result: Result{Pass: l.Result.Pass, Evaluable: l.Result.Evaluable, Evidence: Evidence{l.Result.Evidence}},
		}
	}
	return out, nil
}

// BarsInput selects one symbol's bars in a date range.
type BarsInput struct {
	Symbol    string           `path:"symbol"`
	From      market.Date      `query:"from" format:"date" doc:"First date (inclusive); default: as far back as the bar limit allows"`
	To        market.Date      `query:"to" format:"date" doc:"Last date (inclusive); default: the last bar"`
	Timeframe market.Timeframe `query:"timeframe" enum:"daily,weekly" default:"daily" doc:"weekly: ISO-week bars built from the daily bars in the range, dated by their last day"`
}

// BarsOutput is one symbol's bars, ascending by date.
type BarsOutput struct {
	Body struct {
		Symbol string       `json:"symbol"`
		Bars   []market.Bar `json:"bars"`
	}
}

// bars serves one symbol's bars for one chart (spec §6.4): a range with
// more bars than the limit is a 400, and without from it serves the last
// limit bars up to to, never the whole history.
func (h *handlers) bars(ctx context.Context, in *BarsInput) (*BarsOutput, error) {
	inst, err := h.prov.Instrument(ctx, in.Symbol)
	if err != nil {
		return nil, mapError(err, in.Symbol)
	}
	to := in.To
	if to == "" {
		to = "9999-12-31"
	}
	if in.From != "" && in.From > to {
		return nil, huma.Error400BadRequest("from is after to", &huma.ErrorDetail{
			Message: "must not be after to", Location: "query.from", Value: in.From,
		})
	}
	daily, err := h.prov.Daily(ctx, inst.ID, in.From, to)
	if err != nil {
		return nil, mapError(err, in.Symbol)
	}
	bars := daily
	if in.Timeframe == market.Weekly {
		bars = market.ToWeekly(market.NewDaily(inst, daily)).Bars
	}
	if len(bars) > h.maxBars {
		if in.From != "" {
			return nil, huma.Error400BadRequest(fmt.Sprintf("range has %d bars, over the %d-bar limit", len(bars), h.maxBars),
				&huma.ErrorDetail{Message: fmt.Sprintf("at most %d bars per request", h.maxBars), Location: "query.from", Value: in.From})
		}
		bars = bars[len(bars)-h.maxBars:]
	}
	out := &BarsOutput{}
	out.Body.Symbol, out.Body.Bars = inst.Symbol, bars
	return out, nil
}

func mapError(err error, symbol string) error {
	if errors.Is(err, market.ErrNotFound) {
		return huma.Error404NotFound("unknown symbol " + symbol)
	}
	return huma.Error500InternalServerError("internal error", err)
}
