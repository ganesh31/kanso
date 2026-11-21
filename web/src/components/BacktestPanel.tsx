import {
  ApiError,
  useBacktest,
  useGetDefaultSystem,
  useListPatterns,
  type Condition,
  type Honesty,
  type PortfolioStats,
  type Trade,
  type TradeStats,
} from "@/api/client";
import { useState } from "react";
import { EquityChart } from "@/components/EquityChart";
import { describeRule } from "@/components/EvidenceList";
import { Button } from "@/components/ui/button";
import { formatPrice, formatSignedPct, humanize } from "@/lib/format";
import { cn } from "@/lib/utils";

/** A condition as its registry label and the rule its params set, e.g. "Volume surge: needs volume at least 2×…". */
function ConditionItem({ c, labels }: { c: Condition; labels: Map<string, string> }) {
  const rule = describeRule(c.type, c.params ?? {});
  return (
    <li className="flex flex-col">
      <span className="font-semibold">{labels.get(c.type) ?? humanize(c.type)}</span>
      {rule ? <span className="text-muted-foreground text-sm">{rule}</span> : null}
    </li>
  );
}

function Tile({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5 border-l-2 pl-3">
      <span className="text-muted-foreground text-sm">{label}</span>
      <span className="text-lg font-bold tabular-nums">{value}</span>
    </div>
  );
}

function TradeTiles({ stats }: { stats: TradeStats }) {
  return (
    <section aria-label="Trade-level stats" className="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-6">
      <Tile label="Trades" value={String(stats.trades)} />
      <Tile label="Win rate" value={`${(stats.win_rate * 100).toFixed(1)}%`} />
      <Tile label="Avg win" value={formatSignedPct(stats.avg_win_pct)} />
      <Tile label="Avg loss" value={formatSignedPct(stats.avg_loss_pct)} />
      <Tile label="Expectancy" value={formatSignedPct(stats.expectancy_pct)} />
      <Tile label="Median hold" value={`${stats.median_holding_days} days`} />
    </section>
  );
}

/** A portfolio metric with buy-and-hold Nifty's value beside it, and which one did better. */
function ComparedTile({
  label,
  value,
  nifty,
  better,
  verdict,
}: {
  label: string;
  value: string;
  nifty: string;
  better: boolean;
  verdict: string;
}) {
  return (
    <div role="group" aria-label={label} className="bg-card flex flex-col gap-1 rounded-md border px-4 py-3">
      <span className="text-muted-foreground text-sm">{label}</span>
      <span className="text-2xl font-bold tabular-nums">{value}</span>
      <span className="text-sm tabular-nums">
        <span className="text-muted-foreground">Nifty {nifty}, </span>
        <span className={better ? "text-pass" : "text-fail"}>{verdict}</span>
      </span>
    </div>
  );
}

function PlainTile({ label, value }: { label: string; value: string }) {
  return (
    <div role="group" aria-label={label} className="bg-card flex flex-col gap-1 rounded-md border px-4 py-3">
      <span className="text-muted-foreground text-sm">{label}</span>
      <span className="text-2xl font-bold tabular-nums">{value}</span>
    </div>
  );
}

/** Fractions from the API (0.142) as signed percents (+14.20%). */
const fractionPct = (f: number) => formatSignedPct(f * 100);

function PortfolioTiles({ stats }: { stats: PortfolioStats }) {
  return (
    <section aria-label="Portfolio stats" className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
      <ComparedTile
        label="CAGR"
        value={fractionPct(stats.cagr)}
        nifty={fractionPct(stats.bench_cagr)}
        better={stats.cagr >= stats.bench_cagr}
        verdict={stats.cagr >= stats.bench_cagr ? "ahead" : "behind"}
      />
      <ComparedTile
        label="Max drawdown"
        value={fractionPct(stats.max_drawdown)}
        nifty={fractionPct(stats.bench_max_drawdown)}
        better={stats.max_drawdown >= stats.bench_max_drawdown}
        verdict={stats.max_drawdown >= stats.bench_max_drawdown ? "shallower" : "deeper"}
      />
      <PlainTile label="Final equity" value={`₹${formatPrice(stats.final_equity)}`} />
      <PlainTile label="Exposure" value={`${(stats.exposure * 100).toFixed(1)}%`} />
    </section>
  );
}

