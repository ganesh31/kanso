import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vite-plus/test";
import { configureApi, type BacktestResult, type System } from "@/api/client";
import { BacktestPanel } from "./BacktestPanel";

vi.mock("./EquityChart", () => ({
  EquityChart: ({ equity, benchmark }: { equity: unknown[]; benchmark: unknown[] }) => (
    <div data-testid="equity-chart">
      {equity.length} vs {benchmark.length}
    </div>
  ),
}));

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const system: System = {
  name: "3-year breakout",
  universe: "NIFTY50",
  timeframe: "daily",
  entry: {
    all: [
      { type: "n_year_high", params: { years: 3, all_time: false } },
      { type: "volume_surge", params: { k: 2, window_weeks: 10 } },
    ],
  },
  exits: { stop_pct: 8, trend_sma_weeks: 30 },
  costs: { per_side_pct: 0.2, slippage_pct: 0.1 },
  portfolio: { capital: 1000000, max_positions: 10 },
};

const result: BacktestResult = {
  system,
  trades: [
    {
      symbol: "TCS",
      signal_date: "2024-02-06",
      entry_date: "2024-02-07",
      exit_date: "2024-03-01",
      entry_price: 4105.2,
      exit_price: 3776.78,
      exit_reason: "stop",
      gapped_stop: true,
      return_pct: -8.39,
      holding_days: 17,
    },
    {
      symbol: "INFY",
      signal_date: "2024-09-02",
      entry_date: "2024-09-03",
      exit_date: "2024-11-20",
      entry_price: 1900,
      exit_price: 2100,
      exit_reason: "open",
      gapped_stop: false,
      return_pct: 10.3,
      holding_days: 56,
    },
  ],
  trade_stats: {
    trades: 2,
    win_rate: 0.5,
    avg_win_pct: 10.3,
    avg_loss_pct: -8.39,
    expectancy_pct: 0.956,
    median_holding_days: 36,
  },
  honesty: {
    fill_model: "next-day open",
    per_side_pct: 0.2,
    slippage_pct: 0.1,
    survivorship: true,
    data_source: "Yahoo Finance",
    adjustment: "split-adjusted, not dividend-adjusted",
    data_start: "2007-09-17",
    data_end: "2026-11-20",
    ranking_rule: "crowded days ranked by 6-month RS, then symbol",
    skipped: [],
    skipped_signals: [
      { symbol: "MRF", entry_date: "2024-03-04", reason: "price_above_slot" },
      { symbol: "HDFCBANK", entry_date: "2014-05-12", reason: "insufficient_cash" },
    ],
  },
  portfolio_trades: [],
  portfolio: {
    cagr: 0.142,
    max_drawdown: -0.321,
    final_equity: 2534000.5,
    exposure: 0.62,
    bench_cagr: 0.113,
    bench_max_drawdown: -0.38,
  },
  equity: [
    { date: "2007-09-17", value: 1000000 },
    { date: "2026-11-20", value: 2534000.5 },
  ],
  benchmark: [
    { date: "2007-09-17", value: 1000000 },
    { date: "2026-11-20", value: 2100000 },
  ],
};

const backtest = vi.fn<(body: unknown) => Response>();

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BacktestPanel />
    </QueryClientProvider>,
  );
}

