import type { ReactNode } from "react";
import type { Honesty } from "@/api/client";
import { humanize } from "@/lib/format";
import { VARIATIONS_WARN_ABOVE } from "@/state/variations";

const SKIP_REASONS: Record<string, string> = {
  price_above_slot: "price above slot size",
  insufficient_cash: "insufficient cash",
  no_data: "no data",
};
const SKIPPED_SHOWN = 5;

const reasonText = (r: string) => SKIP_REASONS[r] ?? humanize(r);

/** "1 signal skipped: price above slot size (MRF 2024-03-04)", one line per reason. */
function skippedSignalLines(h: Honesty): string[] {
  const byReason = new Map<string, string[]>();
  for (const s of h.skipped_signals) {
    const list = byReason.get(s.reason);
    if (list) list.push(`${s.symbol} ${s.entry_date}`);
    else byReason.set(s.reason, [`${s.symbol} ${s.entry_date}`]);
  }
  return [...byReason].map(([reason, list]) => {
    const shown = list.slice(0, SKIPPED_SHOWN).join(", ");
    const more = list.length > SKIPPED_SHOWN ? ` and ${list.length - SKIPPED_SHOWN} more` : "";
    const noun = list.length === 1 ? "signal" : "signals";
    return `${list.length} ${noun} skipped: ${reasonText(reason)} (${shown}${more})`;
  });
}

function Row({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 border-t py-2.5 first:border-t-0 first:pt-0">
      <dt className="text-muted-foreground text-sm">{term}</dt>
      <dd className="text-sm">{children}</dd>
    </div>
  );
}

/** The assumptions behind a backtest, always shown beside its results (spec §5.8). */
export function HonestyPanel({ honesty: h, variations }: { honesty: Honesty; variations: number }) {
  const signalLines = skippedSignalLines(h);
  return (
    <aside aria-labelledby="honesty-heading" className="flex flex-col gap-3 border-l-2 pl-4">
      <h3 id="honesty-heading" className="text-lg font-bold">
        How this was tested
      </h3>
      <dl className="flex flex-col">
        <Row term="Fills">At the {h.fill_model}, the day after the signal</Row>
        <Row term="Costs">
          {h.per_side_pct}% per side, plus {h.slippage_pct}% per side slippage
        </Row>
        <Row term="Data">
          {h.data_start} to {h.data_end}, from {h.data_source}, {h.adjustment}
        </Row>
        <Row term="Stock list">
          {h.survivorship ? (
            <>
              <span className="font-semibold">Today&apos;s members only.</span> Stocks that left the list are missing,
              so results look better than they were (survivorship bias).
            </>
          ) : (
            "Members as they were on each date"
          )}
        </Row>
        <Row term="Crowded days">{h.ranking_rule}</Row>
        <Row term="Skipped signals">
          {signalLines.length === 0 ? (
            "None"
          ) : (
            <ul className="flex flex-col gap-1">
              {signalLines.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
          )}
        </Row>
        {h.skipped.length > 0 ? (
          <Row term="Skipped stocks">{h.skipped.map((s) => `${s.symbol} (${reasonText(s.reason)})`).join(", ")}</Row>
        ) : null}
        <Row term="Variations tested">
          <span className="tabular-nums">{variations}</span>
        </Row>
      </dl>
      {variations > VARIATIONS_WARN_ABOVE ? (
        <p role="alert" className="text-fail text-sm font-semibold">
          Tested {variations} variations. The more you try, the more likely the best one is luck.
        </p>
      ) : null}
    </aside>
  );
}
