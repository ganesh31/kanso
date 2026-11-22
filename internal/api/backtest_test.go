package api

import (
	"encoding/json"
	"math"
	"net/http/httptest"
	"testing"

	"kanso/internal/market"
)

// backtestServer serves BREAK, which makes a new 3-year high on bar 795
// and holds it to the end of data (bar 799), plus EMPTY with no bars.
func backtestServer(t *testing.T) *httptest.Server {
	t.Helper()
	bars := make([]market.Bar, 800)
	for i := range bars {
		bars[i] = market.Bar{Date: date(i), Open: 99, High: 100, Low: 98, Close: 99, Volume: 1}
	}
	bars[300].High = 130
	for i := 795; i < 800; i++ {
		bars[i] = market.Bar{Date: date(i), Open: 131, High: 132, Low: 130, Close: 131, Volume: 1}
	}
	prov := market.NewMemProvider(
		[]market.Instrument{{ID: 1, Symbol: "BREAK", Type: market.Equity}, {ID: 2, Symbol: "EMPTY", Type: market.Equity}},
		map[string][]market.Bar{"BREAK": bars}, map[string][]string{"NIFTY50": {"BREAK", "EMPTY"}})
	srv := httptest.NewServer(New(prov, NewMemSystems(), Config{}))
	t.Cleanup(srv.Close)
	return srv
}

func TestBacktestReturnsTradesStatsAndHonesty(t *testing.T) {
	status, body := do(t, backtestServer(t), "POST", "/api/backtest",
		`{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}]}}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		System struct {
			Exits struct {
				StopPct float64 `json:"stop_pct"`
			} `json:"exits"`
		} `json:"system"`
		Trades []struct {
			Symbol     string  `json:"symbol"`
			SignalDate string  `json:"signal_date"`
			EntryDate  string  `json:"entry_date"`
			ExitDate   string  `json:"exit_date"`
			EntryPrice float64 `json:"entry_price"`
			ExitPrice  float64 `json:"exit_price"`
			ExitReason string  `json:"exit_reason"`
		} `json:"trades"`
		TradeStats struct {
			Trades int `json:"trades"`
		} `json:"trade_stats"`
		Honesty struct {
			FillModel    string `json:"fill_model"`
			Survivorship bool   `json:"survivorship"`
			Skipped      []struct {
				Symbol string `json:"symbol"`
				Reason string `json:"reason"`
			} `json:"skipped"`
		} `json:"honesty"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.System.Exits.StopPct != 8 {
		t.Errorf("system defaults not filled: %s", body)
	}
	if len(got.Trades) != 1 || got.TradeStats.Trades != 1 {
		t.Fatalf("want one trade, got %s", body)
	}
	tr := got.Trades[0]
	if tr.Symbol != "BREAK" || tr.SignalDate != string(date(795)) || tr.EntryDate != string(date(796)) ||
		tr.ExitReason != "open" || tr.ExitPrice != 131 || math.Abs(tr.EntryPrice-131.131) > 1e-9 {
		t.Errorf("trade %+v", tr)
	}
	h := got.Honesty
	if h.FillModel != "next-day open" || !h.Survivorship || len(h.Skipped) != 1 || h.Skipped[0].Symbol != "EMPTY" || h.Skipped[0].Reason != "no_data" {
		t.Errorf("honesty %+v", h)
	}
}

// portfolioServer is backtestServer's BREAK plus a flat ^NSEI at 100.
func portfolioServer(t *testing.T) *httptest.Server {
	t.Helper()
	bars := make([]market.Bar, 800)
	bench := make([]market.Bar, 800)
	for i := range bars {
		bars[i] = market.Bar{Date: date(i), Open: 99, High: 100, Low: 98, Close: 99, Volume: 1}
		bench[i] = market.Bar{Date: date(i), Open: 100, High: 100, Low: 100, Close: 100, Volume: 1}
	}
	bars[300].High = 130
	for i := 795; i < 800; i++ {
		bars[i] = market.Bar{Date: date(i), Open: 131, High: 132, Low: 130, Close: 131, Volume: 1}
	}
	prov := market.NewMemProvider(
		[]market.Instrument{{ID: 1, Symbol: "BREAK", Type: market.Equity}, {ID: 2, Symbol: market.Benchmark, Type: market.Index}},
		map[string][]market.Bar{"BREAK": bars, market.Benchmark: bench}, map[string][]string{"NIFTY50": {"BREAK"}})
	srv := httptest.NewServer(New(prov, NewMemSystems(), Config{}))
	t.Cleanup(srv.Close)
	return srv
}

type portfolioBody struct {
	PortfolioTrades []struct {
		Symbol   string `json:"symbol"`
		Quantity int64  `json:"quantity"`
	} `json:"portfolio_trades"`
	Portfolio struct {
		CAGR             float64 `json:"cagr"`
		MaxDrawdown      float64 `json:"max_drawdown"`
		FinalEquity      float64 `json:"final_equity"`
		Exposure         float64 `json:"exposure"`
		BenchCAGR        float64 `json:"bench_cagr"`
		BenchMaxDrawdown float64 `json:"bench_max_drawdown"`
	} `json:"portfolio"`
	Equity []struct {
		Date  string  `json:"date"`
		Value float64 `json:"value"`
	} `json:"equity"`
	Benchmark []struct {
		Date  string  `json:"date"`
		Value float64 `json:"value"`
	} `json:"benchmark"`
	Honesty struct {
		RankingRule    string `json:"ranking_rule"`
		SkippedSignals []struct {
			Symbol    string `json:"symbol"`
			EntryDate string `json:"entry_date"`
			Reason    string `json:"reason"`
		} `json:"skipped_signals"`
	} `json:"honesty"`
}

func TestBacktestReturnsPortfolioAndBenchmark(t *testing.T) {
	status, body := do(t, portfolioServer(t), "POST", "/api/backtest",
		`{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}]}}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got portfolioBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	// 100,000 slot ÷ (131 × 1.001 × 1.002) = 761.8 shares.
	if len(got.PortfolioTrades) != 1 || got.PortfolioTrades[0].Symbol != "BREAK" || got.PortfolioTrades[0].Quantity != 761 {
		t.Fatalf("portfolio trades %+v", got.PortfolioTrades)
	}
	if len(got.Equity) != 800 || len(got.Benchmark) != 800 || got.Equity[0].Date != string(date(0)) || got.Benchmark[799].Value != 1_000_000 {
		t.Fatalf("curves: %d equity, %d benchmark points", len(got.Equity), len(got.Benchmark))
	}
	p := got.Portfolio
	if p.FinalEquity != got.Equity[799].Value || p.BenchCAGR != 0 || p.BenchMaxDrawdown != 0 || p.MaxDrawdown >= 0 || p.Exposure <= 0 {
		t.Fatalf("portfolio %+v", p)
	}
	if got.Honesty.RankingRule != "crowded days ranked by 6-month RS, then symbol" || len(got.Honesty.SkippedSignals) != 0 {
		t.Fatalf("honesty %+v", got.Honesty)
	}
}

func TestBacktestListsSkippedSignals(t *testing.T) {
	status, body := do(t, portfolioServer(t), "POST", "/api/backtest",
		`{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}]},"portfolio":{"capital":1000,"max_positions":10}}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got portfolioBody
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	ss := got.Honesty.SkippedSignals
	if len(ss) != 1 || ss[0].Symbol != "BREAK" || ss[0].EntryDate != string(date(796)) || ss[0].Reason != "price_above_slot" {
		t.Fatalf("skipped signals %+v, want BREAK on %s (one share is above the 100 slot)", ss, date(796))
	}
	if len(got.PortfolioTrades) != 0 || got.Portfolio.FinalEquity != 1000 {
		t.Fatalf("portfolio %+v, trades %+v", got.Portfolio, got.PortfolioTrades)
	}
}

