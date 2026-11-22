import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, test, vi } from "vite-plus/test";
import { configureApi, type Meta, type ParamSpecUnit, type System } from "@/api/client";
import { encodeDraft, fromSystem } from "@/state/draft";
import { SystemBuilder } from "./SystemBuilder";

vi.mock("./EquityChart", () => ({ EquityChart: () => <div data-testid="equity-chart" /> }));

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const num = (name: string, label: string, unit: ParamSpecUnit, def: number, min: number, max: number) => ({
  name,
  label,
  kind: "number" as const,
  unit,
  default: def,
  min,
  max,
  step: 0,
});

const patterns: Meta[] = [
  {
    type: "n_year_high",
    label: "New N-year high",
    description: "",
    provides: ["breakout_level"],
    requires: [],
    params: [num("years", "Years", "years", 3, 1, 20)],
  },
  {
    type: "not_extended",
    label: "Not extended",
    description: "",
    provides: [],
    requires: ["breakout_level"],
    params: [num("pct", "Max % above level", "", 8, 0, 50)],
  },
  {
    type: "volume_surge",
    label: "Volume surge",
    description: "",
    provides: [],
    requires: [],
    params: [
      num("k", "Multiple of average volume", "", 2, 1, 10),
      num("window_weeks", "Average over (weeks)", "weeks", 10, 1, 52),
    ],
  },
  {
    type: "relative_strength",
    label: "Relative strength",
    description: "",
    provides: [],
    requires: [],
    params: [num("rs_months", "Over (months)", "months", 6, 1, 24)],
  },
];

const defaultSystem: System = {
  name: "3-year breakout",
  universe: "NIFTY50",
  timeframe: "daily",
  entry: {
    all: [
      { type: "n_year_high", params: { years: 3 } },
      { type: "volume_surge", params: { k: 2, window_weeks: 10 } },
    ],
  },
  exits: { stop_pct: 8, trend_sma_weeks: 30 },
  costs: { per_side_pct: 0.2, slippage_pct: 0.1 },
  portfolio: { capital: 1000000, max_positions: 10 },
};

/** Each API call's body, by "METHOD path". */
const calls = vi.fn<(key: string, body: unknown) => Response | undefined>();
let saved: System[] = [];

function Harness({ initial, onDraft }: { initial?: string; onDraft?: (d: string) => void }) {
  const [draft, setDraft] = useState(initial);
  return (
    <SystemBuilder
      draftParam={draft}
      onDraftParamChange={(d) => {
        setDraft(d);
        onDraft?.(d);
      }}
      onOpenReport={() => {}}
    />
  );
}

function renderBuilder(initial?: string, onDraft?: (d: string) => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Harness initial={initial} onDraft={onDraft} />
    </QueryClientProvider>,
  );
}

const lastBody = (key: string) => calls.mock.calls.filter(([k]) => k === key).at(-1)?.[1];

