import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";
import { StockReport } from "@/components/StockReport";

const searchSchema = z.object({
  as_of: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/)
    .optional()
    .catch(undefined),
});

export const Route = createFileRoute("/report/$symbol")({
  validateSearch: searchSchema,
  component: ReportPage,
});

function ReportPage() {
  const { symbol } = Route.useParams();
  const { as_of } = Route.useSearch();
  return <StockReport symbol={symbol} asOf={as_of} />;
}
