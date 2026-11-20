import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { ApiError, type Report } from "@/api/client";
import { StockReport } from "./StockReport";

const { report, bars } = vi.hoisted(() => ({ report: vi.fn(), bars: vi.fn() }));

vi.mock("@/api/client", async (importOriginal) => {
  const mod = await importOriginal<typeof import("@/api/client")>();
  return { ...mod, api: { ...mod.api, report, bars } };
});

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
    bars.mockReset();
    bars.mockResolvedValue({ symbol: "X", bars: [{ date: "2024-09-10", open: 1, high: 1, low: 1, close: 1, volume: 1 }] });
  });

  test("StockReport renders ✅/❌ lines and 'insufficient history'", async () => {
    report.mockResolvedValueOnce(breakout);
    const { unmount } = renderReport("DIVISLAB");
    expect(await screen.findByText("✅")).toBeInTheDocument();
    expect(screen.getByText(/Old high 4,105.20 set 2021-10-25, 2 years 10 months ago \(717 bars\)/)).toBeInTheDocument();
    unmount();

    report.mockResolvedValueOnce(tooNew);
    renderReport("NEWCO");
    expect(await screen.findByText("❌")).toBeInTheDocument();
    expect(screen.getByText("not evaluable (insufficient history)")).toBeInTheDocument();
  });

  test("StockReport shows level line value in header", async () => {
    report.mockResolvedValueOnce(breakout);
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
    report.mockRejectedValueOnce(new ApiError(404, "unknown symbol NOPE"));
    renderReport("NOPE");
    expect(await screen.findByRole("alert")).toHaveTextContent("unknown symbol NOPE");
  });
});