describe("SystemBuilder", () => {
  beforeEach(() => {
    calls.mockReset();
    saved = [];
    configureApi({
      baseUrl: "http://test",
      fetch: async (input) => {
        const req = input as Request;
        const path = new URL(req.url).pathname;
        const key = `${req.method} ${path}`;
        const body = req.method === "GET" || req.method === "DELETE" ? undefined : await req.json();
        const answer = calls(key, body);
        if (answer) return answer;
        if (path === "/api/default-system") return json(200, { system: defaultSystem });
        if (path === "/api/patterns") return json(200, { patterns });
        if (path === "/api/universes") {
          return json(200, {
            universes: [
              { name: "NIFTY50", members: 50, has_history: false },
              { name: "NIFTY500", members: 500, has_history: false },
            ],
          });
        }
        if (key === "GET /api/systems") return json(200, { systems: saved });
        if (key === "POST /api/systems") {
          const sys = { ...(body as System), id: "new-id" };
          saved = [...saved, sys];
          return json(201, sys);
        }
        if (key === "POST /api/scan") {
          return json(200, { as_of: "2024-09-10", matches: [], stale: [], skipped: [], auto_inserted: [] });
        }
        return json(404, { title: "Not Found" });
      },
    });
  });

  afterEach(() => configureApi({}));

  test("opens on the default system and backtests it", async () => {
    calls.mockImplementation((key) => (key === "POST /api/backtest" ? json(500, { title: "stop here" }) : undefined));
    renderBuilder();
    expect(await screen.findByRole("group", { name: "New N-year high" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Volume surge" })).toHaveTextContent(
      "Needs volume at least 2× the 10-week average",
    );
    fireEvent.click(screen.getByRole("button", { name: "Backtest" }));
    await waitFor(() => expect(lastBody("POST /api/backtest")).toEqual(defaultSystem));
  });

  test("opens on a Quick Scan draft, not the default conditions, with the default exits", async () => {
    const draft: System = {
      name: "Not extended",
      universe: "NIFTY50",
      timeframe: "weekly",
      entry: { all: [{ type: "n_year_high" }, { type: "not_extended", params: { pct: 5 } }] },
    };
    calls.mockImplementation((key) => (key === "POST /api/backtest" ? json(500, { title: "stop here" }) : undefined));
    renderBuilder(encodeDraft(fromSystem(draft)));
    expect(await screen.findByRole("group", { name: "Not extended" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Volume surge" })).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Backtest" }));
    await waitFor(() =>
      expect(lastBody("POST /api/backtest")).toEqual({
        ...draft,
        exits: defaultSystem.exits,
        costs: defaultSystem.costs,
        portfolio: defaultSystem.portfolio,
      }),
    );
  });

  test("an edit reaches ?draft= and survives a reload", async () => {
    const onDraft = vi.fn();
    const { unmount } = renderBuilder(undefined, onDraft);
    const block = await screen.findByRole("group", { name: "Volume surge" });
    fireEvent.change(within(block).getByLabelText("Multiple of average volume"), { target: { value: "3" } });
    await waitFor(() => expect(onDraft).toHaveBeenCalled());
    const link = onDraft.mock.calls.at(-1)?.[0] as string;
    unmount();

    renderBuilder(link);
    const reloaded = await screen.findByRole("group", { name: "Volume surge" });
    expect(within(reloaded).getByLabelText("Multiple of average volume")).toHaveValue(3);
    expect(reloaded).toHaveTextContent("Needs volume at least 3× the 10-week average");
  });

  test("toggling the timeframe keeps window_weeks unchanged", async () => {
    calls.mockImplementation((key) => (key === "POST /api/backtest" ? json(500, { title: "stop here" }) : undefined));
    renderBuilder();
    fireEvent.click(await screen.findByRole("button", { name: "Weekly" }));
    fireEvent.click(screen.getByRole("button", { name: "Backtest" }));
    await waitFor(() => expect(lastBody("POST /api/backtest")).toMatchObject({ timeframe: "weekly" }));
    const sent = lastBody("POST /api/backtest") as System;
    expect(sent.entry.all[1]?.params).toEqual({ k: 2, window_weeks: 10 });
  });

  test("API field errors show next to the right block", async () => {
    calls.mockImplementation((key) =>
      key === "POST /api/backtest"
        ? json(400, {
            title: "Bad Request",
            detail: "invalid request",
            errors: [
              { location: "body.entry.all[1].params", message: "k must be at most 10" },
              { location: "body.entry.any", message: "needs at least 2 conditions" },
              { location: "body.exits.stop_pct", message: "must be more than 0 and at most 50" },
            ],
          })
        : undefined,
    );
    renderBuilder();
    const any = await screen.findByRole("group", { name: "Match ANY of these" });
    fireEvent.change(within(any).getByLabelText("Add a condition"), { target: { value: "relative_strength" } });
    fireEvent.click(within(any).getByRole("button", { name: "Add" }));
    fireEvent.click(screen.getByRole("button", { name: "Backtest" }));

    expect(
      await within(screen.getByRole("group", { name: "Volume surge" })).findByText("k must be at most 10"),
    ).toBeVisible();
    expect(within(screen.getByRole("group", { name: "New N-year high" })).queryByText(/must be/)).toBeNull();
    expect(within(any).getByText("needs at least 2 conditions")).toBeVisible();
    expect(
      within(screen.getByRole("group", { name: "Exits" })).getByText("Stop-loss: must be more than 0 and at most 50"),
    ).toBeVisible();
  });

  test("a switched-off condition is not sent", async () => {
    calls.mockImplementation((key) => (key === "POST /api/backtest" ? json(500, { title: "stop here" }) : undefined));
    renderBuilder();
    const nyh = await screen.findByRole("group", { name: "New N-year high" });
    fireEvent.click(within(nyh).getByRole("switch", { name: "Use New N-year high" }));
    fireEvent.click(screen.getByRole("button", { name: "Backtest" }));
    await waitFor(() =>
      expect((lastBody("POST /api/backtest") as System).entry.all.map((c) => c.type)).toEqual(["volume_surge"]),
    );
  });

  test("the ANY group's add button is enabled only while it has fewer than 4 items", async () => {
    renderBuilder();
    const any = await screen.findByRole("group", { name: "Match ANY of these" });
    const add = within(any).getByRole("button", { name: "Add" });
    await waitFor(() => expect(add).toBeEnabled());
    for (let i = 0; i < 4; i++) {
      expect(add).toBeEnabled();
      fireEvent.click(add);
    }
    expect(within(any).getAllByRole("switch")).toHaveLength(4);
    expect(add).toBeDisabled();
  });

  test("Scan scans the draft across its stock list", async () => {
    renderBuilder();
    fireEvent.click(await screen.findByRole("button", { name: "Scan" }));
    expect(await screen.findByText("0 matches")).toBeInTheDocument();
    expect(lastBody("POST /api/scan")).toEqual({ system: defaultSystem, mode: "fresh" });
  });

  test("save stores the draft as a system and load opens a saved one", async () => {
    saved = [{ ...defaultSystem, id: "rs", name: "RS only", entry: { all: [{ type: "relative_strength" }] } }];
    renderBuilder();
    fireEvent.change(await screen.findByLabelText("System name"), { target: { value: "Mine" } });
    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(lastBody("POST /api/systems")).toEqual({ ...defaultSystem, name: "Mine" }));
    expect(await screen.findByText("Saved.")).toBeInTheDocument();
    expect(await screen.findByRole("option", { name: "Mine" })).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Saved systems"), { target: { value: "rs" } });
    fireEvent.click(screen.getByRole("button", { name: "Load" }));
    expect(await screen.findByRole("group", { name: "Relative strength" })).toBeInTheDocument();
    expect(screen.queryByRole("group", { name: "Volume surge" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("System name")).toHaveValue("RS only");
  });
});
