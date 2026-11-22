import { type KeyboardEvent, useRef } from "react";
import type { System, Timeframe } from "@/api/client";
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
  onOpenBuilder,
}: {
  search: ScannerSearch;
  onSearchChange: (next: ScannerSearch) => void;
  onOpenReport: (symbol: string, tf: Timeframe) => void;
  onOpenBuilder: (system: System) => void;
}) {
  const tab = tabOf(search);
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);
  // Arrow keys move between the tabs and select (WAI-ARIA tabs, automatic activation).
  const onTabKey = (e: KeyboardEvent, i: number) => {
    const targets: Record<string, number> = {
      ArrowRight: (i + 1) % tabs.length,
      ArrowLeft: (i - 1 + tabs.length) % tabs.length,
      Home: 0,
      End: tabs.length - 1,
    };
    const to = targets[e.key];
    const next = to === undefined ? undefined : tabs[to];
    if (to === undefined || !next) return;
    e.preventDefault();
    tabRefs.current[to]?.focus();
    onSearchChange({ ...search, tab: next.value });
  };
  // The Quick Scan draft starts from the search, so remount it when the request in the URL
  // changes under it (Back/Forward), but not for sort or mode changes.
  const quickKey = JSON.stringify([search.pattern, search.params, search.universe, search.tf]);
  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-baseline gap-x-4">
        <h1 className="text-2xl font-bold tracking-tight">Scanner</h1>
        <span className="text-muted-foreground text-sm">{tabs.find((t) => t.value === tab)?.blurb}</span>
      </header>
      <SegmentedTrack role="tablist" aria-label="Scanner" className="self-start">
        {tabs.map((t, i) => (
          <button
            key={t.value}
            ref={(el) => {
              tabRefs.current[i] = el;
            }}
            type="button"
            role="tab"
            id={`scanner-tab-${t.value}`}
            aria-controls="scanner-panel"
            className={segmentClass(tab === t.value)}
            aria-selected={tab === t.value}
            tabIndex={tab === t.value ? 0 : -1}
            onClick={() => onSearchChange({ ...search, tab: t.value })}
            onKeyDown={(e) => onTabKey(e, i)}
          >
            {t.label}
          </button>
        ))}
      </SegmentedTrack>
      <div role="tabpanel" id="scanner-panel" aria-labelledby={`scanner-tab-${tab}`}>
        {tab === "saved" ? (
          <SavedScan search={search} onSearchChange={onSearchChange} onOpenReport={onOpenReport} />
        ) : (
          <QuickScan
            key={quickKey}
            search={search}
            onSearchChange={onSearchChange}
            onOpenReport={onOpenReport}
            onOpenBuilder={onOpenBuilder}
            onSaved={(id) => onSearchChange({ tab: "saved", system: id, mode: search.mode })}
          />
        )}
      </div>
    </div>
  );
}
