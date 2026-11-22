import { createFileRoute } from "@tanstack/react-router";
import { SystemBuilder } from "@/components/SystemBuilder";
import { builderSearchSchema } from "@/lib/builderSearch";

export const Route = createFileRoute("/builder")({
  validateSearch: builderSearchSchema,
  component: BuilderPage,
});

function BuilderPage() {
  const { draft } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-3xl font-bold tracking-tight">System Builder</h1>
        <p className="text-muted-foreground max-w-2xl">
          Build a breakout rule and test it on every stock since 2007. Trades fill at the next day&apos;s open, after
          costs and slippage. The address bar keeps your draft, so you can share it as a link.
        </p>
      </header>
      <SystemBuilder
        draftParam={draft}
        // Edits replace the history entry, so Back leaves the Builder rather than undoing a keystroke.
        onDraftParamChange={(next) => void navigate({ search: { draft: next }, replace: true })}
        onOpenReport={(symbol, tf) =>
          void navigate({ to: "/report/$symbol", params: { symbol }, search: { tf: tf === "daily" ? undefined : tf } })
        }
      />
    </div>
  );
}
