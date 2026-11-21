const price = new Intl.NumberFormat("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** ₹ amounts and prices, 2 decimals, Indian digit grouping (ADR 0004: round for display only). */
export function formatPrice(n: number): string {
  return price.format(n);
}

/** "2.10% above" / "3.23% below". */
export function formatPctVs(pct: number, above = "above", below = "below"): string {
  return `${Math.abs(pct).toFixed(2)}% ${pct >= 0 ? above : below}`;
}

/** "+20.00%" / "−5.00%" (a true minus sign). */
export function formatSignedPct(pct: number): string {
  return `${pct < 0 ? "−" : "+"}${Math.abs(pct).toFixed(2)}%`;
}

const volume = new Intl.NumberFormat("en-IN", { maximumFractionDigits: 0 });

/** Share volumes, whole shares, Indian digit grouping: "1,23,457". */
export function formatVolume(n: number): string {
  return volume.format(n);
}

function parseDate(d: string): { y: number; m: number; day: number } {
  const [y = 0, m = 0, day = 0] = d.split("-").map(Number);
  return { y, m, day };
}

/** Whole calendar months from one YYYY-MM-DD date to a later one. */
export function monthsBetween(from: string, to: string): number {
  const a = parseDate(from);
  const b = parseDate(to);
  const months = (b.y - a.y) * 12 + (b.m - a.m) - (b.day < a.day ? 1 : 0);
  return Math.max(0, months);
}

const plural = (n: number, word: string) => `${n} ${word}${n === 1 ? "" : "s"}`;

/** "1 year 10 months ago", "3 months ago", "less than a month ago". */
export function formatAge(from: string, to: string): string {
  const months = monthsBetween(from, to);
  if (months === 0) return "less than a month ago";
  const years = Math.floor(months / 12);
  const rest = months % 12;
  const parts = [years > 0 ? plural(years, "year") : "", rest > 0 ? plural(rest, "month") : ""];
  return `${parts.filter(Boolean).join(" ")} ago`;
}

/** "insufficient_history" → "insufficient history". */
export function humanize(code: string): string {
  return code.replaceAll("_", " ");
}
