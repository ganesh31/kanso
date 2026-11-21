import { createFileRoute } from "@tanstack/react-router";
import { QuickScan } from "@/components/QuickScan";
import { scannerSearchSchema } from "@/lib/scannerSearch";

export const Route = createFileRoute("/scanner")({
  validateSearch: scannerSearchSchema,
  component: ScannerPage,
});

function ScannerPage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-baseline gap-x-4">
        <h1 className="text-2xl font-semibold tracking-tight">Scanner</h1>
        <span className="text-muted-foreground text-sm">Quick Scan: one pattern or filter, no saved system needed</span>
      </header>
      <QuickScan
        search={search}
        onSearchChange={(next) => void navigate({ search: next })}
        onOpenReport={(symbol) => void navigate({ to: "/report/$symbol", params: { symbol } })}
      />
    </div>
  );
}
