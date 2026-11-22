import { type FormEvent, useEffect, useMemo, useReducer, useRef, useState } from "react";
import {
  ApiError,
  type BacktestResult,
  type ScanRequestMode,
  type System,
  type Timeframe,
  useBacktest,
  useGetDefaultSystem,
  useListPatterns,
  useListUniverses,
  useScan,
} from "@/api/client";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SegmentedTrack, segmentClass } from "@/components/ui/segmented";
import {
  type Draft,
  decodeDraft,
  encodeDraft,
  errorsByBlock,
  fromSystem,
  layoutKey,
  systemReducer,
  newDraftId,
  toSystem,
  variationsKey,
} from "@/state/draft";
import { recordVariation } from "@/state/variations";
import { useSystems } from "@/state/systems";
import { AnyGroup, ConditionGroup } from "./AnyGroup";
import { BacktestResults } from "./BacktestResults";
import { BlockErrors } from "./ConditionBlock";
import { ExitBlock } from "./ExitBlock";
import { selectClass } from "./PatternPicker";
import { ModeToggle, ScanError, ScanResults } from "./ScanResults";

/** How long the draft sits still before it is written to the URL. */
const URL_DEBOUNCE_MS = 300;

const timeframes: { value: Timeframe; label: string }[] = [
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
];

/**
 * A draft without exits, costs or a portfolio (a Quick Scan's) takes the default system's, and an
 * unsaved draft without an id gets one to count its variations under.
 */
function prepare(d: Draft, def: System): Draft {
  const out = {
    ...d,
    exits: d.exits ?? def.exits,
    costs: d.costs ?? def.costs,
    portfolio: d.portfolio ?? def.portfolio,
  };
  if (!out.id && !out.draftId) out.draftId = newDraftId();
  return out;
}

/** A failed request, with the layout of the draft it was sent for (see `layoutKey`). */
interface Failure {
  error: Error;
  layout: string;
}

/**
 * The System Builder: compose a rule without code (required conditions, one "Match ANY of these"
 * group, exits, costs, portfolio, stock list and timeframe), then backtest or scan it. It opens on
 * the `?draft=` link when there is one, else on the default system, and writes every edit back to
 * the link. The server is the only validator: its field errors are shown beside their blocks.
 */
export function SystemBuilder({
  draftParam,
  onDraftParamChange,
  onOpenReport,
}: {
  draftParam: string | undefined;
  /** Called (debounced) with the edited draft as base64url JSON. */
  onDraftParamChange: (draft: string) => void;
  onOpenReport: (symbol: string, tf: Timeframe) => void;
}) {
  const def = useGetDefaultSystem({ query: { select: (b) => b.system } });
  // Only the link the page opened on seeds the draft; later values are the draft's own echoes.
  const [linked] = useState(() => (draftParam ? decodeDraft(draftParam) : undefined));
  const initial = useMemo(
    () => (def.data ? prepare(linked ?? fromSystem(def.data), def.data) : undefined),
    [linked, def.data],
  );

  if (def.isPending) return <p className="text-muted-foreground">Loading the default system…</p>;
  if (def.isError)
    return (
      <p role="alert" className="text-destructive">
        {`Couldn't load the default system: ${def.error.message}`}
      </p>
    );
  if (!initial) return null;
  return (
    <Editor
      initial={initial}
      draftParam={draftParam}
      onDraftParamChange={onDraftParamChange}
      onOpenReport={onOpenReport}
    />
  );
}

