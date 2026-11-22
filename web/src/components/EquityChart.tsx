import {
  ColorType,
  createChart,
  LineSeries,
  PriceScaleMode,
  type IChartApi,
  type ISeriesApi,
} from "lightweight-charts";
import { useEffect, useRef } from "react";
import type { EquityPoint } from "@/api/client";

const EQUITY_COLOR = "#ece8de";
const NIFTY_COLOR = "#7f8aa3";

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
        textColor: "#a3adc2",
      },
      grid: { vertLines: { color: "#1f2c49" }, horzLines: { color: "#1f2c49" } },
      // Twenty years of daily closes must fit: the default 0.5px minimum bar spacing would crop them.
      timeScale: { borderColor: "#2e3f62", minBarSpacing: 0.01 },
      // Log scale: 19 years of compounding, where equal % moves should look equal.
      rightPriceScale: { borderColor: "#2e3f62", mode: PriceScaleMode.Logarithmic },
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
        <span>Log scale</span>
      </figcaption>
      <div ref={el} className="h-[360px] w-full" data-testid="equity-chart" />
    </figure>
  );
}
