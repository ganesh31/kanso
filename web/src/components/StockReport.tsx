import { useGetBars, useStockReport } from "@/api/client";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { formatPrice } from "@/lib/format";
import { EvidenceList } from "./EvidenceList";
import { PriceChart } from "./PriceChart";

export function StockReport({ symbol, asOf }: { symbol: string; asOf?: string }) {
  const report = useStockReport(symbol, asOf ? { as_of: asOf } : {});
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
        <CardHeader>
          <CardTitle>Conditions</CardTitle>
          <CardDescription>Every pattern with its default params, evaluated on the {r.timeframe} bar.</CardDescription>
        </CardHeader>
        <CardContent>
          <EvidenceList lines={r.lines} asOf={r.as_of} />
        </CardContent>
      </Card>
    </div>
  );
}
