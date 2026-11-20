import {
  CandlestickSeries,
  ColorType,
  createChart,
  LineStyle,
  type IChartApi,
  type ISeriesApi,
  type IPriceLine,
} from "lightweight-charts";
import { useEffect, useRef } from "react";
import type { Bar } from "@/api/client";

/** Candles with an optional dashed breakout-level line. TradingView's attribution logo stays on (license). */
export function PriceChart({ bars, level }: { bars: Bar[]; level?: number }) {
  const el = useRef<HTMLDivElement>(null);
  const chart = useRef<IChartApi | null>(null);
  const candles = useRef<ISeriesApi<"Candlestick"> | null>(null);

  useEffect(() => {
    if (!el.current) return;
    const c = createChart(el.current, {
      autoSize: true,
      layout: {
        attributionLogo: true,
        background: { type: ColorType.Solid, color: "transparent" },
        textColor: "#a3a3a3",
      },
      grid: { vertLines: { color: "#262626" }, horzLines: { color: "#262626" } },
      timeScale: { borderColor: "#404040" },
      rightPriceScale: { borderColor: "#404040" },
    });
    chart.current = c;
    candles.current = c.addSeries(CandlestickSeries, {
      upColor: "#22c55e",
      downColor: "#ef4444",
      borderVisible: false,
      wickUpColor: "#22c55e",
      wickDownColor: "#ef4444",
    });
    return () => {
      c.remove();
      chart.current = null;
      candles.current = null;
    };
  }, []);

  useEffect(() => {
    candles.current?.setData(
      bars.map((b) => ({ time: b.date, open: b.open, high: b.high, low: b.low, close: b.close })),
    );
    chart.current?.timeScale().fitContent();
  }, [bars]);

  useEffect(() => {
    const series = candles.current;
    if (!series || level === undefined) return;
    const line: IPriceLine = series.createPriceLine({
      price: level,
      color: "#f59e0b",
      lineWidth: 2,
      lineStyle: LineStyle.Dashed,
      axisLabelVisible: true,
      title: "Breakout level",
    });
    return () => series.removePriceLine(line);
  }, [level]);

  return <div ref={el} className="h-[420px] w-full" data-testid="price-chart" />;
}
