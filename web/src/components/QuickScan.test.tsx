import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vite-plus/test";
import { configureApi, type Meta, type ScanResult } from "@/api/client";
import type { ScannerSearch } from "@/lib/scannerSearch";
import { QuickScan } from "./QuickScan";

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const patterns: Meta[] = [
  {
    type: "n_year_high",
    label: "New N-year high",
    description: "",
    provides: ["breakout_level"],
    requires: [],
    params: [
      { name: "years", label: "Years", kind: "number", unit: "years", default: 3, min: 1, max: 20, step: 1 },
      { name: "all_time", label: "All-time", kind: "bool", unit: "", default: false, min: 0, max: 0, step: 0 },
    ],
  },
  {
    type: "not_extended",
    label: "Not extended",
    description: "",
    provides: [],
    requires: ["breakout_level"],
    params: [
      { name: "pct", label: "Max % above level", kind: "number", unit: "", default: 8, min: 0, max: 50, step: 0 },
    ],
  },
  {
    type: "volume_surge",
    label: "Volume surge",
    description: "",
    provides: [],
    requires: [],
    params: [
      {
        name: "k",
        label: "Multiple of average volume",
        kind: "number",
        unit: "",
        default: 2,
        min: 1,
        max: 10,
        step: 0,
      },
      {
        name: "window_weeks",
        label: "Average over (weeks)",
        kind: "number",
        unit: "weeks",
        default: 10,
        min: 1,
        max: 52,
        step: 1,
      },
    ],
  },
];

const empty: ScanResult = { as_of: "2024-09-10", matches: [], stale: [], skipped: [], auto_inserted: [] };

/** The fake server records quickscan bodies and answers with what this returns. */
const quickscan = vi.fn<(body: unknown) => ScanResult>();

/** Holds the search params in state, as the router would. */
/** The fake server records created systems' bodies. */
const createSystem = vi.fn<(body: unknown) => void>();

/** When set, /api/patterns fails this many more times. */
let patternFailures = 0;

function Harness({
  initial = {},
  onOpen = () => {},
  onSaved = () => {},
}: {
  initial?: ScannerSearch;
  onOpen?: (s: string, tf: string) => void;
  onSaved?: (id: string) => void;
}) {
  const [search, setSearch] = useState<ScannerSearch>(initial);
  return <QuickScan search={search} onSearchChange={setSearch} onOpenReport={onOpen} onSaved={onSaved} />;
}

function renderQuickScan(
  initial?: ScannerSearch,
  onOpen?: (s: string, tf: string) => void,
  onSaved?: (id: string) => void,
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Harness initial={initial} onOpen={onOpen} onSaved={onSaved} />
    </QueryClientProvider>,
  );
}

