# Kanso: Multi-Year Breakout Scanner, Recogniser & System Builder — Design

Date: 2026-11-19
Status: Approved 2026-11-19

## 1. Intent

Kanso is a web tool for finding and testing **multi-year breakouts** in Indian equities (NSE). It has three faces over one engine:

1. **System Builder** — compose a breakout rule (entry conditions + exits) without code, backtest it over history (since 2007, see §6.1), and see honest results.
2. **Scanner** (pattern → stocks) — run a saved rule, or a single pattern via **Quick Scan**, across a stock list on the latest data and list every stock that qualifies today.
3. **Stock Report / Recogniser** (stock → patterns) — pick one stock and see every pattern/filter it currently passes or fails, with the evidence.

**Why it exists.** It is a **portfolio piece first** (must survive a sharp interviewer reading the code and poking the demo) and a **tool the author will use for their own trading** (must be correct on real data). The author is new to trading and fluent in Go/Kotlin/Node, not Python. Inspiration is Definedge's RZone System Builder; Kanso is its own product, not a clone.

**The story it tells:** "Noise-free breakout detection with a backtester that is honest about its assumptions."

## 2. Goals, non-goals, anti-goals

**Goals (V1)**
- One pattern done properly: **New N-year high**.
- Four on/off quality filters: **weekly close confirmation, volume surge, relative strength vs. Nifty, not extended**.
- Rules with **one level of grouping**: a list of required conditions (ALL) plus at most one "any of these" group (ANY).
- Exits inside the system: **percentage stop-loss** and **trend exit** (weekly close below an N-week moving average).
- Backtest with **next-day-open fills, costs, and slippage**, an **honesty panel**, a trade list, and click-through to the chart.
- Scanner over a stock list on the latest data.
- Stock report for one symbol with ✅/❌ and evidence per condition.
- Nightly data update for personal use; a public read-only demo on a fixed data snapshot.

**Non-goals (V1)** — deliberately later
- Tested-ceiling and long-base patterns (need swing-point detection; V2).
- Point & Figure and Renko charts/patterns (V2+).
- VCP/tightening, sector breakouts, fundamentals.
- A free-form expression box (`1 and (2 or 3)`) and nested groups.
- Futures and options trading (see §9 for the seams that keep this possible).
- Intraday data, live/streaming data, broker integration or order placement.
- User accounts, multi-user, auth.

