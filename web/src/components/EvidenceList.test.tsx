import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vite-plus/test";
import type { Evidence, EvidenceType, NotEvaluableReason, ReportLine } from "@/api/client";
import { EvidenceList, describeEvidence } from "./EvidenceList";

// One fixture per union member. Typed as a Record over the discriminator,
// so adding a variant to the API breaks this file until it gets a fixture.
const fixtures: { [K in EvidenceType]: Extract<Evidence, { type: K }> } = {
  n_year_high: {
    type: "n_year_high",
    level: 1240,
    level_date: "2023-01-12",
    bars_since: 372,
    close: 1266.1,
    pct_above: 2.1048387096774,
  },
  level_age: { type: "level_age", level_date: "2023-01-12", bars_since: 470 },
  not_extended: { type: "not_extended", pct_above: 2.1048387096774 },
  relative_strength: { type: "relative_strength", stock_return: 0.2, bench_return: 0.1, diff: 0.1 },
  volume_surge: { type: "volume_surge", ratio: 2.345, avg_volume: 123456.7 },
  not_evaluable: { type: "not_evaluable", reason: "insufficient_history" },
};

function line(evidence: Evidence, pass: boolean): ReportLine {
  return {
    type: "n_year_high",
    label: "New N-year high",
    params: { years: 3, all_time: false },
    result: { pass, evaluable: evidence.type !== "not_evaluable", evidence },
  };
}

describe("EvidenceList", () => {
  test("EvidenceList handles every evidence type", () => {
    const expected: Record<EvidenceType, string> = {
      n_year_high: "Old high 1,240.00 set 2023-01-12, 1 year 10 months ago (372 bars); close 1,266.10, 2.10% above",
      level_age: "Old high set 2023-01-12, 1 year 10 months ago (470 bars)",
      not_extended: "Close 2.10% above the breakout level",
      relative_strength: "Stock +20.00% vs benchmark +10.00%: 10.00 points ahead",
      volume_surge: "Volume 2.35× the average of 1,23,457",
      not_evaluable: "not evaluable (insufficient history)",
    };
    for (const [type, ev] of Object.entries(fixtures) as [EvidenceType, Evidence][]) {
      expect(describeEvidence(ev, "2024-11-20")).toBe(expected[type]);
    }
  });

  test("lagging the benchmark and a falling market read in plain English", () => {
    const ev: Evidence = { type: "relative_strength", stock_return: -0.05, bench_return: -0.12, diff: 0.07 };
    expect(describeEvidence(ev, "2024-11-20")).toBe("Stock −5.00% vs benchmark −12.00%: 7.00 points ahead");
    const lag: Evidence = { type: "relative_strength", stock_return: 0.03, bench_return: 0.1, diff: -0.07 };
    expect(describeEvidence(lag, "2024-11-20")).toBe("Stock +3.00% vs benchmark +10.00%: 7.00 points behind");
  });

  test("the filters' not-evaluable reasons are described", () => {
    const expected: Partial<Record<NotEvaluableReason, string>> = {
      benchmark_missing: "not evaluable (benchmark missing)",
      no_breakout_level: "not evaluable (no breakout level)",
      zero_denominator: "not evaluable (zero denominator)",
    };
    for (const [reason, text] of Object.entries(expected) as [NotEvaluableReason, string][]) {
      expect(describeEvidence({ type: "not_evaluable", reason }, "2024-11-20")).toBe(text);
    }
  });

  test("close below the level reads as below", () => {
    const ev = { ...fixtures.n_year_high, close: 1200, pct_above: -3.2258064516129 };
    expect(describeEvidence(ev, "2023-02-01")).toBe(
      "Old high 1,240.00 set 2023-01-12, less than a month ago (372 bars); close 1,200.00, 3.23% below",
    );
  });

  test("renders a mark and plain English per line", () => {
    render(
      <EvidenceList
        asOf="2024-11-20"
        lines={[line(fixtures.n_year_high, true), line(fixtures.not_evaluable, false)]}
      />,
    );
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent("✅");
    expect(items[0]).toHaveTextContent("New N-year high");
    expect(items[0]).toHaveTextContent("Old high 1,240.00 set 2023-01-12");
    expect(items[1]).toHaveTextContent("❌");
    expect(items[1]).toHaveTextContent("not evaluable (insufficient history)");
  });
});
