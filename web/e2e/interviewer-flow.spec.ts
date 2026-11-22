import { expect, test } from "@playwright/test";

// Spec §10: the two-minute flow an interviewer runs on the demo, from the default system to one trade's chart.
test("default system backtests against Nifty and charts a trade", async ({ page }) => {
  await page.goto("/builder");

  // The default system loads into the editor.
  await expect(page.getByLabel("System name")).toHaveValue("3-year breakout");
  await expect(page.getByLabel("Stock list")).toHaveValue("NIFTY50");

  await page.getByRole("button", { name: "Backtest", exact: true }).click();

  // Each portfolio tile sets the system beside buy-and-hold Nifty.
  const results = page.getByRole("region", { name: "Results" });
  for (const tile of ["CAGR", "Max drawdown"]) {
    await expect(results.getByRole("group", { name: tile, exact: true })).toContainText(/Nifty [+−-]?\d+\.\d+%/);
  }

  // The honesty panel states the fill model and the survivorship warning.
  const honesty = page.getByRole("complementary", { name: "How this was tested" });
  await expect(honesty).toContainText("next-day open");
  await expect(honesty).toContainText("survivorship bias");

  // The first trade opens its chart with an entry and an exit marker.
  const firstTrade = results
    .getByRole("table", { name: "Trades" })
    .getByRole("button", { name: /^Chart the / })
    .first();
  const symbol = (await firstTrade.getAttribute("aria-label"))?.match(/^Chart the (\S+) trade/)?.[1];
  expect(symbol).toBeTruthy();
  await firstTrade.click();
  const chart = page.getByRole("figure", { name: `${symbol} trade chart` });
  await expect(chart.getByTestId("price-chart")).toHaveAttribute("data-markers", "2");
});
