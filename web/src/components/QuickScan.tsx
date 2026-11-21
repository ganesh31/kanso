import { useState } from "react";
import {
  type Condition,
  type Meta,
  type QuickscanRequest,
  type System,
  type Timeframe,
  useListPatterns,
  useListUniverses,
  useQuickScan,
} from "@/api/client";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import type { ParamValues, ScannerSearch } from "@/lib/scannerSearch";
import { useSystems } from "@/state/systems";
import { ParamEditor, withDefaults } from "./ParamEditor";
import { PatternPicker, selectClass } from "./PatternPicker";
import { ModeToggle, ScanError, ScanResults } from "./ScanResults";

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

/**
 * The system a quick scan stands for: its stock list and timeframe, and an entry rule of the
 * auto-inserted conditions (default params) followed by the pattern.
 */
export function systemOf(name: string, request: QuickscanRequest, autoInserted: string[]): System {
  const pattern: Condition = { type: request.pattern };
  if (request.params) pattern.params = request.params;
  return {
    name,
    universe: request.universe,
    timeframe: request.timeframe ?? "daily",
    entry: { all: [...autoInserted.map((type) => ({ type })), pattern] },
  };
}

/** Saves the scanned rule as a system, so it can be picked on the Saved system tab. */
function SaveAsSystem({
  request,
  autoInserted,
  onSaved,
}: {
  request: QuickscanRequest;
  autoInserted: string[];
  onSaved: (id: string) => void;
}) {
  const { save } = useSystems();
  const [name, setName] = useState("");
  const [state, setState] = useState<{ saving: boolean; error?: Error }>({ saving: false });
  const submit = async () => {
    setState({ saving: true });
    try {
      const saved = await save(systemOf(name.trim(), request, autoInserted));
      setState({ saving: false });
      if (saved.id) onSaved(saved.id);
    } catch (error) {
      setState({ saving: false, error: error as Error });
    }
  };
  return (
    <div className="flex flex-col gap-2 border-t pt-4">
      <p className="text-muted-foreground text-sm">Keep this scan to rerun it from the Saved system tab.</p>
      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        <label className="flex flex-col gap-1 text-sm">
          <span>System name</span>
          <Input aria-label="System name" value={name} onChange={(e) => setName(e.target.value)} className="w-56" />
        </label>
        <Button type="submit" variant="outline" disabled={name.trim() === "" || state.saving}>
          Save as system
        </Button>
      </form>
      {state.error ? <ScanError error={state.error} /> : null}
    </div>
  );
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
  onSaved,
}: {
  search: ScannerSearch;
  onSearchChange: (next: ScannerSearch) => void;
  onOpenReport: (symbol: string) => void;
  /** Called with the new system's id after "Save as system". */
  onSaved: (id: string) => void;
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
          <div className="flex flex-wrap items-start gap-4">
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

      <ModeToggle mode={mode} onChange={(m) => onSearchChange({ ...search, mode: m === "fresh" ? undefined : m })} />

      {request === undefined ? (
        <p className="text-muted-foreground">Pick a pattern and a stock list, then Scan.</p>
      ) : scan.isPending ? (
        <p className="text-muted-foreground">Scanning {request.universe}…</p>
      ) : scan.isError ? (
        <ScanError error={scan.error} />
      ) : (
        <ScanResults
          result={scan.data}
          sort={search.sort}
          onSortChange={(sort) => onSearchChange({ ...search, sort })}
          onOpen={onOpenReport}
          footer={<SaveAsSystem request={request} autoInserted={scan.data.auto_inserted} onSaved={onSaved} />}
        >
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
        </ScanResults>
      )}
    </div>
  );
}
