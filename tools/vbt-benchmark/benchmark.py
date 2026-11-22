"""vectorbt oracle for Kanso's backtester (spec §8).

    benchmark.py <bars.parquet> <systems_dir> <out_dir>

An independent re-implementation of the spec's §5 rules in pandas, with
vectorbt's Portfolio.from_signals doing fills, stops and slippage. It reads
the frozen fixture Kanso exported and each system in <systems_dir>, and
writes expected_signals.csv and expected_trades.csv to <out_dir>. Kanso's
TestBenchmarkAgainstVectorbt compares against them.

It imports nothing from Kanso or V0: every rule below is written from the
spec, not ported from Go.
"""

import json
import sys
from pathlib import Path

import numpy as np
import pandas as pd
import vectorbt as vbt
from numba import njit
from vectorbt.portfolio import nb as pnb
from vectorbt.portfolio.enums import StopEntryPrice, StopExitPrice

BENCH = "^NSEI"
# §6.1: more than this many consecutive benchmark trading days missing is
# a data gap.
MAX_MISSING_DAYS = 5


# ---------------------------------------------------------------- bars


def to_bars(n, unit, tf):
    """§5.3 calendar units: daily week 5, month 21, year 252 bars;
    weekly week 1, month 52/12, year 52 bars; rounded to the nearest bar."""
    per = {
        "daily": {"weeks": 5, "months": 21, "years": 252},
        "weekly": {"weeks": 1, "months": 52 / 12, "years": 52},
    }[tf][unit]
    # Go's math.Round rounds half away from zero; Python's round is banker's.
    x = n * per
    return int(np.floor(x + 0.5)) if x >= 0 else -int(np.floor(-x + 0.5))


class Bars:
    """One timeframe's bars for one instrument, as numpy arrays.

    gaps[i] counts the data gaps that end at bar i (§6.1): on daily bars,
    1 when more than MAX_MISSING_DAYS benchmark days fall strictly between
    bar i−1 and bar i; on weekly bars, the sum over the week's daily bars.
    complete is how many leading bars may be evaluated (§5.1: a week only
    once a later week exists). last_daily[i] is the daily index of bar i's
    last day.
    """

    def __init__(self, dates, o, h, l, c, v, gaps, complete, last_daily):
        self.dates, self.open, self.high, self.low, self.close, self.volume = dates, o, h, l, c, v
        self.gaps, self.complete, self.last_daily = gaps, complete, last_daily

    def __len__(self):
        return len(self.dates)

    def gap_free(self, n):
        """gap_free(n)[t]: no gap ends in bars t−n+1 … t, so bars t−n … t
        are contiguous."""
        s = pd.Series(self.gaps, dtype=float).rolling(n, min_periods=1).sum().to_numpy()
        return s == 0


def daily_bars(df, calendar):
    dates = df["date"].to_numpy()
    if calendar is None or len(dates) == 0:
        gaps = np.zeros(len(dates), dtype=int)
    else:
        prev = np.searchsorted(calendar, dates[:-1], side="right")
        cur = np.searchsorted(calendar, dates[1:], side="left")
        gaps = np.concatenate([[0], ((cur - prev) > MAX_MISSING_DAYS).astype(int)])
    n = len(dates)
    return Bars(
        dates, *(df[k].to_numpy(dtype=float) for k in ["open", "high", "low", "close", "volume"]),
        gaps, n, np.arange(n),
    )


def weekly_bars(d):
    """§5.1: ISO weeks; open = first open, high = max, low = min, close =
    last close, volume = sum, dated on the week's last trading day."""
    iso = pd.to_datetime(pd.Series(d.dates)).dt.isocalendar()
    key = (iso["year"] * 100 + iso["week"]).to_numpy()
    frame = pd.DataFrame({
        "key": key, "i": np.arange(len(d)), "open": d.open, "high": d.high, "low": d.low,
        "close": d.close, "volume": d.volume, "gaps": d.gaps,
    })
    g = frame.groupby("key", sort=False)
    w = g.agg(first=("i", "first"), last=("i", "last"), open=("open", "first"), high=("high", "max"),
              low=("low", "min"), close=("close", "last"), volume=("volume", "sum"), gaps=("gaps", "sum"))
    last = w["last"].to_numpy()
    return Bars(
        d.dates[last], w["open"].to_numpy(), w["high"].to_numpy(), w["low"].to_numpy(),
        w["close"].to_numpy(), w["volume"].to_numpy(), w["gaps"].to_numpy(),
        max(len(w) - 1, 0), last,
    ), key[last]


# ---------------------------------------------------------------- rules
# Each condition returns (pass, evaluable) boolean arrays over the bars.


