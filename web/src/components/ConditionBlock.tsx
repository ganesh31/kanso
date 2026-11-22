import type { Meta } from "@/api/client";
import { describeRule } from "@/components/EvidenceList";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { humanize } from "@/lib/format";
import type { ParamValue, ParamValues } from "@/lib/scannerSearch";
import { cn } from "@/lib/utils";
import type { DraftCondition } from "@/state/draft";
import { ParamEditor, withDefaults } from "./ParamEditor";

/** The API's messages for one block, shown beside it. */
export function BlockErrors({ errors }: { errors?: string[] }) {
  if (!errors || errors.length === 0) return null;
  return (
    <ul className="text-destructive flex flex-col gap-0.5 text-sm">
      {errors.map((e) => (
        <li key={e}>{e}</li>
      ))}
    </ul>
  );
}

/**
 * One condition in the System Builder: its name and the rule its params set in plain English,
 * an on/off switch, its params with units, and actions to move it to the other group or remove it.
 */
export function ConditionBlock({
  condition,
  meta,
  idPrefix,
  errors,
  moveLabel,
  onToggle,
  onParam,
  onMove,
  onRemove,
}: {
  condition: DraftCondition;
  /** The pattern's registry entry; undefined while the list loads or for an unknown type. */
  meta: Meta | undefined;
  idPrefix: string;
  errors?: string[];
  moveLabel: string;
  onToggle: () => void;
  onParam: (name: string, value: ParamValue) => void;
  onMove: () => void;
  onRemove: () => void;
}) {
  const label = meta?.label ?? humanize(condition.type);
  const specs = meta?.params ?? [];
  const params = withDefaults(specs, condition.params as ParamValues);
  const rule = describeRule(condition.type, params);
  const on = !condition.off;
  return (
    <div role="group" aria-label={label} className="flex flex-col gap-3 border-t py-4 first:border-t-0">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-start gap-3">
          <span className="pt-0.5">
            <Switch checked={on} onChange={onToggle} aria-label={`Use ${label}`} />
          </span>
          <div className={cn("flex flex-col", !on && "text-muted-foreground")}>
            <span className="font-semibold">
              {label}
              {on ? null : <span className="font-normal">, switched off</span>}
            </span>
            {rule ? <span className="text-muted-foreground text-sm">{rule}</span> : null}
          </div>
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="ghost" size="sm" onClick={onMove}>
            {moveLabel}
          </Button>
          <Button type="button" variant="ghost" size="sm" onClick={onRemove} aria-label={`Remove ${label}`}>
            Remove
          </Button>
        </div>
      </div>
      {on ? (
        <ParamEditor
          idPrefix={idPrefix}
          specs={specs}
          params={params}
          onChange={(next) => {
            const changed = Object.keys(next).find((k) => next[k] !== params[k]);
            if (changed !== undefined) onParam(changed, next[changed]!);
          }}
        />
      ) : null}
      <BlockErrors errors={errors} />
    </div>
  );
}
