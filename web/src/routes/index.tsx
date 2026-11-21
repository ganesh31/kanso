import { createFileRoute, Link } from "@tanstack/react-router";
import { type Match, type System, useGetDefaultSystem, useScan } from "@/api/client";
import { formatAge, formatPctVs, formatPrice } from "@/lib/format";

export const Route = createFileRoute("/")({
  component: Home,
});

/**
 * Home leads with the answer Kanso exists for: which stocks are breaking out today under the
 * default system, each linking to its Stock Report. The views below say what else you can do.
 */
function Home() {
  const def = useGetDefaultSystem({ query: { select: (b) => b.system } });
  return (
    <div className="flex flex-col gap-12">
      {def.data ? <TodaysBreakouts system={def.data} /> : <Intro />}
      <section aria-labelledby="views" className="flex flex-col gap-4">
        <h2 id="views" className="text-lg font-bold">
          Three ways in
        </h2>
        <ul className="grid gap-6 sm:grid-cols-3">
          <View title="Stock Report" to="/report/$symbol" params={{ symbol: "AXISBANK" }}>
            Pick one stock and see every condition pass or fail, with the numbers behind it and the old high drawn on
            the chart.
          </View>
          <View title="Scanner" to="/scanner">
            Run one pattern, or a saved system, across a stock list on the latest data.
          </View>
          <View title="System Builder" to="/builder">
            Backtest a rule since 2007 with next-day-open fills, costs and whole shares, against buying Nifty 50.
          </View>
        </ul>
      </section>
    </div>
  );
}

function Intro() {
  return (
    <header className="flex flex-col gap-2">
      <h1 className="text-4xl font-bold tracking-tight">Breaking out today</h1>
      <p className="text-muted-foreground">Loading today&apos;s scan…</p>
    </header>
  );
}

function TodaysBreakouts({ system }: { system: System }) {
  const scan = useScan({ system, mode: "all" });
  return (
    <section aria-labelledby="today" className="flex flex-col gap-5">
      <header className="flex flex-col gap-2">
        <h1 id="today" className="text-4xl font-bold tracking-tight">
          Breaking out today
        </h1>
        <p className="text-muted-foreground max-w-2xl">
          {scan.data
            ? `${system.universe} stocks that pass every condition of the "${system.name}" system on ${scan.data.as_of}: a new 3-year high on a volume surge, beating Nifty, and not yet extended.`
            : `Scanning ${system.universe} with the "${system.name}" system…`}
        </p>
      </header>
      {scan.isError ? (
        <p role="alert" className="text-destructive">
          {scan.error.message}
        </p>
      ) : scan.data && scan.data.matches.length === 0 ? (
        <p className="max-w-2xl">
          No stock passes every condition today.{" "}
          <Link
            to="/scanner"
            search={{ tab: "quick", pattern: "n_year_high", universe: system.universe, mode: "all" }}
            className="underline underline-offset-4"
          >
            See every stock at a new 3-year high
          </Link>
          , whatever its volume or strength.
        </p>
      ) : scan.data ? (
        <ul className="divide-border flex flex-col divide-y border-y">
          {scan.data.matches.map((m) => (
            <BreakoutRow key={m.symbol} m={m} />
          ))}
        </ul>
      ) : null}
    </section>
  );
}

function BreakoutRow({ m }: { m: Match }) {
  return (
    <li>
      <Link
        to="/report/$symbol"
        params={{ symbol: m.symbol }}
        className="hover:bg-card focus-visible:bg-card grid grid-cols-1 items-baseline gap-x-6 gap-y-1 px-2 py-4 sm:grid-cols-[12rem_1fr]"
      >
        <span className="text-2xl font-bold tracking-tight">{m.symbol}</span>
        <span className="text-muted-foreground">
          {m.pct_above !== undefined ? `Closed ${formatPctVs(m.pct_above)} ` : "Closed above "}
          {m.level_date ? `an old high set ${formatAge(m.level_date, m.date)}: ` : "its old high: "}
          {m.level !== undefined ? (
            <span className="text-level font-semibold">{`₹${formatPrice(m.level)}`}</span>
          ) : null}
        </span>
      </Link>
    </li>
  );
}

function View({
  title,
  children,
  ...link
}: {
  title: string;
  children: string;
} & ({ to: "/scanner" | "/builder"; params?: never } | { to: "/report/$symbol"; params: { symbol: string } })) {
  return (
    <li className="flex flex-col gap-1 border-t-2 pt-3">
      <Link {...link} className="font-bold underline-offset-4 hover:underline">
        {title}
      </Link>
      <p className="text-muted-foreground text-sm">{children}</p>
    </li>
  );
}
