import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useState } from "react";
import {
  ApiError,
  createSystem,
  deleteSystem,
  getListSystemsQueryKey,
  type System,
  updateSystem,
  useListSystems,
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

/** The browser's systems, or null when they live on the server. */
function readLocal(): System[] | null {
  const raw = readItem(LOCAL_KEY);
  if (raw === null) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as System[]) : [];
  } catch {
    return [];
  }
}

function writeLocal(systems: System[]): System[] {
  const sorted = [...systems].sort((a, b) => a.name.localeCompare(b.name) || (a.id ?? "").localeCompare(b.id ?? ""));
  writeItem(LOCAL_KEY, JSON.stringify(sorted));
  return sorted;
}

function isForbidden(err: unknown): boolean {
  return err instanceof ApiError && err.status === 403;
}

/**
 * Saved systems, backed by the API (`/api/systems`) through TanStack Query. When the server
 * refuses a write with a 403, they switch to browser storage and stay there after a reload.
 */
export function useSystems(): SavedSystems {
  const queryClient = useQueryClient();
  const [local, setLocal] = useState<System[] | null>(readLocal);
  const list = useListSystems({ query: { select: (b) => b.systems, enabled: local === null } });

  const refresh = useCallback(
    () => queryClient.invalidateQueries({ queryKey: getListSystemsQueryKey() }),
    [queryClient],
  );

  const saveLocal = useCallback(async (system: System) => {
    // The server still fills defaults and validates, so a reload shows exactly what runs.
    const { system: normalized } = await validateSystem(system);
    const stored: System = { ...normalized, id: system.id || crypto.randomUUID() };
    const others = (readLocal() ?? []).filter((s) => s.id !== stored.id);
    setLocal(writeLocal([...others, stored]));
    return stored;
  }, []);

  const removeLocal = useCallback((id: string) => {
    setLocal(writeLocal((readLocal() ?? []).filter((s) => s.id !== id)));
  }, []);

  /** Switches this browser to keeping its own systems. */
  const goLocal = useCallback(() => {
    if (readLocal() === null) setLocal(writeLocal([]));
  }, []);

  const save = useCallback(
    async (system: System) => {
      if (readLocal() !== null) return saveLocal(system);
      try {
        const saved = system.id ? await updateSystem(system.id, system) : await createSystem(system);
        await refresh();
        return saved;
      } catch (err) {
        if (!isForbidden(err)) throw err;
        goLocal();
        return saveLocal(system);
      }
    },
    [refresh, saveLocal, goLocal],
  );

  const remove = useCallback(
    async (id: string) => {
      if (readLocal() !== null) return removeLocal(id);
      try {
        await deleteSystem(id);
        await refresh();
      } catch (err) {
        if (!isForbidden(err)) throw err;
        goLocal();
        removeLocal(id);
      }
    },
    [refresh, removeLocal, goLocal],
  );

  if (local !== null) return { systems: local, isLoading: false, error: null, save, remove };
  return { systems: list.data ?? [], isLoading: list.isLoading, error: list.error, save, remove };
}
