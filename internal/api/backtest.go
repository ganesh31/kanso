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
	// Level is the breakout level at the signal bar, for the trade chart.
	Level     *float64     `json:"level,omitempty" doc:"Breakout level at the signal bar; absent when the rule has none"`
	LevelDate *market.Date `json:"level_date,omitempty" format:"date" doc:"Date the breakout level was set"`
}

// PortfolioTrade is a trade the portfolio held.
type PortfolioTrade struct {
	Trade
	Quantity int64 `json:"quantity" doc:"Whole shares bought"`
}

// PortfolioStats are the portfolio metrics next to buy-and-hold Nifty's.
type PortfolioStats struct {
	CAGR             float64 `json:"cagr" doc:"Compound annual growth rate, a fraction (0.12 is 12%)"`
	MaxDrawdown      float64 `json:"max_drawdown" doc:"Deepest fall from a peak, a fraction ≤ 0"`
	FinalEquity      float64 `json:"final_equity" doc:"Rupees at the last close"`
	Exposure         float64 `json:"exposure" doc:"Fraction of trading days with a position held, 0–1"`
	BenchCAGR        float64 `json:"bench_cagr" doc:"Buy-and-hold Nifty 50's CAGR, a fraction"`
	BenchMaxDrawdown float64 `json:"bench_max_drawdown" doc:"Buy-and-hold Nifty 50's max drawdown, a fraction ≤ 0"`
}

// EquityPoint is a curve's value at one close.
type EquityPoint struct {
	Date  market.Date `json:"date" format:"date"`
	Value float64     `json:"value" doc:"Rupees"`
}

// SkippedSignal is a portfolio candidate with a free slot that could not
// buy one whole share.
type SkippedSignal struct {
	Symbol    string      `json:"symbol"`
	EntryDate market.Date `json:"entry_date" format:"date"`
	Reason    string      `json:"reason" enum:"price_above_slot,insufficient_cash"`
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
	RankingRule  string              `json:"ranking_rule" doc:"How crowded entry days are ranked"`
	Skipped      []SkippedInstrument `json:"skipped"`
	// SkippedSignals are listed by entry date.
	SkippedSignals []SkippedSignal `json:"skipped_signals" doc:"Portfolio signals skipped with a slot free, by entry date"`
}

// BacktestResult is a backtest's output.
type BacktestResult struct {
	System     rules.System `json:"system" doc:"The system run, with defaults filled in"`
	Trades     []Trade      `json:"trades" doc:"Single-stock trades (no slot limits), by symbol then entry date"`
	TradeStats TradeStats   `json:"trade_stats"`
	// PortfolioTrades are by entry date, then rank.
	PortfolioTrades []PortfolioTrade `json:"portfolio_trades" doc:"Trades the portfolio held, by entry date then rank"`
	Portfolio       PortfolioStats   `json:"portfolio"`
	Equity          []EquityPoint    `json:"equity" doc:"Portfolio value at each close from the data start"`
	Benchmark       []EquityPoint    `json:"benchmark" doc:"Buy-and-hold Nifty 50 from the same capital, on the same dates"`
	Honesty         Honesty          `json:"honesty"`
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
	out := BacktestResult{
		System: r.System, Trades: make([]Trade, len(r.AllTrades)), PortfolioTrades: make([]PortfolioTrade, len(r.Taken)),
		Equity: toPoints(r.Equity), Benchmark: toPoints(r.Benchmark),
	}
	for i, t := range r.AllTrades {
		out.Trades[i] = toTrade(t)
	}
	for i, t := range r.Taken {
		out.PortfolioTrades[i] = PortfolioTrade{Trade: toTrade(t), Quantity: t.Quantity}
	}
	p := r.Portfolio
	out.Portfolio = PortfolioStats{
		CAGR: p.CAGR, MaxDrawdown: p.MaxDrawdown, FinalEquity: p.FinalEquity, Exposure: p.Exposure,
		BenchCAGR: p.BenchCAGR, BenchMaxDrawdown: p.BenchMaxDrawdown,
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
		RankingRule: h.RankingRule, Skipped: make([]SkippedInstrument, len(h.Skipped)),
		SkippedSignals: make([]SkippedSignal, len(h.SkippedSignals)),
	}
	for i, sk := range h.Skipped {
		out.Honesty.Skipped[i] = SkippedInstrument{Symbol: sk.Symbol, Reason: sk.Reason}
	}
	for i, sk := range h.SkippedSignals {
		out.Honesty.SkippedSignals[i] = SkippedSignal{Symbol: sk.Symbol, EntryDate: sk.EntryDate, Reason: sk.Reason}
	}
	return out
}

// toTrade leaves out RankKey, which can be -Inf (no JSON for it).
func toTrade(t backtest.Trade) Trade {
	return Trade{
		Symbol: t.Symbol, SignalDate: t.SignalDate, EntryDate: t.EntryDate, ExitDate: t.ExitDate,
		EntryPrice: t.EntryPrice, ExitPrice: t.ExitPrice, ExitReason: string(t.ExitReason), GappedStop: t.GappedStop,
		ReturnPct: t.ReturnPct, HoldingDays: t.HoldingDays, Level: t.Level, LevelDate: t.LevelDate,
	}
}

func toPoints(ps []backtest.EquityPoint) []EquityPoint {
	out := make([]EquityPoint, len(ps))
	for i, p := range ps {
		out[i] = EquityPoint{Date: p.Date, Value: p.Value}
	}
	return out
}