function Editor({
  initial,
  draftParam,
  onDraftParamChange,
  onOpenReport,
}: {
  initial: Draft;
  draftParam: string | undefined;
  onDraftParamChange: (draft: string) => void;
  onOpenReport: (symbol: string, tf: Timeframe) => void;
}) {
  const [draft, dispatch] = useReducer(systemReducer, initial);
  const patterns = useListPatterns({ query: { select: (b) => b.patterns } }).data ?? [];
  const universes = useListUniverses({ query: { select: (b) => b.universes } }).data ?? [];
  const saved = useSystems();
  const formRef = useRef<HTMLFormElement>(null);

  // Mirror the draft to ?draft=, once it stops changing.
  const write = useRef(onDraftParamChange);
  write.current = onDraftParamChange;
  useEffect(() => {
    if (draft === initial) return;
    const encoded = encodeDraft(draft);
    if (encoded === draftParam) return;
    const t = setTimeout(() => write.current(encoded), URL_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [draft, initial, draftParam]);

  const run = useBacktest();
  const [scanned, setScanned] = useState<{ system: System; layout: string }>();
  const [mode, setMode] = useState<ScanRequestMode>("fresh");
  const scan = useScan(
    { system: scanned?.system ?? toSystem(draft), mode },
    { query: { enabled: scanned !== undefined } },
  );
  const [shown, setShown] = useState<"backtest" | "scan">();
  const [failure, setFailure] = useState<Failure>();
  const [variations, setVariations] = useState(0);
  const [saveState, setSaveState] = useState<"saving" | "saved">();
  const [pickedSaved, setPickedSaved] = useState("");

  const layout = layoutKey(draft);
  const current: Failure | undefined =
    shown === "scan" && scan.isError && scanned ? { error: scan.error, layout: scanned.layout } : failure;
  const fields = current && current.layout === layout && current.error instanceof ApiError ? current.error.fields : [];
  const errors = errorsByBlock(draft, fields);

  const backtest = () => {
    setShown("backtest");
    setFailure(undefined);
    const system = toSystem(draft);
    const key = variationsKey(draft);
    run.mutate(
      { data: system },
      {
        onSuccess: () => setVariations(recordVariation(key, system)),
        onError: (error) => setFailure({ error, layout }),
      },
    );
  };
  const startScan = () => {
    setShown("scan");
    setFailure(undefined);
    setScanned({ system: toSystem(draft), layout });
  };
  const save = async () => {
    if (!formRef.current?.reportValidity()) return;
    setSaveState("saving");
    setFailure(undefined);
    try {
      const stored = await saved.save(toSystem(draft));
      if (stored.id) dispatch({ type: "setId", id: stored.id });
      setSaveState("saved");
    } catch (error) {
      setSaveState(undefined);
      setFailure({ error: error as Error, layout });
    }
  };
  const submit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    // An emptied param makes the form invalid: point at it rather than sending its last value.
    if (!e.currentTarget.reportValidity()) return;
    const submitter = (e.nativeEvent as SubmitEvent).submitter;
    if (submitter instanceof HTMLButtonElement && submitter.value === "scan") startScan();
    else backtest();
  };

  const loadable = saved.systems.find((s) => s.id === pickedSaved);
  const topErrors = errors.get("setup");

  return (
    <div className="flex flex-col gap-8">
      <form ref={formRef} aria-label="System" className="flex flex-col gap-8" onSubmit={submit}>
        <section aria-label="Setup" className="flex flex-col gap-3">
          <div className="flex flex-wrap items-end gap-4">
            <label className="flex flex-col gap-1 text-sm">
              <span>System name</span>
              <Input
                aria-label="System name"
                value={draft.name}
                onChange={(e) => {
                  setSaveState(undefined);
                  dispatch({ type: "setName", name: e.target.value });
                }}
                className="w-64"
              />
            </label>
            <label className="flex flex-col gap-1 text-sm">
              <span>Stock list</span>
              <select
                aria-label="Stock list"
                className={selectClass}
                value={draft.universe}
                onChange={(e) => dispatch({ type: "setUniverse", universe: e.target.value })}
              >
                {universes.some((u) => u.name === draft.universe) ? null : (
                  <option value={draft.universe}>{draft.universe}</option>
                )}
                {universes.map((u) => (
                  <option key={u.name} value={u.name}>
                    {u.name}
                  </option>
                ))}
              </select>
            </label>
            <div className="flex flex-col gap-1 text-sm">
              <span id="timeframe-label">Timeframe</span>
              <SegmentedTrack role="group" aria-labelledby="timeframe-label">
                {timeframes.map((t) => (
                  <button
                    key={t.value}
                    type="button"
                    className={segmentClass(draft.timeframe === t.value)}
                    aria-pressed={draft.timeframe === t.value}
                    onClick={() => dispatch({ type: "setTimeframe", timeframe: t.value })}
                  >
                    {t.label}
                  </button>
                ))}
              </SegmentedTrack>
            </div>
          </div>
          <BlockErrors errors={topErrors} />
        </section>

        <ConditionGroup
          group="all"
          title="Buy when every one of these holds"
          hint={`Checked on each ${draft.timeframe === "weekly" ? "weekly" : "daily"} bar. Switch one off to try the rule without it.`}
          conditions={draft.all}
          patterns={patterns}
          errors={errors}
          dispatch={dispatch}
        />
        <AnyGroup conditions={draft.any} patterns={patterns} errors={errors} dispatch={dispatch} />
        <ExitBlock
          exits={draft.exits}
          costs={draft.costs}
          portfolio={draft.portfolio}
          errors={errors}
          onExits={(exits) => dispatch({ type: "setExits", exits })}
          onCosts={(costs) => dispatch({ type: "setCosts", costs })}
          onPortfolio={(portfolio) => dispatch({ type: "setPortfolio", portfolio })}
        />

        <div className="flex flex-wrap items-center gap-3 border-t pt-4">
          <Button type="submit" value="backtest" disabled={run.isPending}>
            Backtest
          </Button>
          <Button type="submit" value="scan" variant="outline">
            Scan
          </Button>
          <span className="text-muted-foreground text-sm">Backtest since 2007, or scan for today&apos;s signals.</span>
        </div>
      </form>

      <section aria-label="Save and load" className="flex flex-wrap items-end gap-3">
        <Button
          type="button"
          variant="outline"
          disabled={draft.name.trim() === "" || saveState === "saving"}
          onClick={() => void save()}
        >
          Save
        </Button>
        {saveState === "saved" ? <span className="text-muted-foreground pb-2 text-sm">Saved.</span> : null}
        <label className="flex flex-col gap-1 text-sm">
          <span>Saved systems</span>
          <select
            aria-label="Saved systems"
            className={selectClass}
            value={pickedSaved}
            onChange={(e) => setPickedSaved(e.target.value)}
          >
            <option value="">Pick a system…</option>
            {saved.systems.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </label>
        <Button
          type="button"
          variant="outline"
          disabled={!loadable}
          onClick={() => {
            if (!loadable) return;
            dispatch({ type: "load", system: loadable });
            setSaveState(undefined);
            setFailure(undefined);
          }}
        >
          Load
        </Button>
      </section>

      <Outcome
        shown={shown}
        failure={current}
        hasFieldErrors={fields.length > 0}
        backtest={run.isPending ? "pending" : run.data}
        variations={variations}
        universe={draft.universe}
        scan={scanned === undefined ? undefined : scan.isPending ? "pending" : scan.isSuccess ? scan.data : undefined}
        mode={mode}
        onModeChange={setMode}
        onOpenReport={(symbol) => onOpenReport(symbol, scanned?.system.timeframe ?? "daily")}
      />
    </div>
  );
}

/** The last backtest or scan, or why it failed. */
function Outcome({
  shown,
  failure,
  hasFieldErrors,
  backtest,
  variations,
  scan,
  universe,
  mode,
  onModeChange,
  onOpenReport,
}: {
  shown: "backtest" | "scan" | undefined;
  failure: Failure | undefined;
  hasFieldErrors: boolean;
  backtest: BacktestResult | "pending" | undefined;
  variations: number;
  scan: Parameters<typeof ScanResults>[0]["result"] | "pending" | undefined;
  universe: string;
  mode: ScanRequestMode;
  onModeChange: (m: ScanRequestMode) => void;
  onOpenReport: (symbol: string) => void;
}) {
  const [sort, setSort] = useState<string>();
  const error = failure ? (
    hasFieldErrors ? (
      <p role="alert" className="text-destructive">
        Some settings need changing; see the notes beside them.
      </p>
    ) : (
      <ScanError error={failure.error} />
    )
  ) : null;

  if (shown === "backtest") {
    if (backtest === "pending") return <p className="text-muted-foreground">Backtesting {universe} since 2007…</p>;
    if (failure) return error;
    return backtest ? <BacktestResults result={backtest} variations={variations} /> : null;
  }
  if (shown === "scan") {
    return (
      <div className="flex flex-col gap-4">
        <ModeToggle mode={mode} onChange={onModeChange} />
        {scan === "pending" ? (
          <p className="text-muted-foreground">Scanning {universe}…</p>
        ) : failure ? (
          error
        ) : scan ? (
          <ScanResults result={scan} sort={sort} onSortChange={setSort} onOpen={onOpenReport} />
        ) : null}
      </div>
    );
  }
  return error;
}
