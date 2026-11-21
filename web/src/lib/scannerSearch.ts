import { z } from "zod";
import { QuickscanRequestMode, QuickscanRequestTimeframe } from "@/api/client";

/** A param value as the API takes it. */
export type ParamValue = number | boolean | string;
export type ParamValues = Record<string, ParamValue>;

/**
 * The Scanner's search params, so a scan URL can be shared: the Quick Scan request (pattern, params,
 * universe, timeframe `tf`), the `mode` and the table `sort` ("pct_above", "-pct_above" for descending).
 * Bad values are dropped rather than failing the page.
 */
export const scannerSearchSchema = z.object({
  tab: z.enum(["quick"]).optional().catch(undefined),
  pattern: z.string().optional().catch(undefined),
  params: z
    .record(z.string(), z.union([z.number(), z.boolean(), z.string()]))
    .optional()
    .catch(undefined),
  universe: z.string().optional().catch(undefined),
  tf: z.enum(QuickscanRequestTimeframe).optional().catch(undefined),
  mode: z.enum(QuickscanRequestMode).optional().catch(undefined),
  sort: z.string().optional().catch(undefined),
});

export type ScannerSearch = z.infer<typeof scannerSearchSchema>;