describe("QuickScan", () => {
  beforeEach(() => {
    quickscan.mockReset();
    createSystem.mockReset();
    quickscan.mockReturnValue(empty);
    configureApi({
      baseUrl: "http://test",
      fetch: async (input) => {
        const req = input as Request;
        const path = new URL(req.url).pathname;
        if (path === "/api/patterns") {
          if (patternFailures > 0) {
            patternFailures--;
            return json(500, { title: "Internal Server Error", detail: "patterns unavailable" });
          }
          return json(200, { patterns });
        }
        if (path === "/api/universes") {
          return json(200, { universes: [{ name: "NIFTY50", members: 50, has_history: false }] });
        }
        if (path === "/api/systems" && req.method === "POST") {
          const body = (await req.json()) as object;
          createSystem(body);
          return json(201, { ...body, id: "new-id" });
        }
        if (path === "/api/systems") return json(200, { systems: [] });
        if (path === "/api/quickscan") {
          const body = (await req.json()) as { pattern: string };
          if (body.pattern === "nope") {
            return json(400, {
              title: "Bad Request",
              detail: "invalid request",
              errors: [{ location: "body.pattern", message: 'unknown pattern "nope"' }],
            });
          }
          return json(200, quickscan(body));
        }
        return json(404, { title: "Not Found" });
      },
    });
  });

  afterEach(() => configureApi({}));

  test("quick scan sends the selected pattern and edited params", async () => {
    renderQuickScan();
    await screen.findByRole("option", { name: "Volume surge" });
    fireEvent.change(screen.getByLabelText("Pattern"), { target: { value: "volume_surge" } });
    fireEvent.change(screen.getByLabelText(/Multiple of average volume/), { target: { value: "3" } });
    await screen.findByRole("option", { name: "NIFTY50" });
    fireEvent.click(screen.getByRole("button", { name: "Scan" }));
    await waitFor(() => expect(quickscan).toHaveBeenCalled());
    expect(quickscan).toHaveBeenLastCalledWith({
      pattern: "volume_surge",
      params: { k: 3, window_weeks: 10 },
      universe: "NIFTY50",
      timeframe: "daily",
      mode: "fresh",
    });
    expect(await screen.findByText("as of 2024-09-10")).toBeInTheDocument();
  });

  test("a param can be cleared and retyped, and Enter submits the scan", async () => {
    renderQuickScan();
    await screen.findByRole("option", { name: "NIFTY50" });
    const years = screen.getByLabelText(/Years/);
    fireEvent.change(years, { target: { value: "" } });
    expect(years).toHaveValue(null);
    fireEvent.change(years, { target: { value: "5" } });
    expect(years).toHaveValue(5);
    fireEvent.submit(years.closest("form")!);
    await waitFor(() => expect(quickscan).toHaveBeenCalled());
    expect(quickscan).toHaveBeenLastCalledWith(expect.objectContaining({ params: { years: 5, all_time: false } }));
  });

  test("a failed pattern list is shown with a retry", async () => {
    patternFailures = 1;
    renderQuickScan();
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't load the patterns and stock lists");
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByRole("option", { name: "Volume surge" })).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  test("a cleared param left empty shows its value again", async () => {
    renderQuickScan();
    await screen.findByRole("option", { name: "NIFTY50" });
    const years = screen.getByLabelText(/Years/);
    fireEvent.change(years, { target: { value: "" } });
    fireEvent.blur(years);
    expect(years).toHaveValue(3);
  });

  test("fresh/all toggle changes the request mode", async () => {
    renderQuickScan({ pattern: "n_year_high", universe: "NIFTY50" });
    await waitFor(() => expect(quickscan).toHaveBeenCalledTimes(1));
    expect(quickscan).toHaveBeenLastCalledWith(expect.objectContaining({ mode: "fresh" }));
    expect(screen.getByRole("button", { name: "Fresh signals" })).toHaveAttribute("aria-pressed", "true");

    fireEvent.click(screen.getByRole("button", { name: "All qualifying" }));
    await waitFor(() => expect(quickscan).toHaveBeenCalledTimes(2));
    expect(quickscan).toHaveBeenLastCalledWith(expect.objectContaining({ mode: "all" }));
    expect(screen.getByRole("button", { name: "All qualifying" })).toHaveAttribute("aria-pressed", "true");
  });

  test("the auto-inserted notice is shown", async () => {
    quickscan.mockReturnValue({ ...empty, auto_inserted: ["n_year_high"] });
    renderQuickScan({ pattern: "not_extended", universe: "NIFTY50" });
    expect(
      await screen.findByText("Added New N-year high (3 years) because Not extended needs a breakout level."),
    ).toBeInTheDocument();
  });

  test("stale and skipped counts and a row click open the report", async () => {
    quickscan.mockReturnValue({
      ...empty,
      stale: ["OLD1", "OLD2"],
      skipped: ["NEW"],
      matches: [
        {
          symbol: "TCS",
          date: "2024-09-10",
          signal_date: "2024-09-09",
          outcome: { pass: true, evaluable: true, conditions: [], any_matched: [] },
        },
      ],
    });
    const onOpen = vi.fn();
    renderQuickScan({ pattern: "volume_surge", universe: "NIFTY50" }, onOpen);
    expect(await screen.findByText("2 with stale data, 1 skipped")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "TCS" }));
    expect(onOpen).toHaveBeenCalledWith("TCS", "daily");
  });

  test("validation errors are shown with their field", async () => {
    renderQuickScan({ pattern: "nope", universe: "NIFTY50" });
    expect(await screen.findByRole("alert")).toHaveTextContent('pattern: unknown pattern "nope"');
  });

  test("save as system stores the scan as a system, auto-inserted condition first", async () => {
    quickscan.mockReturnValue({ ...empty, auto_inserted: ["n_year_high"] });
    const onSaved = vi.fn();
    renderQuickScan(
      { pattern: "not_extended", params: { pct: 5 }, universe: "NIFTY50", tf: "weekly" },
      undefined,
      onSaved,
    );
    fireEvent.change(await screen.findByLabelText("System name"), { target: { value: "Tight breakout" } });
    fireEvent.click(screen.getByRole("button", { name: "Save as system" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith("new-id"));
    expect(createSystem).toHaveBeenCalledWith({
      name: "Tight breakout",
      universe: "NIFTY50",
      timeframe: "weekly",
      entry: { all: [{ type: "n_year_high" }, { type: "not_extended", params: { pct: 5 } }] },
    });
  });

  test("save as system needs a name", async () => {
    renderQuickScan({ pattern: "volume_surge", universe: "NIFTY50" });
    expect(await screen.findByRole("button", { name: "Save as system" })).toBeDisabled();
  });
});
