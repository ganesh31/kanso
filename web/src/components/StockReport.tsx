import { type ReportRequest, type Timeframe, useGetBars, useStockReport } from "@/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { formatPrice } from "@/lib/format";
import { EvidenceList } from "./EvidenceList";
import { PriceChart } from "./PriceChart";

const timeframes: { value: Timeframe; label: string }[] = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
];

export function StockReport({
  symbol,
  asOf,
  timeframe = "daily",
  onTimeframeChange,
}: {
  symbol: string;
  asOf?: string;
  timeframe?: Timeframe;
  onTimeframeChange: (tf: Timeframe) => void;
}) {
  const request: ReportRequest = {};
  if (asOf) request.as_of = asOf;
  if (timeframe !== "daily") request.timeframe = timeframe;
  const report = useStockReport(symbol, request);
  // Cut the chart at as_of (inclusive) so it never shows candles from after the report date.
  const bars = useGetBars(symbol, asOf ? { to: asOf } : undefined, { query: { select: (body) => body.bars } });

  if (report.isPending) {
    return <p className="text-muted-foreground">Loading {symbol}…</p>;
  }
  if (report.isError) {
    return (
      <p role="alert" className="text-destructive">
        {report.error.message}
      </p>
    );
  }

  const r = report.data;
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <h1 className="text-2xl font-semibold tracking-tight">{r.symbol}</h1>
        <span className="text-muted-foreground text-sm">
          {r.as_of ? `as of ${r.as_of}` : "no data on or before that date"}
        </span>
        {r.level !== undefined && (
          <span className="text-level text-sm">
            Breakout level {formatPrice(r.level)}
            {r.level_date ? ` (set ${r.level_date})` : ""}
          </span>
        )}
      </header>

      <Card className="py-2">
        <CardContent className="px-2">
          {bars.isError ? (
            <p className="text-destructive p-4 text-sm">Chart unavailable: {bars.error.message}</p>
          ) : (
            <PriceChart bars={bars.data ?? []} level={r.level} />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-wrap items-start justify-between gap-2">
          <div className="flex flex-col gap-1.5">
            <CardTitle>Conditions</CardTitle>
            <CardDescription>
              Every pattern with its default params, evaluated on the {r.timeframe} bar
              {r.timeframe === "weekly" ? " (the last complete week)" : ""}.
            </CardDescription>
          </div>
          <div role="group" aria-label="Timeframe" className="flex gap-1">
            {timeframes.map((t) => (
              <Button
                key={t.value}
                size="sm"
                variant={t.value === timeframe ? "default" : "outline"}
                aria-pressed={t.value === timeframe}
                onClick={() => onTimeframeChange(t.value)}
              >
                {t.label}
              </Button>
            ))}
          </div>
        </CardHeader>
        <CardContent>
          <EvidenceList lines={r.lines} asOf={r.as_of} />
        </CardContent>
      </Card>
    </div>
  );
}
