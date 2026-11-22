# vectorbt benchmark (oracle)

A dev-only cross-check of Kanso's backtester (spec §8). `benchmark.py` re-implements the spec's §5 rules in pandas and uses vectorbt's `Portfolio.from_signals` for fills, stops and slippage. It imports nothing from Kanso or `v0/`. It is never shipped and the server never calls it; the Go tests read only its committed output.

## What it reads and writes

- Input: `testdata/benchmark/bars.parquet`, exported from Kanso's SQLite store by `cmd/export-fixture` (13 Nifty stocks, including DIVISLAB and BAJFINANCE, plus `^NSEI`, 2005-01-01 to 2026-11-20), and the systems in `testdata/benchmark/systems/*.json` (daily defaults, weekly defaults, no filters, stop only).
- Output: `testdata/benchmark/expected_signals.csv` and `expected_trades.csv`.

`TestBenchmarkAgainstVectorbt` in `internal/backtest` runs Kanso on the same fixture in single-stock mode and compares. Signal and entry dates must match exactly, exit dates and reasons must be equal, and prices must agree within 1e-9 relative (ADR 0004). Differences the spec explains are listed in `allowedDiffs` with the § that justifies them.

## Regenerating

```sh
make benchmark KANSO_DB=kanso.db
```

The database needs every symbol in the Makefile's `BENCH_SYMBOLS`. DIVISLAB and HEROMOTOCO are no longer in the Nifty 50, so ingest them into a copy of the database first:

```sh
printf 'Symbol\nDIVISLAB\nHEROMOTOCO\n' > extra.csv
go run ./cmd/ingest -db copy.db -universe BENCH_EXTRA -constituents extra.csv -to 2026-11-20
```

Python and its packages are pinned in `pyproject.toml` and `uv.lock`; `uv` installs them into `tools/vbt-benchmark/.venv` (gitignored).

## Setup choices

- `open`, `high` and `low` are passed to `from_signals`. Without them vectorbt checks stops against the close only (the V0 trap: DIVISLAB 2020-07-02).
- The stop is measured from the entry fill after slippage (`stop_entry_price=FillPrice`), and a stop exit pays slippage (`stop_exit_price=StopMarket`). A day that opens below the stop fills at the open.
- A small `signal_func_nb` enters only when flat and applies the weekly trend exit only to a position already held, which is single-stock mode (§5.5).
- vectorbt places one order per bar and checks a stop before signals, from the bar after the entry. So it differs from §5.4 when a trend exit and a stop fall on the same day, and when the stop is hit on the entry day. Those trades are in `allowedDiffs`.
