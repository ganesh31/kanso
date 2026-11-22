import { useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import {
  type ApiError,
  getListSystemsQueryKey,
  type System,
  useCreateSystem,
  useDeleteSystem,
  useListSystems,
  useUpdateSystem,
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
  // Each write refreshes the list before it resolves, so a caller sees its change on return.
  const refresh = { onSuccess: () => queryClient.invalidateQueries({ queryKey: getListSystemsQueryKey() }) };
  const { mutateAsync: create } = useCreateSystem({ mutation: refresh });
  const { mutateAsync: update } = useUpdateSystem({ mutation: refresh });
  const { mutateAsync: del } = useDeleteSystem({ mutation: refresh });

  const save = useCallback(
    (system: System) => (system.id ? update({ id: system.id, data: system }) : create({ data: system })),
    [create, update],
  );
  const remove = useCallback(
    async (id: string) => {
      await del({ id });
    },
    [del],
  );

  return { systems: list.data ?? [], isLoading: list.isLoading, error: list.error, save, remove };
}
