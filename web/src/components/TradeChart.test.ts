import { describe, expect, test } from "vite-plus/test";
import type { Bar, Trade } from "@/api/client";
import { seriesMarkers } from "./PriceChart";
import { tradeWindow } from "./TradeChart";

const trade: Trade = {
  symbol: "TCS",
  signal_date: "2024-02-06",
  entry_date: "2024-02-07",
  exit_date: "2024-03-01",
  entry_price: 4105.2,
  exit_price: 3776.78,
  exit_reason: "stop",
  gapped_stop: false,
  return_pct: -8.39,
  holding_days: 17,
};

const bar = (date: string): Bar => ({ date, open: 1, high: 1, low: 1, close: 1, volume: 1 });

describe("trade chart window", () => {
  test("a short trade is charted on daily bars from a year before entry to three months after exit", () => {
    expect(tradeWindow(trade)).toEqual({ from: "2023-02-07", to: "2024-06-01" });
  });

  test("a trade held for many years switches to weekly bars to stay under the bar limit", () => {
    expect(tradeWindow({ ...trade, entry_date: "2008-01-02", exit_date: "2024-03-01" })).toEqual({
      from: "2007-01-02",
      to: "2024-06-01",
      timeframe: "weekly",
    });
  });
});

describe("series markers", () => {
  test("a marker lands on the weekly bar holding its day, and one past the last bar is dropped", () => {
    const weekly = [bar("2024-02-02"), bar("2024-02-09"), bar("2024-03-01")];
    const got = seriesMarkers(weekly, [
      { date: "2024-02-07", kind: "entry", price: 4105.2 },
      { date: "2024-03-04", kind: "exit", price: 3776.78 },
    ]);
    expect(got).toHaveLength(1);
    expect(got[0]).toMatchObject({ time: "2024-02-09", position: "belowBar", shape: "arrowUp", text: "Buy 4,105.20" });
  });

  test("an exit is a Sell arrow above its bar", () => {
    const got = seriesMarkers([bar("2024-03-01")], [{ date: "2024-03-01", kind: "exit", price: 3776.78 }]);
    expect(got[0]).toMatchObject({
      time: "2024-03-01",
      position: "aboveBar",
      shape: "arrowDown",
      text: "Sell 3,776.78",
    });
  });
});