def n_year_high(b, p, ctx):
    """§5.2: level(t) = max high of bars t−L … t−1; pass when close > level.
    Needs L prior bars and no gap in the window."""
    if p.get("all_time"):
        raise NotImplementedError("all_time is not in the benchmark systems")
    L = to_bars(p["years"], "years", ctx["tf"])
    level = pd.Series(b.high).shift(1).rolling(L, min_periods=L).max().to_numpy()
    ctx["level"] = level
    has = ~np.isnan(level)
    ev = has & b.gap_free(L) & (np.nan_to_num(level) > 0)
    return ev & (b.close > level), ev


def volume_surge(b, p, ctx):
    """§5.3: volume(t) ≥ k × mean volume of the prior w bars (t excluded)."""
    w = to_bars(p["window_weeks"], "weeks", ctx["tf"])
    vol = pd.Series(b.volume)
    avg = vol.shift(1).rolling(w, min_periods=w).sum().to_numpy() / w
    positive = (vol > 0).astype(float).shift(1).rolling(w, min_periods=w).sum().to_numpy()
    ev = ~np.isnan(avg) & b.gap_free(w) & (np.nan_to_num(positive) > 0) & (np.nan_to_num(avg) > 0)
    with np.errstate(divide="ignore", invalid="ignore"):
        ratio = b.volume / avg
    return ev & (ratio >= p["k"]), ev


def relative_strength(b, p, ctx):
    """§5.3: return(stock, t−n→t) − return(benchmark, t−n→t) > 0. Not
    evaluable without a benchmark bar at both ends (daily: same date;
    weekly: same ISO week, and the benchmark's week complete)."""
    if p.get("benchmark", BENCH) != BENCH:
        raise NotImplementedError("only the ^NSEI benchmark is in the fixture")
    n = to_bars(p["rs_months"], "months", ctx["tf"])
    bclose = ctx["bench_close"]  # bench close aligned to b, NaN where missing
    idx = np.arange(len(b))
    s0 = np.full(len(b), np.nan)
    b0 = np.full(len(b), np.nan)
    s0[n:], b0[n:] = b.close[:-n], bclose[:-n]
    ev = (idx >= n) & b.gap_free(n) & ~np.isnan(bclose) & ~np.isnan(b0)
    ev &= (np.nan_to_num(s0) > 0) & (np.nan_to_num(b0) > 0)
    with np.errstate(divide="ignore", invalid="ignore"):
        diff = (b.close / s0 - 1) - (bclose / b0 - 1)
    return ev & (diff > 0), ev


def not_extended(b, p, ctx):
    """§5.3: close ≤ level × (1 + x), given a breakout level."""
    level = ctx["level"]
    ev = ~np.isnan(level) & (np.nan_to_num(level) > 0)
    return ev & (b.close <= level * (1 + p["pct"] / 100)), ev


CONDITIONS = {
    "n_year_high": n_year_high,
    "volume_surge": volume_surge,
    "relative_strength": relative_strength,
    "not_extended": not_extended,
}


def entry_rule(b, entry, ctx):
    """§5.5: ALL(required) AND ANY(group). The rule is not evaluable when a
    required condition isn't, or when no ANY branch passes and one of them
    isn't evaluable. n_year_high runs first: it publishes the level."""
    alls = sorted(entry["all"], key=lambda c: c["type"] != "n_year_high")
    n = len(b)
    passed, ev = np.ones(n, bool), np.ones(n, bool)
    for c in alls:
        p, e = CONDITIONS[c["type"]](b, c.get("params", {}), ctx)
        passed &= p
        ev &= e
    if entry.get("any"):
        any_pass, any_unknown = np.zeros(n, bool), np.zeros(n, bool)
        for c in entry["any"]:
            p, e = CONDITIONS[c["type"]](b, c.get("params", {}), ctx)
            any_pass |= p
            any_unknown |= ~e
        ev &= any_pass | ~any_unknown
        passed &= any_pass
    passed &= ev
    complete = np.arange(n) < b.complete
    return passed & complete, ev & complete


def signals(passed, ev):
    """§5.5: edge-triggered; t−1 must be evaluable and false."""
    out = np.zeros(len(passed), bool)
    out[1:] = passed[1:] & ev[:-1] & ~passed[:-1]
    return out


# ---------------------------------------------------------------- trades


@njit
def single_stock_signal_nb(c, entries, trend_exits):
    """Single-stock mode (§5.5): enter only when flat; the trend exit only
    acts on a position already held."""
    if c.position_now > 0:
        return False, pnb.get_elem_nb(c, trend_exits), False, False
    return pnb.get_elem_nb(c, entries), False, False, False


def trend_exit_flags(d, n):
    """§5.4: a complete week whose close is below the SMA of the last n
    weekly closes exits at the next trading day's open; flag that day."""
    flags = np.zeros(len(d), bool)
    if n <= 0:
        return flags
    w, _ = weekly_bars(d)
    sma = pd.Series(w.close).rolling(n, min_periods=n).mean().to_numpy()
    below = np.zeros(len(w), bool)
    below[: w.complete] = (w.close < sma)[: w.complete]
    flags[w.last_daily[below] + 1] = True  # a complete week has a next bar
    return flags


