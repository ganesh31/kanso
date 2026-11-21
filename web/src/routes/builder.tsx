import { createFileRoute } from "@tanstack/react-router";
import { BacktestPanel } from "@/components/BacktestPanel";

export const Route = createFileRoute("/builder")({
  component: BuilderPage,
});

function BuilderPage() {
  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-baseline gap-x-4">
        <h1 className="text-2xl font-semibold tracking-tight">System Builder</h1>
        <span className="text-muted-foreground text-sm">Backtest a system with next-day-open fills and costs</span>
      </header>
      <BacktestPanel />
    </div>
  );
}
