import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { configureApi, type Report } from "@/api/client";
import { StockReport } from "./StockReport";

/** A fake server behind the generated client: the report endpoint answers with `report()`. */
const report = vi.fn<(symbol: string, body: unknown) => { status: number; body: unknown }>();

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

// Lightweight Charts needs a real canvas; the chart is checked by hand.
vi.mock("./PriceChart", () => ({
  PriceChart: ({ level, bars }: { level?: number; bars: unknown[] }) => (
    <div data-testid="chart" data-level={level ?? ""} data-bars={bars.length} />
  ),
}));

const breakout: Report = {
  symbol: "DIVISLAB",
  as_of: "2024-09-10",
  timeframe: "daily",
  level: 4105.2,
  level_date: "2021-10-25",
  lines: [
    {
      type: "n_year_high",
      label: "New N-year high",
      params: { years: 3, all_time: false },
      result: {
        pass: true,
        evaluable: true,
        evidence: {
          type: "n_year_high",
          level: 4105.2,
          level_date: "2021-10-25",
          bars_since: 717,
          close: 4300,
          pct_above: 4.745,
        },
      },
    },
  ],
};

const tooNew: Report = {
  symbol: "NEWCO",
  as_of: "2024-09-10",
  timeframe: "daily",
  lines: [
    {
      type: "n_year_high",
      label: "New N-year high",
      params: { years: 3, all_time: false },
      result: {
        pass: false,
        evaluable: false,
        evidence: { type: "not_evaluable", reason: "insufficient_history" },
      },
    },
  ],
};

function renderReport(symbol: string, asOf?: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <StockReport symbol={symbol} asOf={asOf} />
    </QueryClientProvider>,
  );
}

describe("StockReport", () => {
  beforeEach(() => {
    report.mockReset();
    configureApi({
      baseUrl: "http://test",
      fetch: async (input) => {
        const req = input as Request;
        const path = new URL(req.url).pathname;
        if (path.startsWith("/api/bars/")) {
          return json(200, { symbol: "X", bars: [{ date: "2024-09-10", open: 1, high: 1, low: 1, close: 1, volume: 1 }] });
        }
        const symbol = decodeURIComponent(path.replace("/api/report/", ""));
        const res = report(symbol, await req.json());
        return json(res.status, res.body);
      },
    });
  });

  afterEach(() => configureApi({}));

  test("StockReport renders ✅/❌ lines and 'insufficient history'", async () => {
    report.mockReturnValueOnce({ status: 200, body: breakout });
    const { unmount } = renderReport("DIVISLAB");
    expect(await screen.findByText("✅")).toBeInTheDocument();
    expect(screen.getByText(/Old high 4,105.20 set 2021-10-25, 2 years 10 months ago \(717 bars\)/)).toBeInTheDocument();
    unmount();

    report.mockReturnValueOnce({ status: 200, body: tooNew });
    renderReport("NEWCO");
    expect(await screen.findByText("❌")).toBeInTheDocument();
    expect(screen.getByText("not evaluable (insufficient history)")).toBeInTheDocument();
  });

  test("StockReport shows level line value in header", async () => {
    report.mockReturnValueOnce({ status: 200, body: breakout });
    renderReport("DIVISLAB", "2024-09-10");
    expect(await screen.findByRole("heading", { name: "DIVISLAB" })).toBeInTheDocument();
    expect(screen.getByText("as of 2024-09-10")).toBeInTheDocument();
    expect(screen.getByText(/Breakout level 4,105.20 \(set 2021-10-25\)/)).toBeInTheDocument();
    expect(report).toHaveBeenCalledWith("DIVISLAB", { as_of: "2024-09-10" });
    const chart = await screen.findByTestId("chart");
    expect(chart).toHaveAttribute("data-level", "4105.2");
    expect(chart).toHaveAttribute("data-bars", "1");
  });

  test("unknown symbol shows the API error", async () => {
    report.mockReturnValueOnce({ status: 404, body: { title: "Not Found", status: 404, detail: "unknown symbol NOPE" } });
    renderReport("NOPE");
    expect(await screen.findByRole("alert")).toHaveTextContent("unknown symbol NOPE");
  });
});