def run_system(name, sys_, bars_by_symbol, calendar, start):
    tf = sys_.get("timeframe") or "daily"
    exits_ = sys_.get("exits") or {"stop_pct": 8, "trend_sma_weeks": 30}
    costs = sys_.get("costs") or {"per_side_pct": 0.20, "slippage_pct": 0.10}
    bench_d = daily_bars(bars_by_symbol[BENCH], None)
    bench_w, bench_wkey = weekly_bars(bench_d)
    sig_rows, trade_rows = [], []
    for sym in sorted(s for s in bars_by_symbol if not s.startswith("^")):
        d = daily_bars(bars_by_symbol[sym], calendar)
        if tf == "weekly":
            b, wkey = weekly_bars(d)
            pos = {k: i for i, k in enumerate(bench_wkey[: bench_w.complete])}
            bclose = np.array([bench_w.close[pos[k]] if k in pos else np.nan for k in wkey])
        else:
            b = d
            bclose = pd.Series(bench_d.close, index=bench_d.dates).reindex(d.dates).to_numpy()
        ctx = {"tf": tf, "bench_close": bclose}
        passed, ev = entry_rule(b, sys_["entry"], ctx)
        sig = np.flatnonzero(signals(passed, ev))
        sig = sig[b.dates[sig] >= start]  # §5.8: the data window starts at the benchmark's first date
        for t in sig:
            sig_rows.append((name, sym, b.dates[t]))

        # §5.5: fill at the next daily bar's open; on weekly the first bar
        # of the next week, which is the bar after the week's last day.
        entry_idx = b.last_daily[sig] + 1
        entry_idx = entry_idx[entry_idx < len(d)]
        if len(entry_idx) == 0:
            continue
        entries = np.zeros(len(d), bool)
        entries[entry_idx] = True
        trend = trend_exit_flags(d, exits_["trend_sma_weeks"])

        stop = exits_["stop_pct"] / 100
        pf = vbt.Portfolio.from_signals(
            close=d.close,
            # §8 setup trap: without open/high/low vectorbt checks stops
            # against the close only.
            open=d.open,
            high=d.high,
            low=d.low,
            price=d.open,
            signal_func_nb=single_stock_signal_nb,
            signal_args=(vbt.Rep("entries"), vbt.Rep("trend_exits")),
            broadcast_named_args={"entries": entries, "trend_exits": trend},
            size=1.0,
            init_cash=1e12,
            fees=costs["per_side_pct"] / 100,
            slippage=costs["slippage_pct"] / 100,
            sl_stop=stop if stop > 0 else np.nan,
            # §5.4: the stop is x% below the entry fill (after slippage),
            # and a stop exit pays slippage like any other fill.
            stop_entry_price=StopEntryPrice.FillPrice,
            stop_exit_price=StopExitPrice.StopMarket,
        )
        tr = pf.exit_trades.values
        for r in tr:
            e, x = int(r["entry_idx"]), int(r["exit_idx"])
            if r["status"] == 0:  # open: valued at the last close, no exit fill
                reason, exit_price = "open", d.close[-1]
            else:
                # vectorbt checks the stop before signals, so an exit on a
                # bar whose low reached the stop is a stop exit.
                hit = stop > 0 and d.low[x] <= r["entry_price"] * (1 - stop)
                reason, exit_price = ("stop" if hit else "trend"), r["exit_price"]
            trade_rows.append((name, sym, d.dates[e - 1], d.dates[e], d.dates[x] if reason != "open" else d.dates[-1],
                               reason, r["entry_price"], exit_price))
    return sig_rows, trade_rows


def main(argv):
    if len(argv) != 4:
        sys.exit(__doc__)
    bars_path, systems_dir, out_dir = Path(argv[1]), Path(argv[2]), Path(argv[3])
    df = pd.read_parquet(bars_path, engine="pyarrow")
    df = df.sort_values(["symbol", "date"], kind="stable")
    bars_by_symbol = {s: g.reset_index(drop=True) for s, g in df.groupby("symbol", sort=True)}
    calendar = bars_by_symbol[BENCH]["date"].to_numpy()
    start = calendar[0]

    sigs, trades = [], []
    for path in sorted(systems_dir.glob("*.json")):
        s, t = run_system(path.stem, json.loads(path.read_text()), bars_by_symbol, calendar, start)
        sigs += s
        trades += t
    out_dir.mkdir(parents=True, exist_ok=True)
    pd.DataFrame(sigs, columns=["system", "symbol", "signal_date"]).to_csv(out_dir / "expected_signals.csv", index=False)
    cols = ["system", "symbol", "signal_date", "entry_date", "exit_date", "exit_reason", "entry_price", "exit_price"]
    pd.DataFrame(trades, columns=cols).to_csv(out_dir / "expected_trades.csv", index=False, float_format="%.17g")
    print(f"{len(sigs)} signals, {len(trades)} trades")


if __name__ == "__main__":
    main(sys.argv)
