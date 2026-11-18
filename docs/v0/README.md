# Kanso V0: see the end goal with tools that already exist

**Purpose.** Before writing Kanso, use existing tools to (1) learn what multi-year breakouts look like, (2) feel what each Kanso screen should do, and (3) record **reference answers** that V1's Go engine will be checked against.

**Rules**
- **Time-box: 3 days.** If a tool won't install in ~1 hour, skip it and note why.
- Everything in `v0/` is **throwaway**. V1 never imports it, copies it, or depends on it. We rebuild ideas, not code.
  - **One exception:** the idea in `v0/backtest_vbt.py` carries into V1 as the **vectorbt benchmark** (spec §8). V1 gets its own version in `tools/vbt-benchmark/`, which reads Kanso's frozen data fixture instead of yfinance. The V0 script itself still stays throwaway.
- The output of V0 is **notes** in [`reference-results.md`](reference-results.md), not software.
- Fixed test date: pick one recent trading day (e.g. the last Friday) and use it everywhere, so results are comparable.

## Map: which tool previews which Kanso piece

| Kanso piece | V0 tool | Where it runs |
|---|---|---|
| Quick Scan / Scanner | Chartink, PKScreener | Browser / Docker |
| Stock Report (recogniser) | stock-pattern, PKScreener | Python (local) |
| System Builder + backtest | TradingView Strategy Tester, vectorbt | Browser / Python (local) |
| Data layer | eod2 (NSE daily since 1995), Yahoo via yfinance | Local |
| P&F charts (V2) | pypnf | Python (local), optional |
| UI inspiration | Superalgos | Node (local), optional, 1-hour cap |

## Suggested order

- **Day 1 (browser only):** §1 TradingView, §2 Chartink. Pick ~10 breakouts and look at them on charts.
- **Day 2 (local):** §3 Python setup, §4 PKScreener, §5 eod2 + stock-pattern.
- **Day 3 (local):** §6 vectorbt backtest, then compare it with TradingView. Optional §7 pypnf, §8 Superalgos. Write up the notes.

---

## 1. TradingView: the System Builder, as a strategy

1. Free account at tradingview.com. Open a **daily** chart of any NSE stock (e.g. `NSE:RELIANCE`).
2. Pine Editor → paste [`../../v0/tradingview/kanso_v0.pine`](../../v0/tradingview/kanso_v0.pine) → *Add to chart*.
3. Open the **Strategy Tester** tab. Change inputs (years, volume ×, stop %) and watch how results move.
4. Record for 3–5 stocks: trades, win rate, net profit %, max drawdown, and the list of entry dates → `reference-results.md`.

What the script does is the V1 rule in daily mode: a close above the highest high of the prior N years, the volume, RS and "not extended" filters, entry at the next bar's open, an 8% stop, and a trend exit when the weekly close is below the 30-week SMA. Known differences from the Kanso spec are listed at the top of the script.

TradingView tests **one stock at a time**. Testing a whole stock list at once is what Kanso adds.

## 2. Chartink: Quick Scan

1. chartink.com → *Screeners* → *Create new scan*.
2. Build "close is the highest in 3 years" with the visual builder. A 3-year lookback is about 750 trading days. The scan text should look roughly like this (check it in the builder, because the syntax may differ):
   ```
   ( {cash} ( latest close > 1 day ago max( 750 , latest high )
     and latest volume > 2 * latest sma( latest volume , 50 ) ) )
   ```
3. Set the segment to **Nifty 500**. Run it, and also run it as of your fixed test date if Chartink lets you.
4. Record the matched symbols → `reference-results.md`. This list becomes a cross-check for Kanso's Quick Scan.
5. Click 3–4 results and look at the charts. Do they *look* like the breakouts you'd want?

## 3. Local Python setup (once)

You don't need to learn Python. You only run scripts. `uv` handles Python versions and packages:

```bash
brew install uv
```

```bash
cd ~/Workspace/kanso/v0 && uv venv --python 3.11 && source .venv/bin/activate
```

```bash
uv pip install vectorbt yfinance pandas pypnf
```

## 4. PKScreener: a full scanner for NSE (Docker)

Preview of the scanner and recogniser: 40+ built-in scans, breakouts, VCP and more.

```bash
docker run -it pkjmesra/pkscreener:latest
```

- It's a text menu. Explore the **breakout** and **52-week / all-time high** style scans on Nifty 500.
- Record: which scans felt useful, what columns it shows per stock, and how long a scan takes.
- Note what you'd want that it doesn't show (charts, evidence, "why").

## 5. eod2 + stock-pattern: data and pattern recognition

**eod2** downloads NSE daily data (stocks + indices, since 1995) from NSE's own end-of-day files. **stock-pattern** detects chart patterns (double tops, triangles, VCP, head & shoulders…) on that data.

1. Follow eod2's README: github.com/BennyThadikaran/eod2 (Mac/Linux install wiki). Run it once to build the data folder.
2. Follow stock-pattern's README: github.com/BennyThadikaran/stock-pattern. Run its `setup-config.py` and point `DATA_PATH` at eod2's `src/eod2_data/daily`.
3. Run it on 5–10 stocks from your Chartink list. Record which patterns it reports per stock. This is the "stock → patterns" direction, the same job as Kanso's Stock Report.
4. **V1 question to answer:** compare eod2's prices with Yahoo's for 2–3 stocks around a known split or bonus. If eod2 is cleaner, it may be a better V1 data source than Yahoo. Note it.

## 6. vectorbt: honest multi-stock backtest

1. Download the Nifty 50 constituents CSV from niftyindices.com (*Index constituents → NIFTY 50*). Save it as `v0/nifty50.csv`.
2. Run:
   ```bash
   cd ~/Workspace/kanso/v0 && source .venv/bin/activate && python -I backtest_vbt.py nifty50.csv
   ```
3. The script prints trade stats and portfolio stats, and saves `v0/out/trades.csv` and `v0/out/equity.html`.
4. Compare 3 stocks' entry dates with TradingView (§1). Matching dates means both tools agree on the rule. Differences are worth understanding before V1.

Fills are deliberately honest: signal at the close, buy at the **next day's open**, with costs. The limitations are listed in the script header.

## 7. pypnf: Point & Figure (optional, V2 preview)

```bash
cd ~/Workspace/kanso/v0 && source .venv/bin/activate && python -I pnf_view.py RELIANCE.NS
```
It prints the X/O columns. Just look: can you see multi-year resistance in a P&F chart more easily than in candles?

## 8. Superalgos: UI inspiration only (optional, 1-hour cap)

It's Node-based (you know Node). Follow the README at github.com/Superalgos/Superalgos. Don't trade, connect exchanges, or enter any keys. Only look at how it lays out strategy building, charts with layers, and backtest playback. Take 5–10 screenshots of what you like or dislike → `reference-results.md`.

---

## What to bring back to V1

Fill in [`reference-results.md`](reference-results.md). Then we:
1. Turn the breakouts you verified into **real-data tests** (spec §8).
2. Decide **Yahoo vs eod2** as the V1 data source.
3. Adjust any screen or default that V0 showed was wrong.
