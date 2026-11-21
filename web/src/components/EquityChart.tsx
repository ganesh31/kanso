import { ColorType, createChart, LineSeries, type IChartApi, type ISeriesApi } from "lightweight-charts";
import { useEffect, useRef } from "react";
import type { EquityPoint } from "@/api/client";

const EQUITY_COLOR = "#38bdf8";
const NIFTY_COLOR = "#a3a3a3";

const toLine = (points: EquityPoint[]) => points.map((p) => ({ time: p.date, value: p.value }));

/** The portfolio's equity against buy-and-hold Nifty 50 from the same capital. TradingView's attribution logo stays on (license). */
export function EquityChart({ equity, benchmark }: { equity: EquityPoint[]; benchmark: EquityPoint[] }) {
  const el = useRef<HTMLDivElement>(null);
  const chart = useRef<IChartApi | null>(null);
  const lines = useRef<{ equity: ISeriesApi<"Line">; nifty: ISeriesApi<"Line"> } | null>(null);

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
      localization: { priceFormatter: (v: number) => `₹${Math.round(v).toLocaleString("en-IN")}` },
    });
    chart.current = c;
    lines.current = {
      nifty: c.addSeries(LineSeries, { color: NIFTY_COLOR, lineWidth: 2, title: "Nifty 50" }),
      equity: c.addSeries(LineSeries, { color: EQUITY_COLOR, lineWidth: 2, title: "Portfolio" }),
    };
    return () => {
      c.remove();
      chart.current = null;
      lines.current = null;
    };
  }, []);

  useEffect(() => {
    lines.current?.equity.setData(toLine(equity));
    lines.current?.nifty.setData(toLine(benchmark));
    chart.current?.timeScale().fitContent();
  }, [equity, benchmark]);

  return (
    <figure className="flex flex-col gap-1">
      <figcaption className="text-muted-foreground flex gap-4 text-xs">
        <span>
          <span style={{ color: EQUITY_COLOR }}>■</span> Portfolio
        </span>
        <span>
          <span style={{ color: NIFTY_COLOR }}>■</span> Nifty 50 (buy and hold)
        </span>
      </figcaption>
      <div ref={el} className="h-[360px] w-full" data-testid="equity-chart" />
    </figure>
  );
}
