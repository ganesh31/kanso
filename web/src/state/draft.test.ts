import { describe, expect, test } from "vite-plus/test";
import type { System } from "@/api/client";
import { decodeDraft, encodeDraft, fromSystem, systemReducer, toSystem } from "./draft";

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

describe("systemReducer", () => {
  test("toggling the timeframe keeps window_weeks unchanged", () => {
    const weekly = systemReducer(fromSystem(system), { type: "setTimeframe", timeframe: "weekly" });
    const daily = systemReducer(weekly, { type: "setTimeframe", timeframe: "daily" });
    expect(toSystem(weekly).timeframe).toBe("weekly");
    expect(toSystem(weekly).entry.all[1]?.params).toEqual({ k: 2, window_weeks: 10 });
    expect(toSystem(daily).entry.all[1]?.params).toEqual({ k: 2, window_weeks: 10 });
  });

  test("a condition toggled off is left out of the system, and back on is sent again", () => {
    const off = systemReducer(fromSystem(system), { type: "toggle", group: "all", index: 0 });
    expect(toSystem(off).entry.all.map((c) => c.type)).toEqual(["volume_surge"]);
    const on = systemReducer(off, { type: "toggle", group: "all", index: 0 });
    expect(toSystem(on).entry.all.map((c) => c.type)).toEqual(["n_year_high", "volume_surge"]);
  });

  test("add, set a param, move to the ANY group and remove", () => {
    let d = systemReducer(fromSystem(system), {
      type: "addCondition",
      group: "any",
      condition: { type: "relative_strength", params: { rs_months: 6, benchmark: "^NSEI" } },
    });
    d = systemReducer(d, { type: "setParam", group: "any", index: 0, name: "rs_months", value: 12 });
    d = systemReducer(d, { type: "setGroup", from: "all", index: 1, to: "any" });
    expect(toSystem(d).entry).toEqual({
      all: [{ type: "n_year_high", params: { years: 3, all_time: false } }],
      any: [
        { type: "relative_strength", params: { rs_months: 12, benchmark: "^NSEI" } },
        { type: "volume_surge", params: { k: 2, window_weeks: 10 } },
      ],
    });
    d = systemReducer(d, { type: "removeCondition", group: "any", index: 0 });
    expect(toSystem(d).entry.any).toEqual([{ type: "volume_surge", params: { k: 2, window_weeks: 10 } }]);
  });

  test("exits, costs, portfolio, universe and load", () => {
    let d = systemReducer(fromSystem(system), { type: "setExits", exits: { stop_pct: 10, trend_sma_weeks: 0 } });
    d = systemReducer(d, { type: "setCosts", costs: { per_side_pct: 0.1, slippage_pct: 0 } });
    d = systemReducer(d, { type: "setPortfolio", portfolio: { capital: 500000, max_positions: 5 } });
    d = systemReducer(d, { type: "setUniverse", universe: "NIFTY500" });
    expect(toSystem(d)).toMatchObject({
      universe: "NIFTY500",
      exits: { stop_pct: 10, trend_sma_weeks: 0 },
      costs: { per_side_pct: 0.1, slippage_pct: 0 },
      portfolio: { capital: 500000, max_positions: 5 },
    });
    expect(toSystem(systemReducer(d, { type: "load", system }))).toEqual(system);
  });
});

describe("the ?draft= param", () => {
  test("a draft round-trips, toggled-off conditions included", () => {
    const d = systemReducer(fromSystem(system), { type: "toggle", group: "all", index: 1 });
    const encoded = encodeDraft(d);
    expect(encoded).toMatch(/^[A-Za-z0-9_-]+$/);
    expect(decodeDraft(encoded)).toEqual(d);
  });

  test("non-ASCII names survive", () => {
    const d = fromSystem({ ...system, name: "Breakout ₹ – 3y" });
    expect(decodeDraft(encodeDraft(d))?.name).toBe("Breakout ₹ – 3y");
  });

  test("garbage decodes to undefined", () => {
    expect(decodeDraft("not base64 json")).toBeUndefined();
    expect(decodeDraft(encodeDraft(fromSystem(system)).slice(0, 10))).toBeUndefined();
    expect(decodeDraft(btoa(JSON.stringify({ name: 1 })))).toBeUndefined();
  });
});
