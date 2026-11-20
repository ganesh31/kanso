import { useQuery } from "@tanstack/react-query";
import { api, type ReportOptions } from "./client";

/** Stable query keys: one place, so invalidation never drifts from fetching. */
export const queryKeys = {
  patterns: ["patterns"] as const,
  report: (symbol: string, opts: ReportOptions) => ["report", symbol, opts] as const,
  bars: (symbol: string) => ["bars", symbol] as const,
};

export function usePatterns() {
  return useQuery({ queryKey: queryKeys.patterns, queryFn: () => api.patterns() });
}

export function useReport(symbol: string, opts: ReportOptions = {}) {
  return useQuery({
    queryKey: queryKeys.report(symbol, opts),
    queryFn: () => api.report(symbol, opts),
  });
}

export function useBars(symbol: string) {
  return useQuery({
    queryKey: queryKeys.bars(symbol),
    queryFn: async () => (await api.bars(symbol)).bars,
  });
}