const SKIP_REASONS: Record<string, string> = {
  price_above_slot: "price above slot size",
  insufficient_cash: "insufficient cash",
};
const SKIPPED_SHOWN = 5;

/** "1 signal skipped: price above slot size (MRF 2024-03-04)", one line per reason. */
function skippedSignalLines(h: Honesty): string[] {
  const byReason = new Map<string, string[]>();
  for (const s of h.skipped_signals) {
    byReason.set(s.reason, [...(byReason.get(s.reason) ?? []), `${s.symbol} ${s.entry_date}`]);
  }
  return [...byReason].map(([reason, list]) => {
    const shown = list.slice(0, SKIPPED_SHOWN).join(", ");
    const more = list.length > SKIPPED_SHOWN ? ` and ${list.length - SKIPPED_SHOWN} more` : "";
    const noun = list.length === 1 ? "signal" : "signals";
    return `${list.length} ${noun} skipped: ${SKIP_REASONS[reason] ?? reason} (${shown}${more})`;
  });
}

function exitLabel(t: Trade): string {
  return t.exit_reason === "stop" && t.gapped_stop ? "stop (gapped)" : t.exit_reason;
}

const TRADES_SHOWN = 25;

function TradeList({ trades }: { trades: Trade[] }) {
  const [all, setAll] = useState(false);
  if (trades.length === 0) return <p className="text-muted-foreground">No trades.</p>;
  const shown = all ? trades : trades.slice(0, TRADES_SHOWN);
  return (
    <div className="flex flex-col gap-3">
      <div className="overflow-x-auto">
        <table aria-label="Trades" className="w-full text-sm">
          <thead className="text-muted-foreground text-left">
            <tr>
              <th className="py-1 pr-3 font-medium">Symbol</th>
              <th className="py-1 pr-3 font-medium">Signal</th>
              <th className="py-1 pr-3 font-medium">Entry</th>
              <th className="py-1 pr-3 text-right font-medium">Entry price</th>
              <th className="py-1 pr-3 font-medium">Exit</th>
              <th className="py-1 pr-3 text-right font-medium">Exit price</th>
              <th className="py-1 pr-3 font-medium">Reason</th>
              <th className="py-1 pr-3 text-right font-medium">Return</th>
              <th className="py-1 text-right font-medium">Days</th>
            </tr>
          </thead>
          <tbody className="tabular-nums">
            {shown.map((t) => (
              <tr key={`${t.symbol}-${t.entry_date}`} className="border-t">
                <td className="py-1 pr-3 font-medium">{t.symbol}</td>
                <td className="py-1 pr-3">{t.signal_date}</td>
                <td className="py-1 pr-3">{t.entry_date}</td>
                <td className="py-1 pr-3 text-right">{formatPrice(t.entry_price)}</td>
                <td className="py-1 pr-3">{t.exit_date}</td>
                <td className="py-1 pr-3 text-right">{formatPrice(t.exit_price)}</td>
                <td className="py-1 pr-3">{exitLabel(t)}</td>
                <td className={cn("py-1 pr-3 text-right", t.return_pct > 0 ? "text-pass" : "text-fail")}>
                  {formatSignedPct(t.return_pct)}
                </td>
                <td className="py-1 text-right">{t.holding_days}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {trades.length > TRADES_SHOWN ? (
        <Button variant="outline" className="self-start" onClick={() => setAll((a) => !a)}>
          {all ? `Show the first ${TRADES_SHOWN} trades` : `Show all ${trades.length} trades`}
        </Button>
      ) : null}
    </div>
  );
}

/** The System Builder's first cut: the default system, a Backtest button, portfolio tiles and equity vs Nifty, trade-level tiles and the trade list. */
export function BacktestPanel() {
  const def = useGetDefaultSystem();
  const run = useBacktest();
  const patterns = useListPatterns({ query: { select: (b) => b.patterns } });
  const labels = new Map((patterns.data ?? []).map((p) => [p.type, p.label]));

  if (def.isPending) return <p className="text-muted-foreground">Loading the default system…</p>;
  if (def.isError)
    return (
      <p role="alert" className="text-destructive">
        {def.error.message}
      </p>
    );
  const system = def.data.system;

  return (
    <div className="flex flex-col gap-8">
      <section aria-labelledby="system-name" className="bg-card flex flex-col gap-4 rounded-lg border p-5">
        <header className="flex flex-col gap-0.5">
          <h2 id="system-name" className="text-xl font-bold">
            {system.name}
          </h2>
          <span className="text-muted-foreground text-sm">
            {`Buys ${system.universe} stocks when every condition below holds on a ${system.timeframe ?? "daily"} bar`}
          </span>
        </header>
        <ul className="grid gap-3 sm:grid-cols-2">
          {system.entry.all.map((c) => (
            <ConditionItem key={c.type} c={c} labels={labels} />
          ))}
        </ul>
        {system.exits ? (
          <p className="border-t pt-3 text-sm">
            <span className="font-semibold">Exits: </span>
            stop {system.exits.stop_pct}% below entry
            {system.exits.trend_sma_weeks > 0
              ? `, or sell when a week closes below its ${system.exits.trend_sma_weeks}-week average`
              : ""}
            .
          </p>
        ) : null}
        <div>
          <Button onClick={() => run.mutate({ data: system })} disabled={run.isPending}>
            Backtest
          </Button>
        </div>
      </section>

      {run.isPending ? <p className="text-muted-foreground">Backtesting {system.universe} since 2007…</p> : null}
      {run.isError ? (
        <div role="alert" className="text-destructive">
          <p>{run.error.message}</p>
          {run.error instanceof ApiError && run.error.fields.length > 0 ? (
            <ul className="list-disc pl-5 text-sm">
              {run.error.fields.map((f) => (
                <li key={f.path}>
                  {f.path}: {f.message}
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : null}
      {run.isSuccess ? (
        <section className="flex flex-col gap-8">
          <div className="flex flex-col gap-3">
            <h3 className="text-lg font-bold">Portfolio</h3>
            <PortfolioTiles stats={run.data.portfolio} />
            <EquityChart equity={run.data.equity} benchmark={run.data.benchmark} />
          </div>
          <div className="text-muted-foreground flex flex-col gap-1 border-l-2 pl-4 text-sm">
            <h3 className="text-foreground font-semibold">How this was tested</h3>
            <p>
              Fills at the {run.data.honesty.fill_model}; costs {run.data.honesty.per_side_pct}% and slippage{" "}
              {run.data.honesty.slippage_pct}% per side. Data {run.data.honesty.data_start} to{" "}
              {run.data.honesty.data_end}
              {run.data.honesty.survivorship ? "; today's stock list only (survivorship bias)" : ""}
              {run.data.honesty.skipped.length > 0 ? `; ${run.data.honesty.skipped.length} stocks with no data` : ""}.
              Portfolio: {run.data.honesty.ranking_rule}.
            </p>
            {skippedSignalLines(run.data.honesty).map((line) => (
              <p key={line}>{line}</p>
            ))}
          </div>
          <div className="flex flex-col gap-3">
            <h3 className="text-lg font-bold">Every signal</h3>
            <p className="text-muted-foreground -mt-2 text-sm">
              Each signal traded on its own, with no slot or cash limits, to show how the pattern itself behaves.
            </p>
            <TradeTiles stats={run.data.trade_stats} />
            <TradeList trades={run.data.trades} />
          </div>
        </section>
      ) : null}
    </div>
  );
}
