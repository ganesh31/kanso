import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useSyncExternalStore } from "react";
import {
  ApiError,
  getListSystemsQueryKey,
  type System,
  useCreateSystem,
  useDeleteSystem,
  useListSystems,
  useUpdateSystem,
  validateSystem,
} from "@/api/client";
import { readItem, writeItem } from "@/lib/storage";

export interface SavedSystems {
  /** Every saved system, sorted by name; empty while loading. */
  systems: System[];
  isLoading: boolean;
  error: ApiError | null;
  /** Saves a new system (no id) or replaces the one with its id; resolves to the stored system. */
  save: (system: System) => Promise<System>;
  remove: (id: string) => Promise<void>;
}

/**
 * Browser-kept systems. The key exists once the server has refused a write (the public demo
 * stores nothing, spec §6.4), and from then on this browser keeps its systems itself.
 */
const LOCAL_KEY = "kanso.systems.v1";

function parseLocal(raw: string | null): System[] | null {
  if (raw === null) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as System[]) : [];
  } catch {
    return [];
  }
}

/** Every mounted useSystems, told when the browser's systems change. */
const listeners = new Set<() => void>();
let cached: { raw: string | null; value: System[] | null } | undefined;

/** The browser's systems, or null when they live on the server; the same array until they change. */
function readLocal(): System[] | null {
  const raw = readItem(LOCAL_KEY);
  if (cached?.raw !== raw) cached = { raw, value: parseLocal(raw) };
  return cached.value;
}

function subscribeLocal(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function writeLocal(systems: System[]): void {
  const sorted = [...systems].sort((a, b) => a.name.localeCompare(b.name) || (a.id ?? "").localeCompare(b.id ?? ""));
  writeItem(LOCAL_KEY, JSON.stringify(sorted));
  for (const listener of listeners) listener();
}

function isForbidden(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403;
}

/**
 * Saved systems, backed by the API (`/api/systems`) through TanStack Query. When the server
 * refuses a write with a 403 (the read-only demo), they switch to browser storage and stay
 * there after a reload.
 */
export function useSystems(): SavedSystems {
  const queryClient = useQueryClient();
  const local = useSyncExternalStore(subscribeLocal, readLocal);
  const list = useListSystems({ query: { select: (b) => b.systems, enabled: local === null } });
  // Each write refreshes the list before it resolves, so a caller sees its change on return.
  const refresh = { onSuccess: () => queryClient.invalidateQueries({ queryKey: getListSystemsQueryKey() }) };
  const { mutateAsync: create } = useCreateSystem({ mutation: refresh });
  const { mutateAsync: update } = useUpdateSystem({ mutation: refresh });
  const { mutateAsync: del } = useDeleteSystem({ mutation: refresh });

  const saveLocal = useCallback(async (system: System) => {
    // The server still fills defaults and validates, so a reload shows exactly what runs.
    const { system: normalized } = await validateSystem(system);
    const stored: System = { ...normalized, id: system.id || crypto.randomUUID() };
    const others = (readLocal() ?? []).filter((s) => s.id !== stored.id);
    writeLocal([...others, stored]);
    return stored;
  }, []);

  const removeLocal = useCallback((id: string) => {
    writeLocal((readLocal() ?? []).filter((s) => s.id !== id));
  }, []);

  /** Switches this browser to keeping its own systems. */
  const goLocal = useCallback(() => {
    if (readLocal() === null) writeLocal([]);
  }, []);

  const save = useCallback(
    async (system: System) => {
      if (readLocal() !== null) return saveLocal(system);
      try {
        return await (system.id ? update({ id: system.id, data: system }) : create({ data: system }));
      } catch (err) {
        if (!isForbidden(err)) throw err;
        goLocal();
        return saveLocal(system);
      }
    },
    [create, update, saveLocal, goLocal],
  );

  const remove = useCallback(
    async (id: string) => {
      if (readLocal() !== null) return removeLocal(id);
      try {
        await del({ id });
      } catch (err) {
        if (!isForbidden(err)) throw err;
        goLocal();
        removeLocal(id);
      }
    },
    [del, removeLocal, goLocal],
  );

  if (local !== null) return { systems: local, isLoading: false, error: null, save, remove };
  return { systems: list.data ?? [], isLoading: list.isLoading, error: list.error, save, remove };
}
