import { z } from "zod";
import { type Condition, type Costs, type Exits, type Portfolio, type System, SystemTimeframe } from "@/api/client";

/** A condition in the Builder: its params, and whether it is switched off (kept in the draft, not sent). */
export interface DraftCondition {
  type: string;
  params: Record<string, unknown>;
  off?: boolean;
}

export type Group = "all" | "any";

/**
 * The System Builder's working copy of a system. It differs from a `System` only in keeping
 * switched-off conditions, so a toggle can be undone and survives in the `?draft=` link.
 */
export interface Draft {
  id?: string;
  name: string;
  universe: string;
  timeframe: SystemTimeframe;
  all: DraftCondition[];
  any: DraftCondition[];
  exits?: Exits;
  costs?: Costs;
  portfolio?: Portfolio;
}

export type DraftAction =
  | { type: "addCondition"; group: Group; condition: Condition }
  | { type: "removeCondition"; group: Group; index: number }
  | { type: "toggle"; group: Group; index: number }
  | { type: "setParam"; group: Group; index: number; name: string; value: unknown }
  /** Moves a condition to the end of the other group. */
  | { type: "setGroup"; from: Group; index: number; to: Group }
  | { type: "setExits"; exits: Exits }
  | { type: "setCosts"; costs: Costs }
  | { type: "setPortfolio"; portfolio: Portfolio }
  | { type: "setTimeframe"; timeframe: SystemTimeframe }
  | { type: "setUniverse"; universe: string }
  | { type: "setName"; name: string }
  | { type: "load"; system: System };

const toDraftCondition = (c: Condition): DraftCondition => ({ type: c.type, params: { ...c.params } });

export function fromSystem(s: System): Draft {
  const d: Draft = {
    name: s.name,
    universe: s.universe,
    timeframe: s.timeframe ?? "daily",
    all: s.entry.all.map(toDraftCondition),
    any: (s.entry.any ?? []).map(toDraftCondition),
  };
  if (s.id) d.id = s.id;
  if (s.exits) d.exits = { ...s.exits };
  if (s.costs) d.costs = { ...s.costs };
  if (s.portfolio) d.portfolio = { ...s.portfolio };
  return d;
}

/** The switched-on conditions of a group, each with its index in the draft. */
export function sentConditions(conds: DraftCondition[]): { condition: Condition; index: number }[] {
  return conds.flatMap((c, index) => {
    if (c.off) return [];
    const condition: Condition = { type: c.type };
    if (Object.keys(c.params).length > 0) condition.params = { ...c.params };
    return [{ condition, index }];
  });
}

/** The system the draft describes, as the API takes it: switched-off conditions left out. */
export function toSystem(d: Draft): System {
  const s: System = {
    name: d.name,
    universe: d.universe,
    timeframe: d.timeframe,
    entry: { all: sentConditions(d.all).map((c) => c.condition) },
  };
  const any = sentConditions(d.any).map((c) => c.condition);
  if (any.length > 0) s.entry.any = any;
  if (d.id) s.id = d.id;
  if (d.exits) s.exits = { ...d.exits };
  if (d.costs) s.costs = { ...d.costs };
  if (d.portfolio) s.portfolio = { ...d.portfolio };
  return s;
}

function updateAt<T>(list: T[], index: number, f: (t: T) => T): T[] {
  return list.map((t, i) => (i === index ? f(t) : t));
}

export function systemReducer(d: Draft, a: DraftAction): Draft {
  switch (a.type) {
    case "addCondition":
      return { ...d, [a.group]: [...d[a.group], toDraftCondition(a.condition)] };
    case "removeCondition":
      return { ...d, [a.group]: d[a.group].filter((_, i) => i !== a.index) };
    case "toggle":
      return { ...d, [a.group]: updateAt(d[a.group], a.index, (c) => ({ ...c, off: !c.off })) };
    case "setParam":
      return {
        ...d,
        [a.group]: updateAt(d[a.group], a.index, (c) => ({ ...c, params: { ...c.params, [a.name]: a.value } })),
      };
    case "setGroup": {
      const moved = d[a.from][a.index];
      if (!moved || a.from === a.to) return d;
      return { ...d, [a.from]: d[a.from].filter((_, i) => i !== a.index), [a.to]: [...d[a.to], moved] };
    }
    case "setExits":
      return { ...d, exits: a.exits };
    case "setCosts":
      return { ...d, costs: a.costs };
    case "setPortfolio":
      return { ...d, portfolio: a.portfolio };
    case "setTimeframe":
      return { ...d, timeframe: a.timeframe };
    case "setUniverse":
      return { ...d, universe: a.universe };
    case "setName":
      return { ...d, name: a.name };
    case "load":
      return fromSystem(a.system);
  }
}

const conditionSchema = z.object({
  type: z.string(),
  params: z.record(z.string(), z.unknown()),
  off: z.boolean().optional(),
});

const draftSchema = z.object({
  id: z.string().optional(),
  name: z.string(),
  universe: z.string(),
  timeframe: z.enum(SystemTimeframe),
  all: z.array(conditionSchema),
  any: z.array(conditionSchema),
  exits: z.object({ stop_pct: z.number(), trend_sma_weeks: z.number() }).optional(),
  costs: z.object({ per_side_pct: z.number(), slippage_pct: z.number() }).optional(),
  portfolio: z.object({ capital: z.number(), max_positions: z.number() }).optional(),
});

/** The draft as base64url JSON (UTF-8), for the `?draft=` search param. */
export function encodeDraft(d: Draft): string {
  const bytes = new TextEncoder().encode(JSON.stringify(d));
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin).replaceAll("+", "-").replaceAll("/", "_").replace(/=+$/, "");
}

/** The draft a `?draft=` value holds, or undefined when it isn't one (a bad link opens the default). */
export function decodeDraft(encoded: string): Draft | undefined {
  try {
    const bin = atob(encoded.replaceAll("-", "+").replaceAll("_", "/"));
    const json = new TextDecoder("utf-8", { fatal: true }).decode(Uint8Array.from(bin, (ch) => ch.charCodeAt(0)));
    const parsed = draftSchema.safeParse(JSON.parse(json));
    return parsed.success ? parsed.data : undefined;
  } catch {
    return undefined;
  }
}
