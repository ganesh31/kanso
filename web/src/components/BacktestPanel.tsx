import {
  ApiError,
  useBacktest,
  useGetDefaultSystem,
  type Condition,
  type Honesty,
  type PortfolioStats,
  type Trade,
  type TradeStats,
} from "@/api/client";
import { EquityChart } from "@/components/EquityChart";
import { Button } from "@/components/ui/button";
import { formatPrice, formatSignedPct, humanize } from "@/lib/format";

function describeCondition(c: Condition): string {
  const params = Object.entries(c.params ?? {})
    .filter(([, v]) => v !== false)
    .map(([k, v]) => `${humanize(k)} ${String(v)}`);
  return params.length > 0 ? `${humanize(c.type)} (${params.join(", ")})` : humanize(c.type);
}

function Tile({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-card flex flex-col gap-1 rounded-md border px-3 py-2">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className="text-lg font-semibold tabular-nums">{value}</span>
    </div>
  );
}

function TradeTiles({ stats }: { stats: TradeStats }) {
  return (
    <section aria-label="Trade-level stats" className="grid grid-cols-2 gap-2 sm:grid-cols-3 lg:grid-cols-6">
      <Tile label="Trades" value={String(stats.trades)} />
      <Tile label="Win rate" value={`${(stats.win_rate * 100).toFixed(1)}%`} />
      <Tile label="Avg win" value={formatSignedPct(stats.avg_win_pct)} />
      <Tile label="Avg loss" value={formatSignedPct(stats.avg_loss_pct)} />
      <Tile label="Expectancy" value={formatSignedPct(stats.expectancy_pct)} />
      <Tile label="Median hold" value={`${stats.median_holding_days} days`} />
    </section>
  );
}

/** A portfolio metric with buy-and-hold Nifty's value beside it. */
function ComparedTile({ label, value, nifty }: { label: string; value: string; nifty: string }) {
  return (
    <div role="group" aria-label={label} className="bg-card flex flex-col gap-1 rounded-md border px-3 py-2">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className="flex items-baseline gap-2">
        <span className="text-lg font-semibold tabular-nums">{value}</span>
        <span className="text-muted-foreground text-sm tabular-nums">Nifty {nifty}</span>
      </span>
    </div>
  );
}

function PlainTile({ label, value }: { label: string; value: string }) {
  return (
    <div role="group" aria-label={label} className="bg-card flex flex-col gap-1 rounded-md border px-3 py-2">
      <span className="text-muted-foreground text-xs">{label}</span>
      <span className="text-lg font-semibold tabular-nums">{value}</span>
    </div>
  );
}

/** Fractions from the API (0.142) as signed percents (+14.20%). */
const fractionPct = (f: number) => formatSignedPct(f * 100);

function PortfolioTiles({ stats }: { stats: PortfolioStats }) {
  return (
    <section aria-label="Portfolio stats" className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-4">
      <ComparedTile label="CAGR" value={fractionPct(stats.cagr)} nifty={fractionPct(stats.bench_cagr)} />
      <ComparedTile
        label="Max drawdown"
        value={fractionPct(stats.max_drawdown)}
        nifty={fractionPct(stats.bench_max_drawdown)}
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

function TradeList({ trades }: { trades: Trade[] }) {
  if (trades.length === 0) return <p className="text-muted-foreground">No trades.</p>;
  return (
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
          {trades.map((t) => (
            <tr key={`${t.symbol}-${t.entry_date}`} className="border-t">
              <td className="py-1 pr-3 font-medium">{t.symbol}</td>
              <td className="py-1 pr-3">{t.signal_date}</td>
              <td className="py-1 pr-3">{t.entry_date}</td>
              <td className="py-1 pr-3 text-right">{formatPrice(t.entry_price)}</td>
              <td className="py-1 pr-3">{t.exit_date}</td>
              <td className="py-1 pr-3 text-right">{formatPrice(t.exit_price)}</td>
              <td className="py-1 pr-3">{exitLabel(t)}</td>
              <td className={`py-1 pr-3 text-right ${t.return_pct > 0 ? "text-emerald-500" : "text-red-500"}`}>
                {formatSignedPct(t.return_pct)}
              </td>
              <td className="py-1 text-right">{t.holding_days}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** The System Builder's first cut: the default system, a Backtest button, portfolio tiles and equity vs Nifty, trade-level tiles and the trade list. */
export function BacktestPanel() {
  const def = useGetDefaultSystem();
  const run = useBacktest();

  if (def.isPending) return <p className="text-muted-foreground">Loading the default system…</p>;
  if (def.isError)
    return (
      <p role="alert" className="text-destructive">
        {def.error.message}
      </p>
    );
  const system = def.data.system;

  return (
    <div className="flex flex-col gap-4">
      <section className="flex flex-col gap-2 rounded-md border p-3">
        <header className="flex flex-wrap items-baseline justify-between gap-2">
          <h2 className="text-lg font-semibold">{system.name}</h2>
          <span className="text-muted-foreground text-sm">
            {system.universe} · {system.timeframe ?? "daily"}
          </span>
        </header>
        <ul className="list-disc pl-5 text-sm">
          {system.entry.all.map((c) => (
            <li key={c.type}>{describeCondition(c)}</li>
          ))}
        </ul>
        {system.exits ? (
          <p className="text-muted-foreground text-sm">
            Stop {system.exits.stop_pct}% below entry
            {system.exits.trend_sma_weeks > 0
              ? `; exit when a week closes below its ${system.exits.trend_sma_weeks}-week average`
              : ""}
          </p>
        ) : null}
        <div>
          <Button onClick={() => run.mutate({ data: system })} disabled={run.isPending}>
            Backtest
          </Button>
        </div>
      </section>

      {run.isPending ? <p className="text-muted-foreground">Backtesting {system.universe}…</p> : null}
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
        <section className="flex flex-col gap-3">
          <h3 className="font-semibold">Portfolio</h3>
          <PortfolioTiles stats={run.data.portfolio} />
          <EquityChart equity={run.data.equity} benchmark={run.data.benchmark} />
          <h3 className="font-semibold">Trade-level (every signal, no slot limits)</h3>
          <TradeTiles stats={run.data.trade_stats} />
          <div className="text-muted-foreground flex flex-col gap-1 text-xs">
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
          <TradeList trades={run.data.trades} />
        </section>
      ) : null}
    </div>
  );
}
