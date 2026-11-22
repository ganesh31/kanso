import { createFileRoute } from "@tanstack/react-router";
import { StockReport } from "@/components/StockReport";
import { reportSearchSchema } from "@/lib/reportSearch";

export const Route = createFileRoute("/report/$symbol")({
  validateSearch: reportSearchSchema,
  component: ReportPage,
});

function ReportPage() {
  const { symbol } = Route.useParams();
  const { as_of, tf } = Route.useSearch();
  const navigate = Route.useNavigate();
  return (
    <StockReport
      symbol={symbol}
      asOf={as_of}
      timeframe={tf}
      onTimeframeChange={(next) =>
        navigate({ search: (prev) => ({ ...prev, tf: next === "daily" ? undefined : next }) })
      }
    />
  );
}
