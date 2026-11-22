import type { BacktestResult, PortfolioStats, Trade, TradeStats } from "@/api/client";
import { useState } from "react";
import { EquityChart } from "@/components/EquityChart";
import { HonestyPanel } from "@/components/HonestyPanel";
import { exitLabel, TradeChart } from "@/components/TradeChart";
import { Button } from "@/components/ui/button";
import { formatPrice, formatSignedPct } from "@/lib/format";
import { cn } from "@/lib/utils";

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

const TRADES_SHOWN = 25;

const tradeKey = (t: Trade) => `${t.symbol}-${t.entry_date}`;

/** Every trade; clicking one opens its chart above the table. */
function TradeList({ trades }: { trades: Trade[] }) {
  const [all, setAll] = useState(false);
  const [open, setOpen] = useState<string | null>(null);
  if (trades.length === 0) return <p className="text-muted-foreground">No trades.</p>;
  const shown = all ? trades : trades.slice(0, TRADES_SHOWN);
  const charted = trades.find((t) => tradeKey(t) === open);
  return (
    <div className="flex flex-col gap-3">
      {charted ? <TradeChart key={open} trade={charted} onClose={() => setOpen(null)} /> : null}
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
              <tr key={tradeKey(t)} className={cn("border-t", tradeKey(t) === open && "bg-muted")}>
                <td className="py-1 pr-3 font-medium">
                  <button
                    type="button"
                    aria-label={`Chart the ${t.symbol} trade entered ${t.entry_date}`}
                    aria-pressed={tradeKey(t) === open}
                    className="hover:text-foreground focus-visible:ring-ring rounded-sm underline decoration-dotted underline-offset-4 focus-visible:ring-2 focus-visible:outline-none"
                    onClick={() => setOpen((cur) => (cur === tradeKey(t) ? null : tradeKey(t)))}
                  >
                    {t.symbol}
                  </button>
                </td>
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

/**
 * A backtest: portfolio tiles and equity vs Nifty, trade-level tiles and the trade list, with the
 * honesty panel (and the draft's variations count) beside them.
 */
export function BacktestResults({ result, variations }: { result: BacktestResult; variations: number }) {
  return (
    <div className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_20rem]">
      <section aria-label="Results" className="flex min-w-0 flex-col gap-8">
        <div className="flex flex-col gap-3">
          <h3 className="text-lg font-bold">Portfolio</h3>
          <PortfolioTiles stats={result.portfolio} />
          <EquityChart equity={result.equity} benchmark={result.benchmark} />
        </div>
        <div className="flex flex-col gap-3">
          <h3 className="text-lg font-bold">Every signal</h3>
          <p className="text-muted-foreground -mt-2 text-sm">
            Each signal traded on its own, with no slot or cash limits, to show how the pattern itself behaves.
          </p>
          <TradeTiles stats={result.trade_stats} />
          <TradeList trades={result.trades} />
        </div>
      </section>
      <div className="lg:sticky lg:top-6">
        <HonestyPanel honesty={result.honesty} variations={variations} />
      </div>
    </div>
  );
}