**V2 backlog** *(from V0, author's decision 2026-11-20)*
- **Approaching breakout** watch list: stocks within a few % *below* their multi-year high (like PKScreener's scan 49).
- **Plain-English rules**: type a rule in words and get editable condition blocks (like Chartink's Magic Filters).
- **Breakouts-per-day chart**: how many stocks broke out each day, coloured by sector, to show when breakouts cluster.
- **Draw a Pattern, history search**: search every stock at every past date for the drawn shape and show what happened next (forward 20/60-day returns, counts not rates). It builds on the post-V1 Draw a Pattern page (`2026-11-20-kanso-draw-a-pattern-design.md`).
- **Delivery quantity**: NSE delivery data (shares taken into demat) as an extra "real buying" signal. Needs a second data source such as eod2/bhavcopy, with its volume re-adjusted for splits (V0 found eod2 volume unadjusted).

**Anti-goals** — failure even if it "works"
- A backtest that looks good because of **look-ahead bias** or **fantasy fills**.
- Looking like a rebrand of RZone (names, layout copied wholesale, "clone" framing).
- An app that is mostly glue around third-party trading libraries. The pattern, rule, and backtest engines are written in this repo.
- Silently hiding known biases (e.g., survivorship) from the results screen.

## 3. Constraints

- **Language:** Go for the engine and API (author's choice). Web front end in React + TypeScript.
- **Data budget:** free sources only in V1.
- **Market:** NSE equities and NSE indices, daily bars, ~2005 → today.
- **Scale target:** Nifty 500 in V1; design must not block ~5,000 instruments (25M daily rows).
- **Demo:** a recruiter must be able to use it in a browser with no install.

## 4. Approaches considered

| Approach | Trade-off | Verdict |
|---|---|---|
| **A. Engine + API service, React web app** | Two parts, but looks like a real product and each half is interview material | **Chosen** |
| B. Single-language Streamlit app (Python) | Fastest, but generic look, limited interaction, author doesn't know Python | Rejected |
| C. Fully in-browser (DuckDB-WASM, static hosting) | Free hosting, but tens of MB download and harder-to-test JS/WASM math | Rejected for V1 |

**Language for A:** Python was the default habit for finance; rejected because the reasons (pandas, vectorbt, yfinance) don't apply once the engine is hand-written and the data is small. TypeScript end-to-end was the runner-up (shared types for free). **Go chosen** by the author; type sharing is solved with code generation (§7.6). Reconfirmed in the 2026-11-20 stack review (target: full-stack product roles in fintech, plus personal use); see `docs/adr/0001`.

### 4.1 Tech stack *(stack review, 2026-11-20)*
| Layer | Choice | ADR |
|---|---|---|
| Engine + API | Go 1.27, std `net/http` + **huma v2** (OpenAPI 3.1, `/docs`), `log/slog` JSON logs | 0001, 0003 |
| Storage | **SQLite (WAL)** via pure-Go `modernc.org/sqlite`; **sqlc** queries, **goose** embedded migrations. No cgo. | 0002 |
| Export | Parquet via `parquet-go`, output only (oracle fixture, exports) | 0002 |
| Ingest | separate `cmd/ingest` process; launchd 18:30 IST + 08:00 retry | — |
| Contract | OpenAPI → `orval` (typed fetch functions + TanStack Query hooks); Evidence is a discriminated union | 0003 |
| Web | React 19, **Vite+** (Vite, Vitest, Oxlint, Oxfmt in one `vp` CLI), **TanStack Router** (typed URL state), **TanStack Query**, `useReducer` builder draft | — |
| UI | Tailwind v4, shadcn/ui, TanStack Table; dark default | — |
| Charts | TradingView Lightweight Charts for price **and** equity (attribution logo kept on) | — |
| Numbers | float64 prices/money, int64 quantities | 0004 |
| Hosting | Google Cloud Run, DB baked into image, max 1 instance | — |
| CI/CD | GitHub Actions (race tests, lint, API staleness, vitest, Playwright interviewer flow), deploy on main via Workload Identity Federation; CodeRabbit PR review | — |
| Tooling | Node 26 + pnpm, TypeScript 7, go-cmp, Makefile, gofumpt, golangci-lint, Vite+ `vp check` (Oxfmt + type-aware Oxlint + type check) | — |
| Repo | Public, MIT | — |

## 5. Domain definitions (precise)

All calculations use **split-adjusted daily OHLCV**. A bar at date *t* may only use data up to and including *t*'s close.

### 5.1 Timeframe
A system runs on either **daily** bars or **weekly** bars (built from daily: open = first open of the week, high = max, low = min, close = last close, volume = sum; week = Mon–Fri trading days, the bar is "complete" on the week's last trading day).
- **When is a week complete?** *(design call)* No holiday calendar is used. A week is complete only once the first daily bar of a **later** week exists. A weekly signal is dated on the last bar of the completed week and fills at the **open of the first bar of the next week**, which is the same bar that proves the week is complete. Truncating data mid-week therefore yields a partial week that is never evaluated, so the no-look-ahead property test holds.
- In weekly mode the scanner judges the **last completed week**. It may also show a separate, clearly labelled "provisional (week in progress)" column computed on the partial week. Provisional values never feed backtests.
- The **"weekly close confirmation"** filter in the UI is implemented as: *evaluate the entry rule on weekly bars*. When off, the entry rule is evaluated on daily bars.
- Exits are always checked on daily bars (stop-loss) except the trend exit, which is defined on weekly closes (§5.4).

### 5.2 Pattern: New N-year high
- Parameter `years` (default 3; allowed 1–20, or "all-time").
- Lookback length L = `years × 252` daily bars (or `years × 52` weekly bars).
- **Breakout level** at bar t = highest **high** of bars t−L … t−1 (excludes bar t).
- **Condition:** close(t) > level(t).
- Requires at least L bars of history before t; otherwise the condition is "not evaluable" (treated as false, shown as "insufficient history" in reports).
- **"All-time"** *(design call)* = since the instrument's first available bar, with a **minimum of 2 years** (504 daily / 104 weekly bars) of prior history. This stops newly listed stocks from flooding results.
- **The breakout level is shared** *(design call)*. `n_year_high` may appear **at most once, and only in the required (ALL) list**. It is mandatory whenever any condition in the rule requires `breakout_level`, and validation rejects any other placement. It publishes `level(t)` and `level_date(t)` into an evaluation context. Conditions that need a level (`not_extended`, scanner columns, report evidence, chart line) read it from that context. Patterns declare what they need from the context (`Requires: ["breakout_level"]`), and validation checks that something in the rule provides it. Future breakout patterns (tested ceiling, long base) become alternative providers of `breakout_level`.
- **Evidence:** level value, the date the level was set, how many bars since, close(t), % above level.

### 5.3 Filters
| Filter | Default | Definition at bar t | Evidence |
|---|---|---|---|
| Volume surge | k = 2.0, `window_weeks` = 10 (50 daily / 10 weekly bars) | volume(t) ≥ k × mean(volume over prior window bars, excluding t) | ratio |
| Relative strength | `rs_months` = 6 (126 daily / 26 weekly bars), benchmark Nifty 50 | return(stock, t−n→t) − return(benchmark, t−n→t) > 0 | both returns, difference |
| Not extended | x = 8% | close(t) ≤ level(t) × (1 + x) | % above level |
| Old high age *(V0 finding; optional, not in the default system)* | `min_age_years` = 1 (252 daily / 52 weekly bars) | bars since the level was set (t − level index) ≥ min | bars since, level date |

(Weekly close confirmation is the timeframe switch in §5.1.)

**Calendar units** *(plan grill, 2026-11-20)*: every duration param is stored in calendar units and converted to bars per timeframe, so one saved value means the same thing on daily and weekly. Daily: week = 5, month = 21, year = 252 bars. Weekly: week = 1, month = round(52/12 × months), year = 52 bars. Ranges: `window_weeks` 1–52 (whole), `rs_months` 1–24 (whole), `min_age_years` 0.25–10 (step 0.25).

**Why the old-high-age filter is optional** (V0 Day 3, Nifty 50, 2007–2026): with min = 1 year, trades were the best quality of any variation (+17.3% expectancy vs +11.1%, 45% win rate), but there were only 38 in 19 years, so a portfolio built on it sat mostly in cash (3.2% CAGR). Without it, most "new 3-year highs" are **continuation highs** in stocks that are already rising: the old high is days old, not years. So the level's age is **always shown as evidence** (Scanner column, Stock Report line), and the filter is there for users who want only base breakouts. V0 experiments also showed the volume, RS and not-extended filters changed results by under 1%/yr. **The defaults stay as specified**, so we don't overfit to one survivorship-biased sample.

### 5.4 Exits
- **Stop-loss:** x% below entry fill price (default 8%). Checked on every daily bar after entry. If low(d) ≤ stop: exit at **min(open(d), stop)** (a gap below the stop fills at the open, not the stop).
- **Trend exit:** when a **weekly** close < SMA of the last N weekly closes (default N = 30), exit at the **next trading day's open**.
- Whichever triggers first wins. Open positions at the end of data are marked to market at the last close and flagged "open".

### 5.5 Entry signals and fills
- Entry rule = ALL(required conditions) AND (ANY(group conditions) if the group exists and is non-empty).
- A **signal** fires on bar t when the entry rule is true at t **and was evaluable and false at t−1** (edge-triggered). A not-evaluable t−1 (data gap, insufficient history, benchmark not yet started) never produces a signal, so no fake "fresh" signals appear after gaps or on the first evaluable bar. *(plan grill)*
- In **single-stock mode** (trade-level stats), a signal is skipped while that stock's previous trade is open. In the **portfolio**, a signal is skipped only if the portfolio actually holds that symbol; a signal rejected for lack of slots does not hide later signals in the same stock. *(plan grill)*
- **Fill:** next trading day's **open**, plus slippage.
- **Costs:** one combined percentage per side (default 0.20% per side, covering brokerage + statutory charges) plus slippage (default 0.10% per side). Both configurable and shown in the honesty panel.

### 5.6 Portfolio simulation (equity curve)
- Starting capital default ₹10,00,000; max concurrent positions default 10; each new position gets equal-weight capital = current equity / max positions (capped by available cash).
- **Whole shares** *(plan grill)*: quantity = floor(allocation / fill price incl. costs). If that is 0 (e.g. MRF above the slot size), the trade is skipped and listed in the honesty panel. Leftover cash stays as cash.
- If more signals than free slots on a day: rank by the **6-month relative-strength difference** (desc), computed for ranking **whether or not RS is in the rule**, tie-break by symbol (asc) — deterministic. *(plan grill: otherwise rules without RS would silently rank alphabetically.)*
- **Trade-level stats** (all signals, ignoring slot limits) are reported separately from **portfolio stats** so sizing doesn't hide how the pattern itself behaves.
- Benchmark curve: buy-and-hold Nifty 50 with the same starting capital.

### 5.7 Metrics
Trade-level: number of trades, win rate, average win %, average loss %, expectancy per trade, median holding days.
Portfolio: CAGR, max drawdown, final equity, exposure (% of time invested), vs. benchmark CAGR and drawdown.

### 5.8 Honesty panel (always visible next to results)
- Fill model ("next-day open"), costs and slippage values.
- Survivorship warning when the stock list has no historical membership data (V1 case — see §6.3).
- Data source and adjustment ("split-adjusted, not dividend-adjusted").
- **Variations tested:** count of **distinct parameter sets** backtested for this system (re-running identical parameters doesn't count), with a one-line warning about overfitting when > 10. *(design call)* It is tracked client-side per system draft, saved or not, so it works on the stateless demo (§6.4).
- **Actual data window:** the real first date used. Relative strength and the benchmark can't be evaluated before the benchmark's history starts (§6.1).
- **Ranking rule:** "crowded days ranked by 6-month RS, then symbol".
- **Skipped signals:** count and reasons, e.g. "1 signal skipped: price above slot size" (one share costs more than equity ÷ max positions) or "1 signal skipped: insufficient cash" (one share fits the slot, but the cash left can't buy it).

## 6. Data

### 6.1 Source
- `Source` interface in Go; V1 implementation calls Yahoo Finance's public chart endpoint for `SYMBOL.NS` and index tickers (Nifty 50 = `^NSEI`). This endpoint is unofficial and may change or rate-limit; the loader must retry with backoff and fail loudly per symbol rather than writing partial data.
- Yahoo's chart `close`/`open`/`high`/`low` are split-adjusted (not dividend-adjusted); use those. Store the raw response's split events too.
- **Splits and bonuses on update** *(plan grill)*: Yahoo rescales all history after a split or bonus. An incremental update therefore re-fetches and replaces a symbol's **full** history when the response contains a split event in its window, **or** any of the overlapping stored closes differs from Yahoo's new value by more than 0.5% (this also catches bonuses and silent restatements).
- Later implementations (not V1): NSE end-of-day bhavcopy files, a paid vendor.
- **History start** *(design call)*: Yahoo's `^NSEI` history starts around Sept 2007, and some `.NS` stocks start late or have gaps. V1 accepts this. Backtests start from the latest of (requested start, benchmark start), and that date appears in the honesty panel. Relative strength is "not evaluable" before the benchmark's history starts. A gap of more than 5 consecutive missing trading days marks that instrument's bars across the gap as not evaluable. The "~20 years" claim becomes "since 2007" in the README.

### 6.2 Storage
- **SQLite file in WAL mode** (pure-Go driver, `docs/adr/0002`). Tables: `instruments`, `bars_daily`, `splits`, `universes`, `universe_members`, `systems`, `backtest_runs`, `ingest_log`. Schema via goose migrations; queries via sqlc.
- WAL lets the server read while the separate ingest process writes; writers use a busy timeout.
- Weekly bars are computed on load in memory, not stored.
- Parquet is an **export format only** (vectorbt fixture, data export), never read back as a source.

### 6.3 Stock lists (universes)
- `universe_members(universe_id, instrument_id, from_date, to_date NULL)`.
- V1 seeds Nifty 500 from NSE's current constituent CSV with `from_date` = unknown/NULL → the honesty panel shows the survivorship warning. Historical membership can be added later without schema changes.

### 6.4 Ingest
- `cmd/ingest` — full backfill (from 2005-01-01) and incremental daily update; idempotent; records per-symbol status in `ingest_log`.
- Personal use: `cmd/ingest -update` runs as its own process from launchd at **18:30 IST** on weekdays, and again at **08:00 IST** the next morning only if instruments are still behind the benchmark (or the benchmark has no bar for the expected day). Demo mode never updates.
- **Freshness** *(V0 finding)*: Yahoo can lag a day behind the NSE close (V0 saw Chartink with 2026-11-18 while Yahoo still ended at 2026-11-17). Ingest records each instrument's **latest bar date** and prints a summary: "data as of <benchmark's latest date>; N instruments behind". Every result screen shows **"as of <date>" taken from the data**, never from today's calendar. An instrument whose last bar is older than the benchmark's is reported as stale (scanner already excludes these).
- Demo: a frozen SQLite snapshot baked into the container image (Cloud Run, max 1 instance so the per-IP rate limit holds); the server opens it read-only.
- **Demo data scope** *(author's decision)*: the snapshot contains **only Nifty 50 constituents plus the `^NSEI` benchmark**, sourced from Yahoo. There is no bulk-download or raw-export endpoint in demo mode: `/api/bars` serves only the window needed for one chart, and responses are rate-limited. The UI footer and README credit the source and state that it's a non-commercial portfolio demo. Redistribution risk is accepted as low but nonzero, and the `Source` interface keeps the data swappable. Personal (non-demo) installs ingest the full Nifty 500.
- **Demo writes** *(design call)*: the server is stateless in demo mode. `POST/PUT/DELETE /api/systems` return 403. Visitors' saved systems and variation counts live in the browser's localStorage. Backtest, scan and report calls are computations only, so no visitor data is ever stored server-side. A per-IP rate limit on backtests prevents abuse.

## 7. Architecture

### 7.1 Overview
```
React web app (Vite, TS)  ──HTTP/JSON──>  Go API server (huma)  ──>  engine packages  ──>  SQLite (WAL)
   (embedded in the Go binary)      OpenAPI /docs                                     ^
                                    launchd ──> cmd/ingest (separate process) ────────┘ writes bars
```

### 7.2 Go packages
- `internal/market` — `Instrument` (exchange, symbol, type, lot size), `Bar`, series loading, weekly resampling.
- `internal/data` — `Source` interface, Yahoo implementation, SQLite repository (sqlc + goose).
- `internal/patterns` — each condition implements one interface:
  `Evaluate(series, t, params) -> Result{Pass bool, Evaluable bool, Evidence}` plus metadata (name, params with defaults/ranges, description) used by the UI and the report. **Evidence is typed per condition** (one struct per type, a discriminated union on `type` in the API). V1: `NYearHigh`, `VolumeSurge`, `RelativeStrength`, `NotExtended`, `LevelAge`.
- `internal/rules` — rule tree (`Required []Condition`, `AnyGroup *[]Condition`), validation, evaluation, and **explain** (which conditions passed, which ANY branch matched).
- `internal/backtest` — signal generation, `Executor` interface (V1: `EquityExecutor`), `CostModel` interface, portfolio simulation, metrics.
- `internal/scan` — run a rule over a universe at the latest (or a given) date; parallel across instruments with goroutines.
- `internal/report` — run every registered pattern with defaults (or a chosen system) on one instrument, returning evidence.
- `cmd/server` — HTTP API. `cmd/ingest` — data loader.

### 7.3 Rule tree (JSON shape, canonical in Go)
```json
{
  "name": "3y breakout, volume or RS",
  "universe": "NIFTY500",
  "timeframe": "weekly",
  "entry": {
    "all": [ {"type": "n_year_high", "params": {"years": 3}},
             {"type": "not_extended", "params": {"pct": 8}} ],
    "any": [ {"type": "volume_surge", "params": {"k": 2, "window_weeks": 10}},
             {"type": "relative_strength", "params": {"rs_months": 6, "benchmark": "^NSEI"}} ]
  },
  "exits": { "stop_pct": 8, "trend_sma_weeks": 30 },
  "costs": { "per_side_pct": 0.20, "slippage_pct": 0.10 },
  "portfolio": { "capital": 1000000, "max_positions": 10 }
}
```
Validation rejects unknown types, out-of-range params, and an `any` group with fewer than 2 conditions (a 1-item ANY is just a required condition).

### 7.4 API (JSON)
- `GET /api/patterns` — registry metadata (drives the builder UI).
- `GET /api/universes`
- `GET/POST/PUT/DELETE /api/systems` — saved systems.
- `POST /api/backtest` — body: system → results (metrics, equity + benchmark curves, trade list, honesty panel data, run id).
- `POST /api/scan` — body: system (+ optional date) → matching instruments with evidence.
- `POST /api/quickscan` — body: `{pattern, params?, universe, timeframe, date?, mode: "fresh"|"all"}` → same response shape as `/api/scan`. The server builds an ad-hoc entry rule `{all: [pattern]}` with no exits. If the pattern requires `breakout_level`, it auto-inserts `n_year_high` with default params, and the response says it did.
- `POST /api/report/{symbol}` — body: `{system?, as_of?, timeframe?}` → evidence for every condition (all registered patterns with defaults when no system is given). POST with the full system, like scan and backtest, so it works in demo mode where saved systems live only in the browser. *(plan grill)*
- `GET /openapi.json` and `GET /docs` — the API contract (huma, `docs/adr/0003`).
- `GET /api/bars/{symbol}?from&to&timeframe` — chart data; trade markers come from the backtest result.

### 7.5 Web app
Three screens (wireframes agreed in brainstorming):
1. **System Builder:** stock list picker; required conditions as plain-English blocks with editable numbers and on/off toggles; one "Match ANY of these" group; exit block; Backtest and Scan buttons; results panel with metrics, equity vs. Nifty curve, honesty panel, trade list; click a trade → chart with entry/exit markers and the breakout level line.
2. **Scanner:** pick a saved system. *(design call)* The default view is **fresh signals**: stocks whose entry rule turned false→true within the last N bars (default 5 daily / 1 weekly), matching what the backtest would have entered. A toggle shows **all currently qualifying** stocks, including ones that have been true for a while, with a "true since" date. Result is a table of matches (symbol, level, level date, **old high age**, % above, volume ratio, RS), sortable, with "as of <date>" in the header; click → Stock Report.
   **Quick Scan** (V1, author's decision): a mode on the same screen that needs no saved system. Pick any one pattern or filter from the registry list. Its default params are shown and editable. Pick a stock list and timeframe, then scan. Results use the same table. An "Open in System Builder" button turns the quick scan into a draft system, so the pattern can be backtested.
3. **Stock Report:** symbol search → chart with breakout level line → checklist of ✅/❌ per condition with evidence; "insufficient history" shown explicitly. The N-year-high line always says how old the old high is, e.g. "old high 1,240 set 2023-01-12, 1 year 10 months ago", so continuation highs are obvious.
The **System Builder starts from the default system or an empty rule, never from leftover example conditions** (V0: Chartink's page loaded example filters that silently combined with the user's rule). Results lead with **CAGR and max drawdown side by side, each next to the benchmark's**: in V0 the robust finding was risk (−27% vs −60% drawdown), not return.
Charts: TradingView Lightweight Charts for both the price chart and the equity-vs-Nifty curve, attribution logo on. Dark theme default.
Front-end shape (§4.1): React 19 SPA built by Vite and embedded in the Go binary. TanStack Router keeps scanner filters, report options and the builder draft in typed URL search params (shareable links). TanStack Query owns all server state; the builder draft is a typed `useReducer`; the server is the only validator (huma 400 field paths shown on the matching block). UI from Tailwind v4 + shadcn/ui + TanStack Table.

### 7.6 Shared types
Go handler input/output structs are the source of truth. huma generates the OpenAPI 3.1 spec; `orval` generates the TS types, typed fetch functions and TanStack Query hooks, all routed through one hand-written fetcher that throws `ApiError`. Evidence is a discriminated union on `type`. CI fails if the spec or generated client is stale.

## 8. Correctness & testing

- **Golden tests** per pattern on small hand-built series with known answers (including edge cases: exactly-equal to the level, insufficient history, gaps).
- **No-look-ahead property test:** for random t, evaluating signals on data truncated at t must give exactly the same signals up to t as evaluating on the full series.
- **Fill tests:** gap-down through stop fills at open; trend exit fills next open; edge-triggering doesn't re-enter on consecutive true bars; no signal after a not-evaluable bar; **an entry on a gap-up open still fills, with whole-share quantity computed at the fill price** (V0: TradingView silently skipped DIVISLAB's July 2026 trade at 100% sizing).
- **Numbers:** float64 throughout, int64 quantities; comparisons against the oracle use a 1e-9 relative tolerance (`docs/adr/0004`).
- **End-to-end:** one Playwright test of the interviewer flow (§10) against the demo snapshot, in CI.
- **Real-data tests from V0** (skipped unless `KANSO_DB` is set): the five verified breakouts in `docs/v0/reference-results.md` (DIVISLAB 2024-09-10 and 2026-07-14, INFY 2024-09-02, TCS 2024-02-06, HEROMOTOCO 2023-11-29), with level, level date and volume ratio. INFY must pass `n_year_high` and fail `volume_surge` (0.73×). NAVINFLUOR 2026-11-18 must fail a 1-year `level_age` filter (old high 3 bars old). **BAJFINANCE 2025-06-16** (1:2 split + 4:1 bonus) must show a 50-day volume ratio near 1×, not 6×, which proves volume is split-adjusted.
- **Determinism:** same system + same data → byte-identical results.
- **Resampling tests:** weekly bars across holidays and short weeks.
- **Real-data smoke test:** a few known NSE multi-year breakouts on dates the author picks and verifies on a chart.
- **vectorbt benchmark** *(author's decision)*: vectorbt is the independent reference ("oracle") for the Go backtester.
  - **Where it lives:** `tools/vbt-benchmark/`, a dev-only Python tool. It is never shipped, never called by the server, and the Go test suite doesn't need Python installed.
  - **Same input:** it reads a frozen data fixture exported from Kanso's SQLite store as Parquet (`testdata/benchmark/bars.parquet`, exact float64 and dates, about 10 Nifty 50 stocks plus `^NSEI`, 2007→fixed date), never yfinance, so both engines see identical bars.
  - **Golden output:** for each benchmark system (`testdata/benchmark/systems/*.json`: daily defaults, weekly defaults, no-filters, stop-only) it writes `expected_signals.csv` and `expected_trades.csv`, which are committed. A Go test (`TestBenchmarkAgainstVectorbt`) compares Kanso's output with them.
  - **Must match exactly:** signal dates and entry dates per symbol (single-stock mode, no slot limits).
  - **Must match where the semantics agree:** exit dates and exit reasons for trades whose stop wasn't gapped through.
  - **Allowed, listed differences:** the gap-through-stop fill (Kanso fills at the open per §5.4, vectorbt at the stop price) and portfolio slot ranking. Each such trade is listed in the test output with its reason. Any other mismatch fails the test.
  - **The spec wins:** vectorbt is a cross-check, not ground truth. When the two disagree on something §5 defines, Kanso follows §5 and the difference is added to the allowed list with an explanation.
  - **Regenerating:** `make benchmark` regenerates the golden files. Changes to them show up in review like any other diff.
  - **Setup traps found in V0** *(author's decision)*:
    - The tool **must pass `open`, `high` and `low` to `from_signals`**. Without them vectorbt checks stops against the close only. DIVISLAB 2020-07-02 opened at ₹2,100, below its ₹2,196 stop, and closed 20 paise above it: a −12.4% stop-out became a +41.9% winner.
    - `from_signals` has **no max-positions limit**, which is why the benchmark compares single-stock mode only.
    - The fixture must include **DIVISLAB and BAJFINANCE**. That covers the V0 trades and a split plus bonus.
    - **Settle DIVISLAB's Aug 2025 stop date**, where V0's hand calculation (2025-08-06), TradingView (08-07) and vectorbt (08-08) disagree. §5.4 decides, and the reason goes in the test's allowed-differences comments.

## 9. Extensibility seams (built now, used later)

- `Instrument.Type` (equity | index | future | option) and `LotSize`.
- `Source` interface (more exchanges/vendors), `Executor` and `CostModel` interfaces (futures: lot size, margin, roll cost), a series-builder hook for continuous futures (stitch + back-adjust).
- Pattern registry: new patterns (tested ceiling, long base, P&F) plug in without touching rules/backtest/scan/report.
- Universe membership with dates.
- Options are expected to be an *execution* choice on an underlying's signal, never a series patterns run on.

## 10. Success criteria

- **Interviewer:** opens the demo URL and, in under 2 minutes, builds a rule, backtests it, reads the honesty panel, and clicks a trade to see it on the chart. README explains the architecture and the look-ahead/fill decisions.
- **Author:** after the nightly ingest, a scan of Nifty 500 returns in under 5 seconds; stock report for one symbol under 1 second.
- **Correctness:** all §8 tests pass in CI.

## 11. Left to the builder

- Project layout details beyond the plan. (Libraries, CSS approach and hosting were settled in the 2026-11-20 stack review, §4.1.)
- Visual design details beyond the agreed wireframes.
- Pagination/virtualization of tables.
