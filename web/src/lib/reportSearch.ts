import { z } from "zod";
import { ReportRequestTimeframe } from "@/api/client";

/**
 * The Stock Report's search params, so a report URL can be shared: `tf` is the timeframe and
 * `as_of` the report date. Bad values are dropped rather than failing the page.
 */
export const reportSearchSchema = z.object({
  tf: z.enum(ReportRequestTimeframe).optional().catch(undefined),
  as_of: z
    .string()
    .regex(/^\d{4}-\d{2}-\d{2}$/)
    .optional()
    .catch(undefined),
});

export type ReportSearch = z.infer<typeof reportSearchSchema>;
