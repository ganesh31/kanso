package api

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"kanso/internal/backtest"
	"kanso/internal/market"
	"kanso/internal/rules"
)

func registerBacktest(api huma.API, h *handlers) {
	huma.Register(api, huma.Operation{
		OperationID: "get-default-system",
		Method:      http.MethodGet,
		Path:        "/api/default-system",
		Summary:     "The system the Builder opens on",
	}, h.defaultSystem)
	huma.Register(api, huma.Operation{
		OperationID: "backtest",
		Method:      http.MethodPost,
		Path:        "/api/backtest",
		Summary:     "Backtest a system over its stock list",
		Errors:      []int{http.StatusBadRequest, http.StatusNotFound},
	}, h.backtest)
}

// DefaultSystemOutput is the default system.
type DefaultSystemOutput struct {
	Body struct {
		System rules.System `json:"system"`
	}
}

func (h *handlers) defaultSystem(_ context.Context, _ *struct{}) (*DefaultSystemOutput, error) {
	out := &DefaultSystemOutput{}
	out.Body.System = rules.DefaultSystem()
	return out, nil
}

// BacktestInput is the system to backtest, saved or not.
type BacktestInput struct {
	Body rules.System
}

// Trade is one round trip in one stock.
type Trade struct {
	Symbol     string      `json:"symbol"`
	SignalDate market.Date `json:"signal_date" format:"date" doc:"Signal bar (weekly: the signal week's last trading day)"`
	EntryDate  market.Date `json:"entry_date" format:"date" doc:"Next trading day after the signal; filled at its open"`
	ExitDate   market.Date `json:"exit_date" format:"date"`
	EntryPrice float64     `json:"entry_price" doc:"Fill after slippage, before costs"`
	ExitPrice  float64     `json:"exit_price" doc:"Fill after slippage, before costs; the last close for an open trade"`
	ExitReason string      `json:"exit_reason" enum:"stop,trend,open"`
	GappedStop bool        `json:"gapped_stop" doc:"The day opened below the stop, so it filled at the open"`
	ReturnPct  float64     `json:"return_pct" doc:"Per-share return in percent, net of costs and slippage"`
	// HoldingDays counts trading days from entry to exit.
	HoldingDays int `json:"holding_days" doc:"Trading days from entry to exit"`
}

// TradeStats are the trade-level metrics over single-stock trades.
type TradeStats struct {
	Trades            int     `json:"trades"`
	WinRate           float64 `json:"win_rate" doc:"Fraction of trades with a positive return, 0–1"`
	AvgWinPct         float64 `json:"avg_win_pct"`
	AvgLossPct        float64 `json:"avg_loss_pct"`
	ExpectancyPct     float64 `json:"expectancy_pct" doc:"Mean return per trade, in percent"`
	MedianHoldingDays int     `json:"median_holding_days" doc:"In trading days"`
}

// SkippedInstrument is a stock list member the backtest could not use.
type SkippedInstrument struct {
	Symbol string `json:"symbol"`
	Reason string `json:"reason" enum:"no_data"`
}

// Honesty is what the honesty panel shows next to the results.
type Honesty struct {
	FillModel    string              `json:"fill_model"`
	PerSidePct   float64             `json:"per_side_pct"`
	SlippagePct  float64             `json:"slippage_pct"`
	Survivorship bool                `json:"survivorship" doc:"True when the stock list is today's members only"`
	DataSource   string              `json:"data_source"`
	Adjustment   string              `json:"adjustment"`
	DataStart    market.Date         `json:"data_start" doc:"First date used: the later of the requested start and the benchmark's first date"`
	DataEnd      market.Date         `json:"data_end"`
	Skipped      []SkippedInstrument `json:"skipped"`
}

// BacktestResult is a backtest's output.
type BacktestResult struct {
	System     rules.System `json:"system" doc:"The system run, with defaults filled in"`
	Trades     []Trade      `json:"trades" doc:"Single-stock trades (no slot limits), by symbol then entry date"`
	TradeStats TradeStats   `json:"trade_stats"`
	Honesty    Honesty      `json:"honesty"`
}

// BacktestOutput is a backtest response.
type BacktestOutput struct {
	Body BacktestResult
}

func (h *handlers) backtest(ctx context.Context, in *BacktestInput) (*BacktestOutput, error) {
	res, err := backtest.Run(ctx, h.prov, in.Body, "", "")
	if err != nil {
		return nil, mapScanError(err, in.Body.Universe)
	}
	return &BacktestOutput{Body: toBacktestResult(res)}, nil
}

func toBacktestResult(r *backtest.Result) BacktestResult {
	out := BacktestResult{System: r.System, Trades: make([]Trade, len(r.AllTrades))}
	for i, t := range r.AllTrades {
		out.Trades[i] = Trade{
			Symbol: t.Symbol, SignalDate: t.SignalDate, EntryDate: t.EntryDate, ExitDate: t.ExitDate,
			EntryPrice: t.EntryPrice, ExitPrice: t.ExitPrice, ExitReason: string(t.ExitReason), GappedStop: t.GappedStop,
			ReturnPct: t.ReturnPct, HoldingDays: t.HoldingDays,
		}
	}
	s := r.TradeStats
	out.TradeStats = TradeStats{
		Trades: s.Trades, WinRate: s.WinRate, AvgWinPct: s.AvgWinPct, AvgLossPct: s.AvgLossPct,
		ExpectancyPct: s.ExpectancyPct, MedianHoldingDays: s.MedianHoldingDays,
	}
	h := r.Honesty
	out.Honesty = Honesty{
		FillModel: h.FillModel, PerSidePct: h.PerSidePct, SlippagePct: h.SlippagePct, Survivorship: h.Survivorship,
		DataSource: h.DataSource, Adjustment: h.Adjustment, DataStart: h.DataStart, DataEnd: h.DataEnd,
		Skipped: make([]SkippedInstrument, len(h.Skipped)),
	}
	for i, sk := range h.Skipped {
		out.Honesty.Skipped[i] = SkippedInstrument{Symbol: sk.Symbol, Reason: sk.Reason}
	}
	return out
}
