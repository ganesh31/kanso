import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vite-plus/test";
import { configureApi, type ScanResult, type System } from "@/api/client";
import type { ScannerSearch } from "@/lib/scannerSearch";
import { SavedScan } from "./SavedScan";

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const breakout: System = {
  id: "sys-1",
  name: "Breakout",
  universe: "NIFTY500",
  timeframe: "weekly",
  entry: { all: [{ type: "n_year_high", params: { years: 3 } }] },
};
const volume: System = {
  id: "sys-2",
  name: "Volume only",
  universe: "NIFTY50",
  timeframe: "daily",
  entry: { all: [{ type: "volume_surge", params: { k: 2 } }] },
};

const result: ScanResult = {
  as_of: "2024-09-06",
  matches: [
    {
      symbol: "TCS",
      date: "2024-09-06",
      signal_date: "2024-09-06",
      outcome: { pass: true, evaluable: true, conditions: [], any_matched: [] },
    },
  ],
  stale: [],
  skipped: [],
  auto_inserted: [],
};

const scan = vi.fn<(body: unknown) => ScanResult>();
let saved: System[] = [];

function Harness({ initial, onOpen }: { initial: ScannerSearch; onOpen: (s: string) => void }) {
  const [search, setSearch] = useState<ScannerSearch>(initial);
  return (
    <>
      <SavedScan search={search} onSearchChange={setSearch} onOpenReport={onOpen} />
      <output data-testid="search">{JSON.stringify(search)}</output>
    </>
  );
}

function renderSavedScan(initial: ScannerSearch = {}, onOpen: (s: string) => void = () => {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Harness initial={initial} onOpen={onOpen} />
    </QueryClientProvider>,
  );
}

describe("SavedScan", () => {
  beforeEach(() => {
    saved = [breakout, volume];
    scan.mockReset();
    scan.mockReturnValue(result);
    configureApi({
      baseUrl: "http://test",
      fetch: async (input) => {
        const req = input as Request;
        const path = new URL(req.url).pathname;
        if (path === "/api/systems") return json(200, { systems: saved });
        if (path === "/api/scan") return json(200, scan(await req.json()));
        return json(404, { title: "Not Found" });
      },
    });
  });

  afterEach(() => configureApi({}));

  test("picking a saved system scans it and puts it in the URL", async () => {
    renderSavedScan();
    await screen.findByRole("option", { name: "Volume only" });
    expect(scan).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("System"), { target: { value: "sys-1" } });
    await waitFor(() => expect(scan).toHaveBeenCalledTimes(1));
    expect(scan).toHaveBeenLastCalledWith({ system: breakout, mode: "fresh" });
    expect(JSON.parse(screen.getByTestId("search").textContent)).toMatchObject({ tab: "saved", system: "sys-1" });
    expect(await screen.findByText("as of 2024-09-06")).toBeInTheDocument();
    expect(screen.getByText("Scans NIFTY500 on weekly bars for n_year_high")).toBeInTheDocument();
  });

  test("a system in the URL is scanned on load, and the mode toggle rescans", async () => {
    renderSavedScan({ tab: "saved", system: "sys-2" });
    await waitFor(() => expect(scan).toHaveBeenCalledTimes(1));
    expect(scan).toHaveBeenLastCalledWith({ system: volume, mode: "fresh" });

    fireEvent.click(screen.getByRole("button", { name: "All qualifying" }));
    await waitFor(() => expect(scan).toHaveBeenCalledTimes(2));
    expect(scan).toHaveBeenLastCalledWith({ system: volume, mode: "all" });
    expect(screen.getByRole("button", { name: "All qualifying" })).toHaveAttribute("aria-pressed", "true");
  });

  test("a row click opens the stock report", async () => {
    const onOpen = vi.fn();
    renderSavedScan({ system: "sys-1" }, onOpen);
    fireEvent.click(await screen.findByRole("button", { name: "TCS" }));
    expect(onOpen).toHaveBeenCalledWith("TCS", "weekly");
  });

  test("an unknown system in the URL says so instead of scanning", async () => {
    renderSavedScan({ system: "gone" });
    expect(await screen.findByText(/no longer saved/)).toBeInTheDocument();
    expect(scan).not.toHaveBeenCalled();
  });

  test("with no saved systems it points at Quick Scan", async () => {
    saved = [];
    renderSavedScan();
    expect(await screen.findByText(/No saved systems yet/)).toBeInTheDocument();
  });
});
