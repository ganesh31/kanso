import { render, screen } from "@testing-library/react";
import { describe, expect, test } from "vitest";
import type { Evidence, EvidenceType, ReportLine } from "@/api/client";
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
      n_year_high:
        "Old high 1,240.00 set 2023-01-12, 1 year 10 months ago (372 bars); close 1,266.10, 2.10% above",
      not_evaluable: "not evaluable (insufficient history)",
    };
    for (const [type, ev] of Object.entries(fixtures) as [EvidenceType, Evidence][]) {
      expect(describeEvidence(ev, "2024-11-20")).toBe(expected[type]);
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
