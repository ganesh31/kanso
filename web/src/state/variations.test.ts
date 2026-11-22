import { beforeEach, describe, expect, test, vi } from "vite-plus/test";
import type { System } from "@/api/client";
import { countVariations, recordVariation } from "./variations";

const system: System = {
  name: "3-year breakout",
  universe: "NIFTY50",
  timeframe: "daily",
  entry: { all: [{ type: "n_year_high", params: { years: 3, all_time: false } }] },
  exits: { stop_pct: 8, trend_sma_weeks: 30 },
  costs: { per_side_pct: 0.2, slippage_pct: 0.1 },
  portfolio: { capital: 1000000, max_positions: 10 },
};

beforeEach(() => localStorage.clear());

describe("variations", () => {
  test("identical params counted once, changed stop counts twice", () => {
    expect(recordVariation("d1", system)).toBe(1);
    expect(recordVariation("d1", structuredClone(system))).toBe(1);
    expect(recordVariation("d1", { ...system, exits: { stop_pct: 10, trend_sma_weeks: 30 } })).toBe(2);
    expect(countVariations("d1")).toBe(2);
  });

  test("renaming system does not count", () => {
    recordVariation("d1", system);
    expect(recordVariation("d1", { ...system, name: "Renamed" })).toBe(1);
  });

  test("changing the stock list does not count", () => {
    recordVariation("d1", system);
    expect(recordVariation("d1", { ...system, universe: "NIFTY500" })).toBe(1);
  });

  test("key order does not matter", () => {
    recordVariation("d1", system);
    const reordered = { ...system, costs: { slippage_pct: 0.1, per_side_pct: 0.2 } };
    expect(recordVariation("d1", reordered)).toBe(1);
  });

  test("each draft counts on its own", () => {
    recordVariation("d1", system);
    expect(recordVariation("d2", { ...system, exits: { stop_pct: 10, trend_sma_weeks: 30 } })).toBe(1);
    expect(countVariations("d1")).toBe(1);
    expect(countVariations("never-run")).toBe(0);
  });

  test("counts survive in the browser's storage", () => {
    recordVariation("d1", system);
    expect(localStorage.getItem("kanso.variations.v1")).toContain("d1");
  });

  test("works in memory when storage throws", () => {
    const setItem = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("quota");
    });
    try {
      expect(recordVariation("d3", system)).toBe(1);
      expect(recordVariation("d3", { ...system, exits: { stop_pct: 5, trend_sma_weeks: 30 } })).toBe(2);
      expect(countVariations("d3")).toBe(2);
    } finally {
      setItem.mockRestore();
    }
  });
});
