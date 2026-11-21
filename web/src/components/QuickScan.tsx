import { useState } from "react";
import {
  ApiError,
  type Meta,
  type QuickscanRequest,
  type QuickscanRequestMode,
  type Timeframe,
  useListPatterns,
  useListUniverses,
  useQuickScan,
} from "@/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import type { ParamValues, ScannerSearch } from "@/lib/scannerSearch";
import { MatchTable } from "./MatchTable";
import { ParamEditor, withDefaults } from "./ParamEditor";
import { PatternPicker, selectClass } from "./PatternPicker";

const modes: { value: QuickscanRequestMode; label: string }[] = [
  { value: "fresh", label: "Fresh signals" },
  { value: "all", label: "All qualifying" },
];

const timeframes: { value: Timeframe; label: string }[] = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
];

/** The request the search params describe, or undefined until a scan has been asked for. */
function requestOf(search: ScannerSearch): QuickscanRequest | undefined {
  if (!search.pattern || !search.universe) return undefined;
  const req: QuickscanRequest = {
    pattern: search.pattern,
    universe: search.universe,
    timeframe: search.tf ?? "daily",
    mode: search.mode ?? "fresh",
  };
  if (search.params) req.params = search.params;
  return req;
}

/** "3 years": a pattern's defining default, for the auto-insert notice. */
function describeDefaults(meta: Meta): string {
  const p = meta.params.find((s) => s.kind === "number");
  if (!p) return "default params";
  return p.unit ? `${String(p.default)} ${p.unit}` : String(p.default);
}

/**
 * Quick Scan: pick one pattern or filter, edit its params, pick a stock list and timeframe, and see
 * every stock that qualifies. The submitted request and the table state live in the search params,
 * so a scan can be shared as a link.
 */
export function QuickScan({
  search,
  onSearchChange,
  onOpenReport,
}: {
  search: ScannerSearch;
  onSearchChange: (next: ScannerSearch) => void;
  onOpenReport: (symbol: string) => void;
}) {
  const patternsQ = useListPatterns({ query: { select: (b) => b.patterns } });
  const universesQ = useListUniverses({ query: { select: (b) => b.universes } });
  const patterns = patternsQ.data ?? [];
  const universes = universesQ.data ?? [];

  // The form's draft; it becomes the search (and so the request) on Scan.
  const [draft, setDraft] = useState<{ pattern?: string; params: ParamValues; universe?: string; tf?: Timeframe }>(
    () => ({ pattern: search.pattern, params: search.params ?? {}, universe: search.universe, tf: search.tf }),
  );
  const pattern = draft.pattern ?? patterns[0]?.type ?? "";
  const meta = patterns.find((p) => p.type === pattern);
  const params = withDefaults(meta?.params ?? [], draft.params);
  const universe = draft.universe ?? universes[0]?.name ?? "";
  const tf = draft.tf ?? "daily";

  const request = requestOf(search);
  const scan = useQuickScan(request ?? { pattern: "", universe: "" }, { query: { enabled: request !== undefined } });
  const mode = search.mode ?? "fresh";

  const submit = () =>
    onSearchChange({ ...search, tab: "quick", pattern, params, universe, tf: tf === "daily" ? undefined : tf });

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-wrap items-end gap-4">
            <PatternPicker
              patterns={patterns}
              value={pattern}
              onChange={(type) => setDraft((d) => ({ ...d, pattern: type, params: {} }))}
            />
            <label className="flex flex-col gap-1 text-sm">
              <span>Stock list</span>
              <select
                aria-label="Stock list"
                className={selectClass}
                value={universe}
                onChange={(e) => setDraft((d) => ({ ...d, universe: e.target.value }))}
              >
                {universes.map((u) => (
                  <option key={u.name} value={u.name}>
                    {u.name}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Timeframe</span>
              <select
                aria-label="Timeframe"
                className={selectClass}
                value={tf}
                onChange={(e) => setDraft((d) => ({ ...d, tf: e.target.value as Timeframe }))}
              >
                {timeframes.map((t) => (
                  <option key={t.value} value={t.value}>
                    {t.label}
                  </option>
                ))}
              </select>
            </label>
          </div>
          {meta ? (
            <ParamEditor
              specs={meta.params}
              params={params}
              onChange={(next) => setDraft((d) => ({ ...d, params: next }))}
            />
          ) : null}
          <div>
            <Button type="button" onClick={submit} disabled={!meta || !universe}>
              Scan
            </Button>
          </div>
        </CardContent>
      </Card>

      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="Mode">
        {modes.map((m) => (
          <Button
            key={m.value}
            type="button"
            size="sm"
            variant={mode === m.value ? "default" : "outline"}
            aria-pressed={mode === m.value}
            onClick={() => onSearchChange({ ...search, mode: m.value === "fresh" ? undefined : m.value })}
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

      {request === undefined ? (
        <p className="text-muted-foreground">Pick a pattern and a stock list, then Scan.</p>
      ) : scan.isPending ? (
        <p className="text-muted-foreground">Scanning {request.universe}…</p>
      ) : scan.isError ? (
        <div role="alert" className="text-destructive">
          <p>{scan.error.message}</p>
          {scan.error instanceof ApiError && scan.error.fields.length > 0 ? (
            <ul className="list-disc pl-5 text-sm">
              {scan.error.fields.map((f) => (
                <li key={f.path}>
                  {f.path}: {f.message}
                </li>
              ))}
            </ul>
          ) : null}
        </div>
      ) : (
        <section className="flex flex-col gap-3">
          <header className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
            <h2 className="text-lg font-semibold">
              {scan.data.matches.length} {scan.data.matches.length === 1 ? "match" : "matches"}
            </h2>
            <span className="text-muted-foreground text-sm">{`as of ${scan.data.as_of}`}</span>
            <span className="text-muted-foreground text-sm">
              {`${scan.data.stale.length} stale, ${scan.data.skipped.length} skipped`}
            </span>
          </header>
          {scan.data.auto_inserted.map((type) => {
            const inserted = patterns.find((p) => p.type === type);
            const asked = patterns.find((p) => p.type === request.pattern);
            return (
              <p key={type} className="bg-muted rounded-md px-3 py-2 text-sm">
                {`Added ${inserted?.label ?? type} (${inserted ? describeDefaults(inserted) : "default params"}) ` +
                  `because ${asked?.label ?? request.pattern} needs a breakout level.`}
              </p>
            );
          })}
          {scan.data.matches.length > 0 ? (
            <MatchTable
              matches={scan.data.matches}
              sort={search.sort}
              onSortChange={(sort) => onSearchChange({ ...search, sort })}
              onOpen={onOpenReport}
            />
          ) : (
            <p className="text-muted-foreground">No stock qualifies.</p>
          )}
        </section>
      )}
    </div>
  );
}
