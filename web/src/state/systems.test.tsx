import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, test } from "vite-plus/test";
import { configureApi, type System } from "@/api/client";
import { useSystems } from "./systems";

function json(status: number, body: unknown) {
  return new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const draft: System = { name: "Breakout", universe: "NIFTY50", entry: { all: [{ type: "n_year_high" }] } };

/** A fake systems API backed by a map, recording each request as "METHOD path". */
function fakeServer() {
  const stored = new Map<string, System>();
  const calls: string[] = [];
  let next = 1;
  configureApi({
    baseUrl: "http://test",
    fetch: async (input) => {
      const req = input as Request;
      const path = new URL(req.url).pathname;
      calls.push(`${req.method} ${path}`);
      const id = path.split("/")[3];
      if (path === "/api/systems" && req.method === "GET") {
        return json(200, { systems: [...stored.values()].sort((a, b) => a.name.localeCompare(b.name)) });
      }
      if (path === "/api/systems" && req.method === "POST") {
        const sys = { ...((await req.json()) as System), id: `id-${next++}`, timeframe: "daily" as const };
        stored.set(sys.id, sys);
        return json(201, sys);
      }
      if (id && req.method === "PUT") {
        const sys = { ...((await req.json()) as System), id };
        stored.set(id, sys);
        return json(200, sys);
      }
      if (id && req.method === "DELETE") {
        stored.delete(id);
        return new Response(null, { status: 204 });
      }
      return json(404, { title: "Not Found" });
    },
  });
  return { stored, calls };
}

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe("useSystems", () => {
  let server: ReturnType<typeof fakeServer>;
  beforeEach(() => {
    server = fakeServer();
  });
  afterEach(() => configureApi({}));

  test("lists saved systems from the API", async () => {
    server.stored.set("a", { ...draft, id: "a" });
    const { result } = renderHook(() => useSystems(), { wrapper });
    await waitFor(() => expect(result.current.systems).toHaveLength(1));
    expect(result.current.systems[0]).toMatchObject({ id: "a", name: "Breakout" });
  });

  test("save creates a new system, returns it with its id and refreshes the list", async () => {
    const { result } = renderHook(() => useSystems(), { wrapper });
    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.systems).toEqual([]);

    let saved: System | undefined;
    await act(async () => {
      saved = await result.current.save(draft);
    });
    expect(saved).toMatchObject({ id: "id-1", timeframe: "daily" });
    expect(server.calls).toContain("POST /api/systems");
    await waitFor(() => expect(result.current.systems.map((s) => s.id)).toEqual(["id-1"]));
  });

  test("save with an id replaces that system", async () => {
    server.stored.set("a", { ...draft, id: "a" });
    const { result } = renderHook(() => useSystems(), { wrapper });
    await waitFor(() => expect(result.current.systems).toHaveLength(1));
    await act(async () => {
      await result.current.save({ ...draft, id: "a", name: "Renamed" });
    });
    expect(server.calls).toContain("PUT /api/systems/a");
    await waitFor(() => expect(result.current.systems[0]?.name).toBe("Renamed"));
  });

  test("remove deletes a system and refreshes the list", async () => {
    server.stored.set("a", { ...draft, id: "a" });
    const { result } = renderHook(() => useSystems(), { wrapper });
    await waitFor(() => expect(result.current.systems).toHaveLength(1));
    await act(async () => {
      await result.current.remove("a");
    });
    expect(server.calls).toContain("DELETE /api/systems/a");
    await waitFor(() => expect(result.current.systems).toEqual([]));
  });
});
