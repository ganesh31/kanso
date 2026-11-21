import { useScan } from "@/api/client";
import { Card, CardContent } from "@/components/ui/card";
import type { ScannerSearch } from "@/lib/scannerSearch";
import { useSystems } from "@/state/systems";
import { selectClass } from "./PatternPicker";
import { ModeToggle, ScanError, ScanResults } from "./ScanResults";

/**
 * The Scanner's "Saved system" tab: pick a saved system and see the stocks in its list that its
 * entry rule picks today. The system, mode and sort live in the search params.
 */
export function SavedScan({
  search,
  onSearchChange,
  onOpenReport,
}: {
  search: ScannerSearch;
  onSearchChange: (next: ScannerSearch) => void;
  onOpenReport: (symbol: string) => void;
}) {
  const { systems, isLoading, error } = useSystems();
  const system = systems.find((s) => s.id === search.system);
  const mode = search.mode ?? "fresh";
  const scan = useScan(
    { system: system ?? { name: "", universe: "", entry: { all: [] } }, mode },
    { query: { enabled: system !== undefined } },
  );

  if (isLoading) return <p className="text-muted-foreground">Loading saved systems…</p>;
  if (error) return <ScanError error={error} />;
  if (systems.length === 0) {
    return (
      <p className="text-muted-foreground">
        No saved systems yet. Run a Quick Scan and save it as a system, then pick it here.
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardContent className="flex flex-wrap items-end gap-4">
          <label className="flex flex-col gap-1 text-sm">
            <span>System</span>
            <select
              aria-label="System"
              className={selectClass}
              value={system?.id ?? ""}
              onChange={(e) => onSearchChange({ ...search, tab: "saved", system: e.target.value || undefined })}
            >
              <option value="">Pick a system…</option>
              {systems.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
          {system ? (
            <span className="text-muted-foreground pb-2 text-sm">
              {`${system.universe} · ${system.timeframe ?? "daily"} · ${system.entry.all
                .concat(system.entry.any ?? [])
                .map((c) => c.type)
                .join(", ")}`}
            </span>
          ) : null}
        </CardContent>
      </Card>

      <ModeToggle mode={mode} onChange={(m) => onSearchChange({ ...search, mode: m === "fresh" ? undefined : m })} />

      {search.system && !system ? (
        <p className="text-muted-foreground">That system is no longer saved. Pick another.</p>
      ) : !system ? (
        <p className="text-muted-foreground">Pick a saved system to scan its stock list.</p>
      ) : scan.isPending ? (
        <p className="text-muted-foreground">Scanning {system.universe}…</p>
      ) : scan.isError ? (
        <ScanError error={scan.error} />
      ) : (
        <ScanResults
          result={scan.data}
          sort={search.sort}
          onSortChange={(sort) => onSearchChange({ ...search, sort })}
          onOpen={onOpenReport}
        />
      )}
    </div>
  );
}
