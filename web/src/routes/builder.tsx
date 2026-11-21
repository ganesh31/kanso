import { createFileRoute } from "@tanstack/react-router";
import { BacktestPanel } from "@/components/BacktestPanel";

export const Route = createFileRoute("/builder")({
  component: BuilderPage,
});

function BuilderPage() {
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold tracking-tight">System Builder</h1>
        <p className="text-muted-foreground max-w-2xl">
          Test a breakout rule on every stock since 2007. Trades fill at the next day&apos;s open, after costs and
          slippage.
        </p>
      </header>
      <BacktestPanel />
    </div>
  );
}
