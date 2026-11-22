import { useMemo } from "react";
import { type GetBarsParams, type Trade, useGetBars } from "@/api/client";
import { Button } from "@/components/ui/button";
import { formatPrice } from "@/lib/format";
import { type ChartMarker, PriceChart } from "./PriceChart";

/** A YYYY-MM-DD date moved by whole years and months (UTC, so no time zone shifts the day). */
function shiftDate(date: string, years: number, months: number): string {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCFullYear(d.getUTCFullYear() + years, d.getUTCMonth() + months);
  return d.toISOString().slice(0, 10);
}

/** Daily bars cover about 250 a year, so past this span a window would near the 5,000-bar limit. */
const WEEKLY_AFTER_YEARS = 15;

/**
 * The bars to fetch for a trade: a year before entry for the breakout's context, to three months
 * after the exit. A trade held for many years is charted on weekly bars to stay under the limit.
 */
export function tradeWindow(t: Trade): GetBarsParams {
  const from = shiftDate(t.entry_date, -1, 0);
  const to = shiftDate(t.exit_date, 0, 3);
  const years = (Date.parse(to) - Date.parse(from)) / (365.25 * 24 * 3600 * 1000);
  return years > WEEKLY_AFTER_YEARS ? { from, to, timeframe: "weekly" } : { from, to };
}

/** Entry and exit markers; a trade still open at the end of data has no exit. */
export function tradeMarkers(t: Trade): ChartMarker[] {
  const out: ChartMarker[] = [{ date: t.entry_date, kind: "entry", price: t.entry_price }];
  if (t.exit_reason !== "open") out.push({ date: t.exit_date, kind: "exit", price: t.exit_price });
  return out;
}

export function exitLabel(t: Trade): string {
  return t.exit_reason === "stop" && t.gapped_stop ? "stop (gapped)" : t.exit_reason;
}

/** One trade on its stock's chart: entry and exit markers and the breakout level line. */
export function TradeChart({ trade, onClose }: { trade: Trade; onClose: () => void }) {
  const range = tradeWindow(trade);
  const bars = useGetBars(trade.symbol, range, { query: { select: (body) => body.bars } });
  const markers = useMemo(() => tradeMarkers(trade), [trade]);
  const title = `${trade.symbol} trade chart`;

  return (
    <figure aria-label={title} className="flex flex-col gap-2 border-t pt-4">
      <header className="flex items-start justify-between gap-4">
        <div className="flex flex-col gap-0.5">
          <h4 className="font-semibold">{trade.symbol}</h4>
          <p className="text-muted-foreground text-sm tabular-nums">
            Bought {trade.entry_date} at {formatPrice(trade.entry_price)}.{" "}
            {trade.exit_reason === "open"
              ? `Still open at the end of the data, last close ${formatPrice(trade.exit_price)}.`
              : `Sold ${trade.exit_date} at ${formatPrice(trade.exit_price)}, ${exitLabel(trade)}.`}
          </p>
        </div>
        <Button variant="outline" size="sm" onClick={onClose}>
          Close chart
        </Button>
      </header>
      {bars.isError ? (
        <p className="text-destructive text-sm">Chart unavailable: {bars.error.message}</p>
      ) : (
        <PriceChart bars={bars.data ?? []} level={trade.level} markers={markers} />
      )}
      <figcaption className="text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs">
        <span>Buy and Sell arrows mark the fills{range.timeframe === "weekly" ? " (weekly bars)" : ""}.</span>
        {trade.level !== undefined ? (
          <span className="flex items-center gap-2">
            <span aria-hidden className="border-level inline-block w-5 border-t-2 border-dashed" />
            Breakout level {formatPrice(trade.level)}
            {trade.level_date ? `, set ${trade.level_date}` : ""}
          </span>
        ) : null}
      </figcaption>
    </figure>
  );
}
