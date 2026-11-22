import type { Costs, Exits, ParamSpec, Portfolio } from "@/api/client";
import type { ParamValues } from "@/lib/scannerSearch";
import { BlockErrors } from "./ConditionBlock";
import { ParamEditor, withDefaults } from "./ParamEditor";

const field = (name: string, label: string, min: number, max: number, step = 0): ParamSpec => ({
  name,
  label,
  kind: "number",
  unit: "",
  default: undefined,
  min,
  max,
  step,
});

// Input ranges match the server's (internal/rules/validate.go); the server still decides.
const exitSpecs = [
  field("stop_pct", "Stop-loss below entry (%)", 0, 50),
  field("trend_sma_weeks", "Sell below this weekly average (weeks, 0 for none)", 0, 100, 1),
];
const costSpecs = [
  field("per_side_pct", "Costs per side (%)", 0, 5),
  field("slippage_pct", "Slippage per side (%)", 0, 5),
];
const portfolioSpecs = [
  field("capital", "Starting capital (₹)", 0, 1e12),
  field("max_positions", "Most positions at once", 1, 50, 1),
];

function FieldsBlock<T extends object>({
  label,
  hint,
  specs,
  value,
  errors,
  onChange,
}: {
  label: string;
  hint: string;
  specs: ParamSpec[];
  value: T | undefined;
  errors?: string[];
  onChange: (next: T) => void;
}) {
  const params = withDefaults(specs, value as ParamValues | undefined);
  return (
    <div role="group" aria-label={label} className="flex flex-col gap-2 border-t py-4 first:border-t-0">
      <div className="flex flex-col">
        <span className="font-semibold">{label}</span>
        <span className="text-muted-foreground text-sm">{hint}</span>
      </div>
      <ParamEditor
        idPrefix={label.toLowerCase()}
        specs={specs}
        params={params}
        onChange={(next) => onChange(next as unknown as T)}
      />
      <BlockErrors errors={errors} />
    </div>
  );
}

/** When a trade is sold, what each fill costs, and how much is invested. */
export function ExitBlock({
  exits,
  costs,
  portfolio,
  errors,
  onExits,
  onCosts,
  onPortfolio,
}: {
  exits: Exits | undefined;
  costs: Costs | undefined;
  portfolio: Portfolio | undefined;
  /** The API's errors by block key (see `errorsByBlock`). */
  errors: Map<string, string[]>;
  onExits: (e: Exits) => void;
  onCosts: (c: Costs) => void;
  onPortfolio: (p: Portfolio) => void;
}) {
  return (
    <section aria-labelledby="exit-heading" className="flex flex-col gap-1">
      <h2 id="exit-heading" className="text-lg font-bold">
        Exits, costs and sizing
      </h2>
      <FieldsBlock
        label="Exits"
        hint="Sell at the stop, or when a week closes below its average. Fills are at the next day's open."
        specs={exitSpecs}
        value={exits}
        errors={errors.get("exits")}
        onChange={onExits}
      />
      <FieldsBlock
        label="Costs"
        hint="Charged on every buy and every sell."
        specs={costSpecs}
        value={costs}
        errors={errors.get("costs")}
        onChange={onCosts}
      />
      <FieldsBlock
        label="Portfolio"
        hint="Each new position gets the equity divided by the most positions, in whole shares."
        specs={portfolioSpecs}
        value={portfolio}
        errors={errors.get("portfolio")}
        onChange={onPortfolio}
      />
    </section>
  );
}
