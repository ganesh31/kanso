import type * as React from "react";
import { cn } from "@/lib/utils";

/**
 * One look for every "pick one of these" control (Scanner tabs, Fresh/All, Daily/Weekly):
 * a recessed track with the chosen option raised. Callers keep their own semantics
 * (role="tab" + aria-selected, or aria-pressed) on the buttons.
 */
export function SegmentedTrack({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("bg-muted inline-flex gap-0.5 rounded-md p-0.5", className)} {...props} />;
}

export function segmentClass(active: boolean): string {
  return cn(
    "h-8 rounded-[calc(var(--radius)-2px)] px-3 text-sm font-medium transition-colors outline-none",
    "focus-visible:ring-ring focus-visible:ring-2",
    active ? "bg-input text-foreground shadow-xs" : "text-muted-foreground hover:text-foreground",
  );
}
