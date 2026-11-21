import type { ParamSpec } from "@/api/client";
import { Input } from "@/components/ui/input";
import type { ParamValue, ParamValues } from "@/lib/scannerSearch";
import { cn } from "@/lib/utils";

/** The pattern's defaults overlaid with the given values: every param has a value. */
export function withDefaults(specs: ParamSpec[], values: ParamValues = {}): ParamValues {
  const out: ParamValues = {};
  for (const s of specs) {
    const v = values[s.name] ?? s.default;
    if (typeof v === "number" || typeof v === "boolean" || typeof v === "string") out[s.name] = v;
  }
  return out;
}

/**
 * Edits a pattern's params from its specs: number inputs with min/max/step and a unit suffix,
 * a switch for bools, a text input for symbols. Ranges are checked by the server.
 */
export function ParamEditor({
  specs,
  params,
  onChange,
}: {
  specs: ParamSpec[];
  params: ParamValues;
  onChange: (next: ParamValues) => void;
}) {
  const set = (name: string, v: ParamValue) => onChange({ ...params, [name]: v });
  if (specs.length === 0) return null;
  return (
    <div className="flex flex-wrap items-end gap-4">
      {specs.map((s) => {
        const id = `param-${s.name}`;
        const value = params[s.name];
        if (s.kind === "bool") {
          const on = value === true;
          return (
            <div key={s.name} className="flex items-center gap-2 text-sm">
              <button
                id={id}
                type="button"
                role="switch"
                aria-checked={on}
                aria-label={s.label}
                onClick={() => set(s.name, !on)}
                className={cn("relative h-5 w-9 rounded-full border transition-colors", on ? "bg-primary" : "bg-muted")}
              >
                <span
                  className={cn(
                    "absolute top-0.5 left-0 size-3.5 rounded-full transition-transform",
                    on ? "bg-primary-foreground translate-x-4" : "bg-muted-foreground translate-x-0.5",
                  )}
                />
              </button>
              <label htmlFor={id}>{s.label}</label>
            </div>
          );
        }
        return (
          <label key={s.name} htmlFor={id} className="flex flex-col gap-1 text-sm">
            <span>
              {s.label}
              {s.unit && !s.label.toLowerCase().includes(s.unit) ? (
                <span className="text-muted-foreground"> ({s.unit})</span>
              ) : null}
            </span>
            {s.kind === "number" ? (
              <Input
                id={id}
                type="number"
                min={s.min}
                max={s.max}
                step={s.step > 0 ? s.step : "any"}
                value={typeof value === "number" ? value : ""}
                onChange={(e) => {
                  const n = e.target.valueAsNumber;
                  if (!Number.isNaN(n)) set(s.name, n);
                }}
                className="w-28"
              />
            ) : (
              <Input
                id={id}
                value={typeof value === "string" ? value : ""}
                onChange={(e) => set(s.name, e.target.value)}
                className="w-28"
              />
            )}
          </label>
        );
      })}
    </div>
  );
}
