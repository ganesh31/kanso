import { SegmentedTrack, segmentClass } from "@/components/ui/segmented";
import type { ScannerSearch } from "@/lib/scannerSearch";
import { QuickScan } from "./QuickScan";
import { SavedScan } from "./SavedScan";

type Tab = NonNullable<ScannerSearch["tab"]>;

const tabs: { value: Tab; label: string; blurb: string }[] = [
  { value: "saved", label: "Saved system", blurb: "Run a saved system across its stock list" },
  { value: "quick", label: "Quick Scan", blurb: "One pattern or filter, no saved system needed" },
];

/** The tab the search shows: as given, else Quick Scan for a pre-tab Quick Scan link, else Saved system. */
export function tabOf(search: ScannerSearch): Tab {
  return search.tab ?? (search.pattern ? "quick" : "saved");
}

/**
 * The Scanner screen: a "Saved system" tab and a "Quick Scan" tab. Everything that defines a scan
 * (tab, system, request, mode, sort) lives in the search params so it can be shared.
 */
export function Scanner({
  search,
  onSearchChange,
  onOpenReport,
}: {
  search: ScannerSearch;
  onSearchChange: (next: ScannerSearch) => void;
  onOpenReport: (symbol: string) => void;
}) {
  const tab = tabOf(search);
  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-baseline gap-x-4">
        <h1 className="text-2xl font-bold tracking-tight">Scanner</h1>
        <span className="text-muted-foreground text-sm">{tabs.find((t) => t.value === tab)?.blurb}</span>
      </header>
      <SegmentedTrack role="tablist" aria-label="Scanner" className="self-start">
        {tabs.map((t) => (
          <button
            key={t.value}
            type="button"
            role="tab"
            className={segmentClass(tab === t.value)}
            aria-selected={tab === t.value}
            onClick={() => onSearchChange({ ...search, tab: t.value })}
          >
            {t.label}
          </button>
        ))}
      </SegmentedTrack>
      <div role="tabpanel">
        {tab === "saved" ? (
          <SavedScan search={search} onSearchChange={onSearchChange} onOpenReport={onOpenReport} />
        ) : (
          <QuickScan
            search={search}
            onSearchChange={onSearchChange}
            onOpenReport={onOpenReport}
            onSaved={(id) => onSearchChange({ tab: "saved", system: id, mode: search.mode })}
          />
        )}
      </div>
    </div>
  );
}
