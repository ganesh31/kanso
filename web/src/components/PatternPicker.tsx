import type { Meta } from "@/api/client";

const selectClass =
  "border-input bg-background h-9 rounded-md border px-2 text-sm shadow-xs outline-none focus-visible:ring-2 focus-visible:ring-ring";

/** Picks one pattern or filter from the registry. */
export function PatternPicker({
  patterns,
  value,
  onChange,
}: {
  patterns: Meta[];
  value: string;
  onChange: (type: string) => void;
}) {
  const current = patterns.find((p) => p.type === value);
  return (
    <label className="flex max-w-md flex-col gap-1 text-sm">
      <span>Pattern</span>
      <select aria-label="Pattern" className={selectClass} value={value} onChange={(e) => onChange(e.target.value)}>
        {patterns.map((p) => (
          <option key={p.type} value={p.type}>
            {p.label}
          </option>
        ))}
      </select>
      {current?.description ? <span className="text-muted-foreground text-xs">{current.description}</span> : null}
    </label>
  );
}

export { selectClass };
