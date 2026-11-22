import {
  CandlestickSeries,
  ColorType,
  createChart,
  createSeriesMarkers,
  LineStyle,
  type IChartApi,
  type ISeriesApi,
  type ISeriesMarkersPluginApi,
  type IPriceLine,
  type SeriesMarker,
  type Time,
} from "lightweight-charts";
import { useEffect, useRef } from "react";
import type { Bar } from "@/api/client";
import { formatPrice } from "@/lib/format";

/** A trade's entry or exit, drawn as a labelled arrow on its bar. */
export type ChartMarker = { date: string; kind: "entry" | "exit"; price: number };

/** Washi on indigo: markers stay neutral so they don't read as candles, a verdict or the level. */
const MARKER_COLOR = "#ece8de";

/**
 * Lightweight Charts markers for bars (ascending). A marker dated between bars (a weekly chart)
 * goes on the first bar dated on or after it, the bar that holds that day; one after the last bar
 * is dropped.
 */
export function seriesMarkers(bars: Bar[], markers: ChartMarker[]): SeriesMarker<Time>[] {
  const out: SeriesMarker<Time>[] = [];
  for (const m of markers) {
    const bar = bars.find((b) => b.date >= m.date);
    if (!bar) continue;
    out.push(
      m.kind === "entry"
        ? {
            time: bar.date,
            position: "belowBar",
            shape: "arrowUp",
            color: MARKER_COLOR,
            text: `Buy ${formatPrice(m.price)}`,
          }
        : {
            time: bar.date,
            position: "aboveBar",
            shape: "arrowDown",
            color: MARKER_COLOR,
            text: `Sell ${formatPrice(m.price)}`,
          },
    );
  }
  return out;
}

/** Candles with an optional dashed breakout-level line and trade markers. TradingView's attribution logo stays on (license). */
export function PriceChart({ bars, level, markers }: { bars: Bar[]; level?: number; markers?: ChartMarker[] }) {
  const el = useRef<HTMLDivElement>(null);
  const chart = useRef<IChartApi | null>(null);
  const candles = useRef<ISeriesApi<"Candlestick"> | null>(null);
  const markerApi = useRef<ISeriesMarkersPluginApi<Time> | null>(null);

  useEffect(() => {
    if (!el.current) return;
    const c = createChart(el.current, {
      autoSize: true,
      layout: {
        attributionLogo: true,
        background: { type: ColorType.Solid, color: "transparent" },
        textColor: "#a3adc2",
      },
      grid: { vertLines: { color: "#1f2c49" }, horzLines: { color: "#1f2c49" } },
      timeScale: { borderColor: "#2e3f62" },
      rightPriceScale: { borderColor: "#2e3f62" },
    });
    chart.current = c;
    const series = c.addSeries(CandlestickSeries, {
      upColor: "#79c49a",
      downColor: "#e5746a",
      borderVisible: false,
      wickUpColor: "#79c49a",
      wickDownColor: "#e5746a",
    });
    candles.current = series;
    markerApi.current = createSeriesMarkers(series);
    return () => {
      c.remove();
      chart.current = null;
      candles.current = null;
      markerApi.current = null;
    };
  }, []);

  useEffect(() => {
    candles.current?.setData(
      bars.map((b) => ({ time: b.date, open: b.open, high: b.high, low: b.low, close: b.close })),
    );
    chart.current?.timeScale().fitContent();
  }, [bars]);

  useEffect(() => {
    markerApi.current?.setMarkers(seriesMarkers(bars, markers ?? []));
  }, [bars, markers]);

  useEffect(() => {
    const series = candles.current;
    if (!series || level === undefined) return;
    const line: IPriceLine = series.createPriceLine({
      price: level,
      color: "#e3a63b",
      lineWidth: 2,
      lineStyle: LineStyle.Dashed,
      axisLabelVisible: true,
      title: "Breakout level",
    });
    // On unmount the chart (and this series) is removed first; a disposed series throws.
    return () => {
      if (candles.current === series) series.removePriceLine(line);
    };
  }, [level]);

  return <div ref={el} className="h-[420px] w-full" data-testid="price-chart" />;
}
