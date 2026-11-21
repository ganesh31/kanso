import type { ReactNode } from "react";
import { ApiError, type ScanRequestMode, type ScanResult } from "@/api/client";
import { Button } from "@/components/ui/button";
import { MatchTable } from "./MatchTable";

const modes: { value: ScanRequestMode; label: string }[] = [
  { value: "fresh", label: "Fresh signals" },
  { value: "all", label: "All qualifying" },
];

/** The "Fresh signals" / "All qualifying" switch shared by both Scanner tabs. */
export function ModeToggle({ mode, onChange }: { mode: ScanRequestMode; onChange: (mode: ScanRequestMode) => void }) {
  return (
    <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Mode">
      {modes.map((m) => (
        <Button
          key={m.value}
          type="button"
          size="sm"
          variant={mode === m.value ? "default" : "outline"}
          aria-pressed={mode === m.value}
          onClick={() => onChange(m.value)}
        >
          {m.label}
        </Button>
      ))}
      <span className="text-muted-foreground text-xs">
        {mode === "fresh"
          ? "Stocks whose rule turned true in the last 5 daily / 1 weekly bars."
          : "Every stock the rule is true for now."}
      </span>
    </div>
  );
}

/** A failed scan, with each invalid field when the API names them. */
export function ScanError({ error }: { error: Error }) {
  return (
    <div role="alert" className="text-destructive">
      <p>{error.message}</p>
      {error instanceof ApiError && error.fields.length > 0 ? (
        <ul className="list-disc pl-5 text-sm">
          {error.fields.map((f) => (
            <li key={f.path}>
              {f.path}: {f.message}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

/** A scan's header (count, as-of date, stale and skipped), any notices, and the match table. */
export function ScanResults({
  result,
  sort,
  onSortChange,
  onOpen,
  children,
}: {
  result: ScanResult;
  sort?: string;
  onSortChange: (sort: string | undefined) => void;
  onOpen: (symbol: string) => void;
  /** Notices and actions shown under the header. */
  children?: ReactNode;
}) {
  return (
    <section className="flex flex-col gap-3">
      <header className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        <h2 className="text-lg font-semibold">
          {result.matches.length} {result.matches.length === 1 ? "match" : "matches"}
        </h2>
        <span className="text-muted-foreground text-sm">{`as of ${result.as_of}`}</span>
        <span className="text-muted-foreground text-sm">
          {`${result.stale.length} stale, ${result.skipped.length} skipped`}
        </span>
      </header>
      {children}
      {result.matches.length > 0 ? (
        <MatchTable matches={result.matches} sort={sort} onSortChange={onSortChange} onOpen={onOpen} />
      ) : (
        <p className="text-muted-foreground">No stock qualifies.</p>
      )}
    </section>
  );
}
