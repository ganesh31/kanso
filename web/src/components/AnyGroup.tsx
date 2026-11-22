import { useState } from "react";
import type { Meta } from "@/api/client";
import { Button } from "@/components/ui/button";
import type { DraftAction, DraftCondition, Group } from "@/state/draft";
import { BlockErrors, ConditionBlock } from "./ConditionBlock";
import { withDefaults } from "./ParamEditor";
import { selectClass } from "./PatternPicker";

/** The ANY group holds at most this many conditions (spec §5.5). */
export const MAX_ANY = 4;

const other = (g: Group): Group => (g === "all" ? "any" : "all");

/**
 * A group of condition blocks with a picker to add one: the required conditions ("all") or the
 * "Match ANY of these" group. `canAdd` false disables the add button.
 */
export function ConditionGroup({
  group,
  title,
  hint,
  conditions,
  patterns,
  errors,
  canAdd = true,
  dispatch,
}: {
  group: Group;
  title: string;
  hint: string;
  conditions: DraftCondition[];
  patterns: Meta[];
  /** The API's errors by block key (see `errorsByBlock`). */
  errors: Map<string, string[]>;
  canAdd?: boolean;
  dispatch: (a: DraftAction) => void;
}) {
  const [picked, setPicked] = useState<string>();
  const type = picked ?? patterns[0]?.type ?? "";
  const meta = patterns.find((p) => p.type === type);
  const headingId = `group-${group}`;
  return (
    <section role="group" aria-labelledby={headingId} className="flex flex-col gap-1">
      <h2 id={headingId} className="text-lg font-bold">
        {title}
      </h2>
      <p className="text-muted-foreground text-sm">{hint}</p>
      <BlockErrors errors={errors.get(`group:${group}`)} />
      <div className="flex flex-col">
        {conditions.map((c, index) => (
          <ConditionBlock
            // Blocks have no ids of their own; the index is their identity within the group.
            key={`${index}-${c.type}`}
            condition={c}
            meta={patterns.find((p) => p.type === c.type)}
            idPrefix={`${group}-${index}`}
            errors={errors.get(`${group}:${index}`)}
            moveLabel={group === "all" ? "Move to ANY" : "Make required"}
            onToggle={() => dispatch({ type: "toggle", group, index })}
            onParam={(name, value) => dispatch({ type: "setParam", group, index, name, value })}
            onMove={() => dispatch({ type: "setGroup", from: group, index, to: other(group) })}
            onRemove={() => dispatch({ type: "removeCondition", group, index })}
          />
        ))}
      </div>
      <div className="flex flex-wrap items-end gap-2 pt-2">
        <label className="flex flex-col gap-1 text-sm">
          <span>Add a condition</span>
          <select className={selectClass} value={type} onChange={(e) => setPicked(e.target.value)}>
            {patterns.map((p) => (
              <option key={p.type} value={p.type}>
                {p.label}
              </option>
            ))}
          </select>
        </label>
        <Button
          type="button"
          variant="outline"
          disabled={!meta || !canAdd}
          onClick={() => {
            if (meta) dispatch({ type: "addCondition", group, condition: { type, params: withDefaults(meta.params) } });
          }}
        >
          Add
        </Button>
      </div>
    </section>
  );
}

/** The one "Match ANY of these" group: its add button is enabled only while it has fewer than 4 items. */
export function AnyGroup(props: Omit<Parameters<typeof ConditionGroup>[0], "group" | "title" | "hint" | "canAdd">) {
  return (
    <ConditionGroup
      {...props}
      group="any"
      title="Match ANY of these"
      hint={`At least one of these must also hold. Use 2 to ${MAX_ANY}, or leave it empty.`}
      canAdd={props.conditions.length < MAX_ANY}
    />
  );
}
