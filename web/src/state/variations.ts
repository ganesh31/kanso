import type { System } from "@/api/client";

/** Above this many distinct parameter sets the honesty panel warns about overfitting (spec §5.8). */
export const VARIATIONS_WARN_ABOVE = 10;

const STORAGE_KEY = "kanso.variations.v1";

/** Draft id → the canonical keys of every parameter set backtested in it. */
type Store = Record<string, string[]>;

/** Sets that could not be written to storage (private window, quota), by draft id. */
const unsaved = new Map<string, string[]>();

/** JSON with every object's keys sorted, so key order never makes two equal values differ. */
function canonical(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value !== null && typeof value === "object") {
    const entries = Object.entries(value)
      .filter(([, v]) => v !== undefined)
      .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0));
    return `{${entries.map(([k, v]) => `${JSON.stringify(k)}:${canonical(v)}`).join(",")}}`;
  }
  return JSON.stringify(value);
}

/** A system's parameter set: everything that changes its results except the name and the stock list. */
function paramsKey(s: System): string {
  return canonical({ timeframe: s.timeframe, entry: s.entry, exits: s.exits, costs: s.costs, portfolio: s.portfolio });
}

function readStore(): Store {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : {};
    return parsed !== null && typeof parsed === "object" && !Array.isArray(parsed) ? (parsed as Store) : {};
  } catch {
    return {};
  }
}

function keysFor(store: Store, draftId: string): Set<string> {
  const saved = Array.isArray(store[draftId]) ? store[draftId] : [];
  return new Set([...saved, ...(unsaved.get(draftId) ?? [])]);
}

/** How many distinct parameter sets have been backtested in this draft. */
export function countVariations(draftId: string): number {
  return keysFor(readStore(), draftId).size;
}

/**
 * Records a backtest of `system` in the draft (a saved system's id or a generated draft id)
 * and returns the draft's distinct parameter sets so far. Re-running identical parameters,
 * renaming the system or changing its stock list doesn't add one.
 */
export function recordVariation(draftId: string, system: System): number {
  const store = readStore();
  const keys = keysFor(store, draftId);
  keys.add(paramsKey(system));
  const list = [...keys];
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({ ...store, [draftId]: list }));
    unsaved.delete(draftId);
  } catch {
    unsaved.set(draftId, list);
  }
  return list.length;
}
