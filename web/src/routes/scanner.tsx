import { createFileRoute } from "@tanstack/react-router";
import { Scanner } from "@/components/Scanner";
import { scannerSearchSchema } from "@/lib/scannerSearch";
import { encodeDraft, fromSystem } from "@/state/draft";

export const Route = createFileRoute("/scanner")({
  validateSearch: scannerSearchSchema,
  component: ScannerPage,
});

function ScannerPage() {
  const search = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <Scanner
      search={search}
      onSearchChange={(next) => void navigate({ search: next })}
      onOpenReport={(symbol, tf) =>
        void navigate({ to: "/report/$symbol", params: { symbol }, search: { tf: tf === "daily" ? undefined : tf } })
      }
      onOpenBuilder={(system) => void navigate({ to: "/builder", search: { draft: encodeDraft(fromSystem(system)) } })}
    />
  );
}
