import { z } from "zod";

/**
 * The System Builder's search params: `draft` is the working system as base64url JSON
 * (see `encodeDraft`), so a half-built rule can be shared as a link.
 */
export const builderSearchSchema = z.object({
  draft: z.string().optional().catch(undefined),
});

export type BuilderSearch = z.infer<typeof builderSearchSchema>;
