import type { Evidence, ReportLine } from "@/api/client";
import { formatAge, formatPctVs, formatPrice, humanize } from "@/lib/format";
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
    case "not_evaluable":
      return `not evaluable (${humanize(ev.reason)})`;
    default: {
      const unhandled: never = ev;
      return unhandled;
    }
  }
}

export function EvidenceList({ lines, asOf }: { lines: ReportLine[]; asOf: string }) {
  return (
    <ul className="divide-y divide-border">
      {lines.map((l) => {
        const { pass, evaluable, evidence } = l.result;
        return (
          <li key={l.type} className="flex gap-3 py-3">
            <span aria-label={pass ? "pass" : "fail"} className="text-lg leading-6">
              {pass ? "✅" : "❌"}
            </span>
            <div className="min-w-0">
              <div className="font-medium">{l.label}</div>
              <div className={cn("text-sm", evaluable ? "text-muted-foreground" : "text-destructive")}>
                {describeEvidence(evidence, asOf)}
              </div>
            </div>
          </li>
        );
      })}
    </ul>
  );
}
