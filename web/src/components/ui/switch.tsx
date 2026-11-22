import { cn } from "@/lib/utils";

/** An on/off switch (role="switch"); the caller names it with `aria-label` or a `<label htmlFor>`. */
export function Switch({
  checked,
  onChange,
  id,
  "aria-label": ariaLabel,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  id?: string;
  "aria-label"?: string;
}) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      onClick={() => onChange(!checked)}
      className={cn(
        "relative h-5 w-9 shrink-0 rounded-full border transition-colors",
        checked ? "bg-primary" : "bg-muted",
      )}
    >
      <span
        className={cn(
          "absolute top-0.5 left-0 size-3.5 rounded-full transition-transform",
          checked ? "bg-primary-foreground translate-x-4" : "bg-muted-foreground translate-x-0.5",
        )}
      />
    </button>
  );
}
