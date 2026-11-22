import { render, screen, within } from "@testing-library/react";
import { describe, expect, test } from "vite-plus/test";
import type { Honesty } from "@/api/client";
import { HonestyPanel } from "./HonestyPanel";

const honesty: Honesty = {
  fill_model: "next-day open",
  per_side_pct: 0.2,
  slippage_pct: 0.1,
  survivorship: true,
  data_source: "Yahoo Finance",
  adjustment: "split-adjusted, not dividend-adjusted",
  data_start: "2007-09-17",
  data_end: "2026-11-20",
  ranking_rule: "crowded days ranked by 6-month RS, then symbol",
  skipped: [{ symbol: "NEWCO", reason: "no_data" }],
  skipped_signals: [
    { symbol: "MRF", entry_date: "2024-03-04", reason: "price_above_slot" },
    { symbol: "HDFCBANK", entry_date: "2014-05-12", reason: "insufficient_cash" },
  ],
};

const panel = () => screen.getByRole("complementary", { name: "How this was tested" });

describe("HonestyPanel", () => {
  test("shows how the backtest was run", () => {
    render(<HonestyPanel honesty={honesty} variations={1} />);
    const p = within(panel());
    expect(p.getByText(/next-day open/)).toBeInTheDocument();
    expect(p.getByText(/0\.2% per side/)).toBeInTheDocument();
    expect(p.getByText(/0\.1% per side/)).toBeInTheDocument();
    expect(p.getByText(/Yahoo Finance/)).toBeInTheDocument();
    expect(p.getByText(/split-adjusted, not dividend-adjusted/)).toBeInTheDocument();
    expect(p.getByText(/2007-09-17 to 2026-11-20/)).toBeInTheDocument();
    expect(p.getByText(/NEWCO/)).toBeInTheDocument();
  });

  test("honesty panel shows survivorship warning and overfitting warning above 10", () => {
    const { rerender } = render(<HonestyPanel honesty={honesty} variations={10} />);
    expect(within(panel()).getByText(/survivorship bias/)).toBeInTheDocument();
    expect(within(panel()).getByText(/^10$/)).toBeInTheDocument();
    expect(screen.queryByText(/more likely the best one is luck/)).not.toBeInTheDocument();

    rerender(<HonestyPanel honesty={honesty} variations={11} />);
    expect(within(panel()).getByRole("alert")).toHaveTextContent(
      "Tested 11 variations. The more you try, the more likely the best one is luck.",
    );
  });

  test("no survivorship warning when the stock list has history", () => {
    render(<HonestyPanel honesty={{ ...honesty, survivorship: false }} variations={1} />);
    expect(screen.queryByText(/survivorship bias/)).not.toBeInTheDocument();
  });

  test("honesty panel lists ranking rule and skipped signals", () => {
    render(<HonestyPanel honesty={honesty} variations={1} />);
    const p = within(panel());
    expect(p.getByText(/crowded days ranked by 6-month RS, then symbol/)).toBeInTheDocument();
    expect(p.getByText(/1 signal skipped: price above slot size \(MRF 2024-03-04\)/)).toBeInTheDocument();
    expect(p.getByText(/1 signal skipped: insufficient cash \(HDFCBANK 2014-05-12\)/)).toBeInTheDocument();
  });

  test("says when no signal was skipped", () => {
    render(<HonestyPanel honesty={{ ...honesty, skipped_signals: [] }} variations={1} />);
    expect(within(panel()).getByText("None")).toBeInTheDocument();
  });
});
