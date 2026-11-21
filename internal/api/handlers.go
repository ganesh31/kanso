package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/report"
)

type handlers struct {
	prov market.Provider
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
		Summary:     "Daily bars for one chart",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound},
	}, h.bars)
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
	// AsOf evaluates the last bar on or before this date; empty means the
	// latest bar.
	AsOf market.Date `json:"as_of,omitempty" format:"date" doc:"Evaluate the last bar on or before this date (default: latest)"`
	// Timeframe of the bars every condition evaluates on.
	Timeframe market.Timeframe `json:"timeframe,omitempty" enum:"daily,weekly" default:"daily" doc:"Bars to evaluate on; weekly bars are complete ISO weeks built from daily bars"`
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
	rep, err := report.Run(ctx, h.prov, in.Symbol, req.AsOf, req.Timeframe)
	if err != nil {
		return nil, mapError(err, in.Symbol)
	}
	out := &ReportOutput{Body: Report{
		Symbol: rep.Symbol, AsOf: rep.AsOf, Timeframe: rep.Timeframe,
		Level: rep.Level, LevelDate: rep.LevelDate, Lines: make([]ReportLine, len(rep.Lines)),
	}}
	for i, l := range rep.Lines {
		out.Body.Lines[i] = ReportLine{
			Type: l.Type, Label: l.Label, Params: l.Params,
			Result: Result{Pass: l.Result.Pass, Evaluable: l.Result.Evaluable, Evidence: Evidence{l.Result.Evidence}},
		}
	}
	return out, nil
}

// BarsInput selects one symbol's daily bars.
type BarsInput struct {
	Symbol string      `path:"symbol"`
	From   market.Date `query:"from" format:"date" doc:"First date (inclusive); default: the first bar"`
	To     market.Date `query:"to" format:"date" doc:"Last date (inclusive); default: the last bar"`
}

// BarsOutput is one symbol's daily bars, ascending by date.
type BarsOutput struct {
	Body struct {
		Symbol string       `json:"symbol"`
		Bars   []market.Bar `json:"bars"`
	}
}

func (h *handlers) bars(ctx context.Context, in *BarsInput) (*BarsOutput, error) {
	inst, err := h.prov.Instrument(ctx, in.Symbol)
	if err != nil {
		return nil, mapError(err, in.Symbol)
	}
	to := in.To
	if to == "" {
		to = "9999-12-31"
	}
	bars, err := h.prov.Daily(ctx, inst.ID, in.From, to)
	if err != nil {
		return nil, mapError(err, in.Symbol)
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
