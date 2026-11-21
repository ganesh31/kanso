import { defineConfig } from "orval";

// Generates typed fetch functions and TanStack Query hooks from the huma spec.
// Regenerate with `make api`; CI fails if src/api/generated.ts is stale.
export default defineConfig({
  kanso: {
    input: "../api/openapi.json",
    output: {
      target: "src/api/generated.ts",
      mode: "single",
      client: "react-query",
      httpClient: "fetch",
      clean: false,
      urlEncodeParameters: true,
      override: {
        mutator: { path: "src/api/fetcher.ts", name: "apiFetch" },
        fetch: { includeHttpResponseReturnType: false },
        operations: {
          // A report is a read even though it is a POST (the body carries options).
          "stock-report": { query: { useQuery: true, useMutation: false } },
          // A quick scan is a read too; its request lives in the URL, so it is a cached query.
          "quick-scan": { query: { useQuery: true, useMutation: false } },
        },
      },
    },
  },
});