func TestBacktestTradeCarriesBreakoutLevelForTheChart(t *testing.T) {
	status, body := do(t, backtestServer(t), "POST", "/api/backtest",
		`{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}]}}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		Trades []struct {
			Level     *float64 `json:"level"`
			LevelDate string   `json:"level_date"`
		} `json:"trades"`
		PortfolioTrades []struct {
			Level *float64 `json:"level"`
		} `json:"portfolio_trades"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Trades) != 1 || got.Trades[0].Level == nil || *got.Trades[0].Level != 130 || got.Trades[0].LevelDate != string(date(300)) {
		t.Fatalf("trades: %s", body)
	}
	if len(got.PortfolioTrades) != 1 || got.PortfolioTrades[0].Level == nil || *got.PortfolioTrades[0].Level != 130 {
		t.Fatalf("portfolio trades: %s", body)
	}
}

func TestBacktestUnknownUniverse404(t *testing.T) {
	status, body := do(t, backtestServer(t), "POST", "/api/backtest",
		`{"name":"x","universe":"NOPE","entry":{"all":[{"type":"n_year_high"}]}}`)
	if status != 404 {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestDefaultSystemEndpoint(t *testing.T) {
	status, body := do(t, backtestServer(t), "GET", "/api/default-system", "")
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		System struct {
			Universe string `json:"universe"`
			Entry    struct {
				All []struct {
					Type string `json:"type"`
				} `json:"all"`
			} `json:"entry"`
		} `json:"system"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.System.Universe != "NIFTY50" || len(got.System.Entry.All) != 4 || got.System.Entry.All[0].Type != "n_year_high" {
		t.Fatalf("got %s", body)
	}
}
