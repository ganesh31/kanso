import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vite-plus/test";
import { configureApi, type Report, type Timeframe } from "@/api/client";
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

function renderReport(
  symbol: string,
  asOf?: string,
  timeframe?: Timeframe,
  onTimeframeChange: (tf: Timeframe) => void = () => {},
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <StockReport symbol={symbol} asOf={asOf} timeframe={timeframe} onTimeframeChange={onTimeframeChange} />
    </QueryClientProvider>,
  );
}

const allBars = [
  { date: "2024-09-10", open: 1, high: 1, low: 1, close: 1, volume: 1 },
  { date: "2024-09-11", open: 1, high: 1, low: 1, close: 1, volume: 1 },
];

describe("StockReport", () => {
  beforeEach(() => {
    report.mockReset();
    configureApi({
      baseUrl: "http://test",
      fetch: async (input) => {
        const req = input as Request;
        const path = new URL(req.url).pathname;
        if (path.startsWith("/api/bars/")) {
          // Like the server: `to` is an inclusive cutoff.
          const to = new URL(req.url).searchParams.get("to") ?? "9999-12-31";
          return json(200, { symbol: "X", bars: allBars.filter((b) => b.date <= to) });
        }
        const symbol = decodeURIComponent(path.replace("/api/report/", ""));
        const res = report(symbol, await req.json());
        return json(res.status, res.body);
      },
    });
  });

  afterEach(() => configureApi({}));

  test("StockReport renders pass/can't-tell lines and 'insufficient history'", async () => {
    report.mockReturnValueOnce({ status: 200, body: breakout });
    const { unmount } = renderReport("DIVISLAB");
    expect(await screen.findByText("Pass")).toBeInTheDocument();
    expect(
      screen.getByText(/Old high 4,105.20 set 2021-10-25, 2 years 10 months ago \(717 bars\)/),
    ).toBeInTheDocument();
    unmount();

    report.mockReturnValueOnce({ status: 200, body: tooNew });
    renderReport("NEWCO");
    expect(await screen.findByText("Can't tell")).toBeInTheDocument();
    expect(screen.getByText("not evaluable (insufficient history)")).toBeInTheDocument();
    expect(screen.getByText("Can't tell yet: insufficient history.")).toBeInTheDocument();
  });

  test("StockReport leads with the breakout verdict and the level", async () => {
    report.mockReturnValueOnce({ status: 200, body: breakout });
    renderReport("DIVISLAB", "2024-09-10");
    expect(await screen.findByRole("heading", { name: "DIVISLAB" })).toBeInTheDocument();
    expect(screen.getByText("as of 2024-09-10")).toBeInTheDocument();
    const verdict = screen.getByTestId("verdict");
    expect(verdict).toHaveTextContent(
      "Breaking out: closed at ₹4,300.00, 4.75% above its 3-year high of ₹4,105.20 (set 2021-10-25).",
    );
    expect(screen.getByText("1 of 1 conditions pass")).toBeInTheDocument();
    expect(report).toHaveBeenCalledWith("DIVISLAB", { as_of: "2024-09-10" });
    const chart = await screen.findByTestId("chart");
    expect(chart).toHaveAttribute("data-level", "4105.2");
    // The 2024-09-11 candle is after as_of, so it must not be charted.
    await waitFor(() => expect(chart).toHaveAttribute("data-bars", "1"));
  });

  test("chart is empty when no bar exists on or before as_of", async () => {
    report.mockReturnValueOnce({ status: 200, body: { ...breakout, as_of: "" } });
    renderReport("DIVISLAB", "2001-01-01");
    expect(await screen.findByText("no data on or before that date")).toBeInTheDocument();
    await waitFor(() => expect(screen.getByTestId("chart")).toHaveAttribute("data-bars", "0"));
  });

  test("weekly timeframe asks for weekly bars and says so", async () => {
    report.mockReturnValueOnce({ status: 200, body: { ...breakout, timeframe: "weekly" } });
    renderReport("DIVISLAB", "2024-09-10", "weekly");
    expect(await screen.findByText(/evaluated on the weekly bar/)).toBeInTheDocument();
    expect(report).toHaveBeenCalledWith("DIVISLAB", { as_of: "2024-09-10", timeframe: "weekly" });
    expect(screen.getByRole("button", { name: "Weekly" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByRole("button", { name: "Daily" })).toHaveAttribute("aria-pressed", "false");
  });

  test("timeframe toggle reports the chosen timeframe", async () => {
    report.mockReturnValueOnce({ status: 200, body: breakout });
    const onChange = vi.fn();
    renderReport("DIVISLAB", undefined, undefined, onChange);
    await screen.findByText("Pass");
    expect(report).toHaveBeenCalledWith("DIVISLAB", {});
    fireEvent.click(screen.getByRole("button", { name: "Weekly" }));
    expect(onChange).toHaveBeenCalledWith("weekly");
  });

  test("unknown symbol shows the API error", async () => {
    report.mockReturnValueOnce({
      status: 404,
      body: { title: "Not Found", status: 404, detail: "unknown symbol NOPE" },
    });
    renderReport("NOPE");
    expect(await screen.findByRole("alert")).toHaveTextContent("unknown symbol NOPE");
  });
});
