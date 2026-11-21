import type { Evidence, ReportLine } from "@/api/client";
import { formatAge, formatPctVs, formatPrice, formatSignedPct, formatVolume, humanize } from "@/lib/format";
import { cn } from "@/lib/utils";

/**
 * Plain-English evidence. The switch is exhaustive over the generated union:
 * a new evidence type in the API fails type-checking here until it is described.
 */
export function describeEvidence(ev: Evidence, asOf: string): string {
  switch (ev.type) {
    case "n_year_high":
      return (
        `Old high ${formatPrice(ev.level)} set ${ev.level_date}, ${formatAge(ev.level_date, asOf)} ` +
        `(${ev.bars_since} bars); close ${formatPrice(ev.close)}, ${formatPctVs(ev.pct_above)}`
      );
    case "level_age":
      return `Old high set ${ev.level_date}, ${formatAge(ev.level_date, asOf)} (${ev.bars_since} bars)`;
    case "not_extended":
      return `Close ${formatPctVs(ev.pct_above)} the breakout level`;
    case "relative_strength":
      return (
        `Stock ${formatSignedPct(ev.stock_return * 100)} vs benchmark ${formatSignedPct(ev.bench_return * 100)}: ` +
        `${Math.abs(ev.diff * 100).toFixed(2)} points ${ev.diff > 0 ? "ahead" : "behind"}`
      );
    case "volume_surge":
      return `Volume ${ev.ratio.toFixed(2)}× the average of ${formatVolume(ev.avg_volume)}`;
    case "not_evaluable":
      return `not evaluable (${humanize(ev.reason)})`;
    default: {
      const unhandled: never = ev;
      return unhandled;
    }
  }
}

const num = (v: unknown) => (typeof v === "number" ? v : undefined);
const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`;
const benchmarkName = (sym: unknown) => (typeof sym === "string" && sym !== "^NSEI" ? sym : "Nifty 50");

/**
 * The rule a line tests, read from its params, so a verdict is never shown without its threshold
 * ("196 bars" means nothing until you know it needed a year). Undefined for an unknown type.
 */
export function describeRule(type: string, params: Record<string, unknown>): string | undefined {
  switch (type) {
    case "n_year_high":
      return params.all_time === true
        ? "Close above the all-time high"
        : `Close above the highest high of the prior ${plural(num(params.years) ?? 0, "year")}`;
    case "level_age":
      return `Needs an old high set at least ${plural(num(params.min_age_years) ?? 0, "year")} ago`;
    case "not_extended":
      return `Needs the close at most ${num(params.pct) ?? 0}% above the breakout level`;
    case "relative_strength":
      return `Needs a better ${num(params.rs_months) ?? 0}-month return than ${benchmarkName(params.benchmark)}`;
    case "volume_surge":
      return `Needs volume at least ${num(params.k) ?? 0}× the ${num(params.window_weeks) ?? 0}-week average`;
    default:
      return undefined;
  }
}

type Verdict = "pass" | "fail" | "unknown";

export function verdictOf(pass: boolean, evaluable: boolean): Verdict {
  if (!evaluable) return "unknown";
  return pass ? "pass" : "fail";
}

const marks: Record<Verdict, { text: string; glyph: string; className: string }> = {
  pass: { text: "Pass", glyph: "✓", className: "text-pass border-pass/40 bg-pass/10" },
  fail: { text: "Fail", glyph: "✕", className: "text-fail border-fail/40 bg-fail/10" },
  unknown: { text: "Can't tell", glyph: "?", className: "text-muted-foreground border-border bg-muted" },
};

/** A verdict as a word, never colour alone. */
export function VerdictMark({ verdict }: { verdict: Verdict }) {
  const m = marks[verdict];
  return (
    <span
      className={cn(
        "inline-flex h-6 shrink-0 items-center gap-1 justify-self-start rounded-full border px-2.5 text-xs font-semibold whitespace-nowrap",
        m.className,
      )}
    >
      <span aria-hidden>{m.glyph}</span>
      {m.text}
    </span>
  );
}

export function EvidenceList({ lines, asOf }: { lines: ReportLine[]; asOf: string }) {
  return (
    <ul className="divide-border divide-y">
      {lines.map((l) => {
        const { pass, evaluable, evidence } = l.result;
        const rule = describeRule(l.type, l.params ?? {});
        return (
          <li key={l.type} className="grid grid-cols-[6rem_1fr] items-start gap-x-4 py-4">
            <VerdictMark verdict={verdictOf(pass, evaluable)} />
            <div className="flex min-w-0 flex-col gap-0.5">
              <div className="font-semibold">{l.label}</div>
              {rule ? <div className="text-muted-foreground text-sm">{rule}</div> : null}
              <div className={cn("text-sm", evaluable ? "text-foreground/90" : "text-muted-foreground")}>
                {describeEvidence(evidence, asOf)}
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}