describe("BacktestPanel", () => {
  beforeEach(() => {
    backtest.mockReset();
    backtest.mockReturnValue(json(200, result));
    configureApi({
      baseUrl: "http://test",
      fetch: async (input) => {
        const req = input as Request;
        const path = new URL(req.url).pathname;
        if (path === "/api/default-system") return json(200, { system });
        if (path === "/api/backtest") return backtest(await req.json());
        return json(404, { title: "Not Found" });
      },
    });
  });

  afterEach(() => configureApi({}));

  test("opens on the default system and backtests it", async () => {
    renderPanel();
    expect(await screen.findByText("3-year breakout")).toBeInTheDocument();
    expect(screen.getByText("Buys NIFTY50 stocks when every condition below holds on a daily bar")).toBeInTheDocument();
    expect(screen.getByText("Close above the highest high of the prior 3 years")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Backtest" }));
    await waitFor(() => expect(backtest).toHaveBeenCalledWith(system));

    const tiles = await screen.findByRole("region", { name: "Trade-level stats" });
    expect(within(tiles).getByText("Trades").nextSibling).toHaveTextContent("2");
    expect(within(tiles).getByText("Win rate").nextSibling).toHaveTextContent("50.0%");
    expect(within(tiles).getByText("Avg win").nextSibling).toHaveTextContent("+10.30%");
    expect(within(tiles).getByText("Avg loss").nextSibling).toHaveTextContent("−8.39%");
    expect(within(tiles).getByText("Expectancy").nextSibling).toHaveTextContent("+0.96%");
    expect(within(tiles).getByText("Median hold").nextSibling).toHaveTextContent("36 days");
    expect(screen.getByText(/next-day open/)).toBeInTheDocument();
  });

  test("CAGR and max drawdown tiles each show the Nifty comparison", async () => {
    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Backtest" }));
    const tiles = await screen.findByRole("region", { name: "Portfolio stats" });
    const cagr = within(tiles).getByRole("group", { name: "CAGR" });
    expect(cagr).toHaveTextContent("+14.20%");
    expect(cagr).toHaveTextContent("Nifty +11.30%");
    const mdd = within(tiles).getByRole("group", { name: "Max drawdown" });
    expect(mdd).toHaveTextContent("−32.10%");
    expect(mdd).toHaveTextContent("Nifty −38.00%");
    expect(within(tiles).getByRole("group", { name: "Final equity" })).toHaveTextContent("₹25,34,000.50");
    expect(within(tiles).getByRole("group", { name: "Exposure" })).toHaveTextContent("62.0%");
    expect(screen.getByTestId("equity-chart")).toHaveTextContent("2 vs 2");
  });

  test("lists the ranking rule and skipped signals", async () => {
    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Backtest" }));
    expect(await screen.findByText(/crowded days ranked by 6-month RS, then symbol/)).toBeInTheDocument();
    expect(screen.getByText(/1 signal skipped: price above slot size \(MRF 2024-03-04\)/)).toBeInTheDocument();
    expect(screen.getByText(/1 signal skipped: insufficient cash \(HDFCBANK 2014-05-12\)/)).toBeInTheDocument();
  });

  test("lists every trade with its exit reason", async () => {
    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Backtest" }));
    const table = await screen.findByRole("table", { name: "Trades" });
    const rows = within(table).getAllByRole("row");
    expect(rows).toHaveLength(3);
    expect(rows[1]).toHaveTextContent("TCS");
    expect(rows[1]).toHaveTextContent("stop (gapped)");
    expect(rows[1]).toHaveTextContent("4,105.20");
    expect(rows[2]).toHaveTextContent("INFY");
    expect(rows[2]).toHaveTextContent("open");
  });

  test("shows a backtest error", async () => {
    backtest.mockReturnValue(
      json(400, {
        title: "Bad Request",
        detail: "invalid request",
        errors: [{ location: "body.entry.any", message: "too few" }],
      }),
    );
    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Backtest" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("entry.any: too few");
  });

  test("long trade lists show 25 rows until asked for all", async () => {
    const first = result.trades[0]!;
    const trades = Array.from({ length: 30 }, (_, i) => ({
      ...first,
      symbol: `S${String(i).padStart(2, "0")}`,
    }));
    backtest.mockReturnValue(json(200, { ...result, trades }));
    renderPanel();
    fireEvent.click(await screen.findByRole("button", { name: "Backtest" }));
    const table = await screen.findByRole("table", { name: "Trades" });
    expect(within(table).getAllByRole("row")).toHaveLength(1 + 25);
    fireEvent.click(screen.getByRole("button", { name: "Show all 30 trades" }));
    expect(within(table).getAllByRole("row")).toHaveLength(1 + 30);
  });
});
