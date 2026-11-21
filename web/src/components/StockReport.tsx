import { type ReportLine, type ReportRequest, type Timeframe, useGetBars, useStockReport } from "@/api/client";
import { SegmentedTrack, segmentClass } from "@/components/ui/segmented";
import { formatPctVs, formatPrice, humanize } from "@/lib/format";
import { describeEvidence, EvidenceList } from "./EvidenceList";
import { PriceChart } from "./PriceChart";

const timeframes: { value: Timeframe; label: string }[] = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
];

/**
 * The report's answer in one sentence, from the N-year-high line: breaking out or not, by how
 * much, against which old high. The level price is the one gold value on the page.
 */
function BreakoutVerdict({ lines }: { lines: ReportLine[] }) {
  const line = lines.find((l) => l.type === "n_year_high");
  if (!line) return null;
  const ev = line.result.evidence;
  if (ev.type !== "n_year_high") {
    return (
      <p data-testid="verdict" className="text-lg">
        {ev.type === "not_evaluable"
          ? `Can't tell yet: ${humanize(ev.reason)}.`
          : `Can't tell yet: ${describeEvidence(ev, "")}.`}
      </p>
    );
  }
  const years = typeof line.params?.years === "number" ? line.params.years : undefined;
  const span = line.params?.all_time === true || years === undefined ? "all-time" : `${years}-year`;
  return (
    <p data-testid="verdict" className="max-w-3xl text-lg">
      <span className={line.result.pass ? "text-pass font-semibold" : "font-semibold"}>
        {line.result.pass ? "Breaking out" : "Not breaking out"}
      </span>
      {`: closed at ₹${formatPrice(ev.close)}, ${formatPctVs(ev.pct_above)} its ${span} high of `}
      <span className="text-level font-semibold">{`₹${formatPrice(ev.level)}`}</span>{" "}
      <span className="whitespace-nowrap">{`(set ${ev.level_date}).`}</span>
    </p>
  );
}

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
  const passing = r.lines.filter((l) => l.result.pass).length;
  return (
    <div className="flex flex-col gap-8">
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
          <h1 className="text-4xl font-bold tracking-tight">{r.symbol}</h1>
          <span className="text-muted-foreground text-sm">
            {r.as_of ? `as of ${r.as_of}` : "no data on or before that date"}
          </span>
        </div>
        <BreakoutVerdict lines={r.lines} />
        {r.lines.length > 0 ? (
          <p className="text-muted-foreground text-sm">{`${passing} of ${r.lines.length} conditions pass`}</p>
        ) : null}
      </header>

      <figure className="bg-card rounded-lg border p-2">
        {bars.isError ? (
          <p className="text-destructive p-4 text-sm">Chart unavailable: {bars.error.message}</p>
        ) : (
          <PriceChart bars={bars.data ?? []} level={r.level} />
        )}
        {r.level !== undefined ? (
          <figcaption className="text-muted-foreground flex items-center gap-2 px-2 pt-2 text-xs">
            <span aria-hidden className="border-level inline-block w-5 border-t-2 border-dashed" />
            Breakout level {formatPrice(r.level)}
            {r.level_date ? `, set ${r.level_date}` : ""}
          </figcaption>
        ) : null}
      </figure>

      <section aria-labelledby="conditions" className="flex flex-col gap-2">
        <div className="flex flex-wrap items-end justify-between gap-3 border-b pb-3">
          <div className="flex flex-col gap-1">
            <h2 id="conditions" className="text-lg font-bold">
              Conditions
            </h2>
            <p className="text-muted-foreground text-sm">
              Every pattern with its default params, evaluated on the {r.timeframe} bar
              {r.timeframe === "weekly" ? " (the last complete week)" : ""}.
            </p>
          </div>
          <SegmentedTrack role="group" aria-label="Timeframe">
            {timeframes.map((t) => (
              <button
                key={t.value}
                type="button"
                className={segmentClass(t.value === timeframe)}
                aria-pressed={t.value === timeframe}
                onClick={() => onTimeframeChange(t.value)}
              >
                {t.label}
              </button>
            ))}
          </SegmentedTrack>
        </div>
        <EvidenceList lines={r.lines} asOf={r.as_of} />
      </section>
    </div>
  );
}
