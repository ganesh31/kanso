import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { afterEach, beforeEach, describe, expect, test } from "vite-plus/test";
import { configureApi, type Meta, type ScanResult, type System } from "@/api/client";
import type { ScannerSearch } from "@/lib/scannerSearch";
import { Scanner } from "./Scanner";

function json(status: number, body: unknown) {
  return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } });
}

const patterns: Meta[] = [
  {
    type: "volume_surge",
    label: "Volume surge",
    description: "",
    provides: [],
    requires: [],
    params: [{ name: "k", label: "Multiple", kind: "number", unit: "", default: 2, min: 1, max: 10, step: 0 }],
  },
];

const result: ScanResult = { as_of: "2024-09-10", matches: [], stale: [], skipped: [], auto_inserted: [] };

let stored: System[] = [];
let scanned: unknown[] = [];

function Harness({ initial }: { initial: ScannerSearch }) {
  const [search, setSearch] = useState<ScannerSearch>(initial);
  return (
    <>
      <Scanner search={search} onSearchChange={setSearch} onOpenReport={() => {}} />
      <output data-testid="search">{JSON.stringify(search)}</output>
    </>
  );
}

function renderScanner(initial: ScannerSearch = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Harness initial={initial} />
    </QueryClientProvider>,
  );
}

const search = () => JSON.parse(screen.getByTestId("search").textContent) as ScannerSearch;

describe("Scanner", () => {
  beforeEach(() => {
    stored = [];
    scanned = [];
    configureApi({
      baseUrl: "http://test",
      fetch: async (input) => {
        const req = input as Request;
        const path = new URL(req.url).pathname;
        if (path === "/api/patterns") return json(200, { patterns });
        if (path === "/api/universes") {
          return json(200, { universes: [{ name: "NIFTY50", members: 50, has_history: false }] });
        }
        if (path === "/api/systems" && req.method === "POST") {
          const sys = { ...((await req.json()) as System), id: "id-1" };
          stored.push(sys);
          return json(201, sys);
        }
        if (path === "/api/systems") return json(200, { systems: stored });
        if (path === "/api/quickscan") return json(200, result);
        if (path === "/api/scan") {
          scanned.push(await req.json());
          return json(200, result);
        }
        return json(404, { title: "Not Found" });
      },
    });
  });

  afterEach(() => configureApi({}));

  test("opens on the Saved system tab, and the tab lives in the URL", async () => {
    renderScanner();
    expect(screen.getByRole("tab", { name: "Saved system" })).toHaveAttribute("aria-selected", "true");
    expect(await screen.findByText(/No saved systems yet/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: "Quick Scan" }));
    expect(search().tab).toBe("quick");
    expect(await screen.findByLabelText("Pattern")).toBeInTheDocument();
  });

  test("arrow keys move between the tabs; only the selected tab is in the Tab order", async () => {
    renderScanner();
    const saved = screen.getByRole("tab", { name: "Saved system" });
    const quick = screen.getByRole("tab", { name: "Quick Scan" });
    expect(saved).toHaveAttribute("tabindex", "0");
    expect(quick).toHaveAttribute("tabindex", "-1");
    saved.focus();
    fireEvent.keyDown(saved, { key: "ArrowRight" });
    expect(search().tab).toBe("quick");
    expect(quick).toHaveFocus();
    expect(quick).toHaveAttribute("tabindex", "0");
    fireEvent.keyDown(quick, { key: "ArrowRight" });
    expect(search().tab).toBe("saved");
    expect(await screen.findByRole("tabpanel", { name: "Saved system" })).toBeInTheDocument();
  });

  test("an old Quick Scan link without a tab still opens Quick Scan", async () => {
    renderScanner({ pattern: "volume_surge", universe: "NIFTY50" });
    expect(screen.getByRole("tab", { name: "Quick Scan" })).toHaveAttribute("aria-selected", "true");
  });

  test("a quick scan saved as a system opens on the Saved system tab and is scanned", async () => {
    renderScanner({ tab: "quick", pattern: "volume_surge", universe: "NIFTY50" });
    fireEvent.change(await screen.findByLabelText("System name"), { target: { value: "Volume" } });
    fireEvent.click(screen.getByRole("button", { name: "Save as system" }));

    await waitFor(() => expect(search()).toMatchObject({ tab: "saved", system: "id-1" }));
    await waitFor(() => expect(scanned).toHaveLength(1));
    expect(scanned[0]).toMatchObject({ system: { id: "id-1", name: "Volume" }, mode: "fresh" });
    expect(screen.getByLabelText("System")).toHaveValue("id-1");
  });
});
