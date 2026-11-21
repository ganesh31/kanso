import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, test, vi } from "vite-plus/test";
import type { Match } from "@/api/client";
import { MatchTable } from "./MatchTable";

const outcome = { pass: true, evaluable: true, conditions: [], any_matched: [] };

function match(symbol: string, pct_above?: number): Match {
  return pct_above === undefined
    ? { symbol, date: "2024-09-10", outcome }
    : { symbol, date: "2024-09-10", outcome, pct_above, level: 100, level_date: "2022-11-10" };
}

// As the server sends them: by % above, nulls last, then symbol.
const matches = [match("C", 1), match("B", 2), match("D", 2), match("A")];

function symbols(): string[] {
  return screen
    .getAllByRole("row")
    .slice(1)
    .map((r) => within(r).getAllByRole("cell")[0]?.textContent ?? "");
}

describe("MatchTable", () => {
  test("sorting by % above is stable and nulls last", () => {
    const onSortChange = vi.fn();
    const { rerender } = render(
      <MatchTable matches={matches} sort="pct_above" onSortChange={onSortChange} onOpen={() => {}} />,
    );
    expect(symbols()).toEqual(["C", "B", "D", "A"]);

    fireEvent.click(screen.getByRole("button", { name: /% above/ }));
    expect(onSortChange).toHaveBeenCalledWith("-pct_above");

    rerender(<MatchTable matches={matches} sort="-pct_above" onSortChange={onSortChange} onOpen={() => {}} />);
    // Ties keep their order and the stock without a level stays last.
    expect(symbols()).toEqual(["B", "D", "C", "A"]);
  });

  test("sorts by symbol", () => {
    render(<MatchTable matches={matches} sort="symbol" onSortChange={() => {}} onOpen={() => {}} />);
    expect(symbols()).toEqual(["A", "B", "C", "D"]);
  });

  test("shows level, old high age and fresh signal date", () => {
    const m: Match = {
      ...match("TCS", 2.5),
      signal_date: "2024-09-09",
      vol_ratio: 3,
      rs_diff: 0.05,
      level_age_bars: 470,
    };
    render(<MatchTable matches={[m]} sort="pct_above" onSortChange={() => {}} onOpen={() => {}} />);
    const cells = within(screen.getAllByRole("row")[1]!)
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells).toEqual(["TCS", "100.00", "2022-11-10", "1y 10m", "2.50%", "3.00×", "+5.00%", "2024-09-09"]);
  });

  test("a weekly scan shows the provisional column", () => {
    const m: Match = { ...match("TCS", 1), provisional: { ...outcome, pass: false } };
    render(<MatchTable matches={[m]} sort="pct_above" onSortChange={() => {}} onOpen={() => {}} />);
    expect(screen.getByRole("columnheader", { name: /This week/ })).toBeInTheDocument();
    expect(screen.getByText("Fails so far")).toBeInTheDocument();
  });

  test("hides columns no match fills, such as volume and RS on a one-pattern scan", () => {
    render(<MatchTable matches={[match("TCS", 2.5)]} sort="pct_above" onSortChange={() => {}} onOpen={() => {}} />);
    expect(screen.queryByRole("columnheader", { name: /Volume/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("columnheader", { name: /RS vs Nifty/ })).not.toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: /% above/ })).toBeInTheDocument();
  });
});
