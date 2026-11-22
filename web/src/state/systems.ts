import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import {
  type ApiError,
  createSystem,
  deleteSystem,
  getListSystemsQueryKey,
  type System,
  updateSystem,
  useListSystems,
} from "@/api/client";

export interface SavedSystems {
  /** Every saved system, sorted by name; empty while loading. */
  systems: System[];
  isLoading: boolean;
  error: ApiError | null;
  /** Saves a new system (no id) or replaces the one with its id; resolves to the stored system. */
  save: (system: System) => Promise<System>;
  remove: (id: string) => Promise<void>;
}

/** Saved systems, backed by the API (`/api/systems`) through TanStack Query. */
export function useSystems(): SavedSystems {
  const queryClient = useQueryClient();
  const list = useListSystems({ query: { select: (b) => b.systems } });

  const refresh = useCallback(
    () => queryClient.invalidateQueries({ queryKey: getListSystemsQueryKey() }),
    [queryClient],
  );
  const save = useCallback(
    async (system: System) => {
      const saved = system.id ? await updateSystem(system.id, system) : await createSystem(system);
      await refresh();
      return saved;
    },
    [refresh],
  );
  const remove = useCallback(
    async (id: string) => {
      await deleteSystem(id);
      await refresh();
    },
    [refresh],
  );

  return { systems: list.data ?? [], isLoading: list.isLoading, error: list.error, save, remove };
}
