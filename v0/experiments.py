"""Kanso V0: run many variations of the breakout system on the same data and compare. THROWAWAY.

Usage:  python -I experiments.py nifty50.csv [extra symbols...]
Same engine as backtest_slots.py (vectorbt single-stock trades + Kanso-style 10-slot portfolio), but
parameterised. Market data is cached in out/raw.pkl so every variation sees identical bars.
"""
import sys
from pathlib import Path

import numpy as np
import pandas as pd
import vectorbt as vbt
import yfinance as yf
from numpy.lib.stride_tricks import sliding_window_view

BASE = dict(years=3, vol=True, rs=True, ext=True, stop=0.08, sma=30, min_age=0)
VARIANTS = [
    ("Baseline (V1 defaults)", {}),
    ("No filters (3y high only)", dict(vol=False, rs=False, ext=False)),
    ("Without volume filter", dict(vol=False)),
    ("Without RS filter", dict(rs=False)),
    ("Without not-extended filter", dict(ext=False)),
    ("1-year high (52-week)", dict(years=1)),
    ("5-year high", dict(years=5)),
    ("Old high ≥ 1 year old", dict(min_age=252)),
    ("Stop 15% (looser)", dict(stop=0.15)),
    ("Trend exit 40 weeks (looser)", dict(sma=40)),
    ("No trend exit (stop only)", dict(sma=0)),
]
COST, SLIP, SLOTS, CAPITAL, VOL_K, VOL_WIN, RS_BARS, MAX_EXT = 0.002, 0.001, 10, 1_000_000, 2.0, 50, 126, 0.08

out = Path("out"); out.mkdir(exist_ok=True)
cache = out / "raw.pkl"
syms = [s.strip() + ".NS" for s in pd.read_csv(sys.argv[1])["Symbol"]] + [s + ".NS" for s in sys.argv[2:]]
if cache.exists():
    raw = pd.read_pickle(cache)
else:
    raw = yf.download(syms + ["^NSEI"], start="2005-01-01", auto_adjust=False, actions=False, progress=False)
    raw.to_pickle(cache)
syms = [s for s in syms if raw["Close"][s].notna().sum() > 252 * 5]
o, h, lo, c, v = (raw[f][syms] for f in ("Open", "High", "Low", "Close", "Volume"))
bench = raw["Close"]["^NSEI"]
closes = c.ffill()
rs_all = c.pct_change(RS_BARS, fill_method=None).sub(bench.pct_change(RS_BARS, fill_method=None), axis=0)
vol_ok = v >= VOL_K * v.rolling(VOL_WIN).mean().shift(1)


def level_and_age(L):
    """Highest high of the prior L bars, and how many bars ago that high was set."""
    lvl = h.rolling(L, min_periods=L).max().shift(1)
    age = pd.DataFrame(np.nan, index=h.index, columns=h.columns)
    for s in h.columns:
        x = h[s].to_numpy()
        if len(x) <= L:
            continue
        win = sliding_window_view(np.nan_to_num(x, nan=-np.inf), L)[:-1]  # windows ending at t-1
        # argmax on the reversed window = offset of the most recent bar holding the max (0 = bar t-1)
        age.iloc[L:, age.columns.get_loc(s)] = np.argmax(win[:, ::-1], axis=1) + 1
    return lvl, age


def run(p):
    L = p["years"] * 252
    lvl, age = level_and_age(L)
    rule = c > lvl
    if p["vol"]: rule &= vol_ok
    if p["rs"]: rule &= rs_all > 0
    if p["ext"]: rule &= c <= lvl * (1 + MAX_EXT)
    if p["min_age"]: rule &= age >= p["min_age"]
    signal = rule & ~rule.shift(1, fill_value=False)
    entries = signal.shift(1, fill_value=False)
    if p["sma"]:
        wc = c.resample("W-FRI").last()
        exits = (wc < wc.rolling(p["sma"]).mean()).reindex(c.index, method="ffill").shift(1, fill_value=False).astype(bool)
    else:
        exits = pd.DataFrame(False, index=c.index, columns=c.columns)
    pf = vbt.Portfolio.from_signals(close=c, open=o, high=h, low=lo, entries=entries, exits=exits,
                                    price=o * (1 + SLIP), sl_stop=p["stop"], fees=COST, init_cash=CAPITAL, freq="1D")
    tr = pf.trades.records_readable.rename(columns={"Column": "sym", "Entry Timestamp": "entry", "Exit Timestamp": "exit",
                                                    "Avg Entry Price": "px_in", "Avg Exit Price": "px_out", "Return": "ret"})
    prev = dict(zip(c.index[1:], c.index[:-1]))
    tr["rank"] = [rs_all.at[prev[r.entry], r.sym] if r.entry in prev else 0.0 for r in tr.itertuples()]
    tr["rank"] = tr["rank"].fillna(0.0)
    cash, held, curve, taken = CAPITAL, {}, [], 0
    by_entry = {k: g.sort_values(["rank", "sym"], ascending=[False, True]) for k, g in tr.groupby("entry")}
    by_exit = {k: g for k, g in tr.groupby("exit")}
    for d in c.index:
        for r in (by_exit[d].itertuples() if d in by_exit else []):
            q = held.pop((r.sym, r.entry), None)
            if q is not None:
                cash += q * r.px_out * (1 - COST)
        eq_prev = cash + sum(q * closes.at[d, s] for (s, _), q in held.items())
        for r in (by_entry[d].itertuples() if d in by_entry else []):
            if len(held) >= SLOTS or any(s == r.sym for s, _ in held):
                continue
            alloc = min(eq_prev / SLOTS, cash)
            q = alloc / (r.px_in * (1 + COST)); cash -= alloc; held[(r.sym, r.entry)] = q; taken += 1
        curve.append(cash + sum(q * closes.at[d, s] for (s, _), q in held.items()))
    eq = pd.Series(curve, index=c.index)
    eq = eq[eq.index >= bench.first_valid_index()]
    yrs = (eq.index[-1] - eq.index[0]).days / 365.25
    closed = tr[tr["Status"] == "Closed"]
    w = closed[closed.ret > 0]; l = closed[closed.ret <= 0]
    return dict(trades=len(closed), win=len(w) / max(len(closed), 1), avg_win=w.ret.mean(), avg_loss=l.ret.mean(),
                exp=closed.ret.mean(), cagr=(eq.iloc[-1] / eq.iloc[0]) ** (1 / yrs) - 1, mdd=(eq / eq.cummax() - 1).min(),
                taken=taken)


b = bench.dropna(); yrs = (b.index[-1] - b.index[0]).days / 365.25
print(f"{len(syms)} stocks, {b.index[0].date()} → {b.index[-1].date()}. Nifty 50 buy & hold: "
      f"CAGR {(b.iloc[-1]/b.iloc[0])**(1/yrs)-1:.1%}, max DD {(b/b.cummax()-1).min():.0%}\n")
rows = []
for name, change in VARIANTS:
    r = run({**BASE, **change}); r["variant"] = name; rows.append(r)
    print(f"{name:30s} trades {r['trades']:4d}  win {r['win']:4.0%}  avg win {r['avg_win']:+6.1%}  avg loss {r['avg_loss']:+6.1%}  "
          f"exp/trade {r['exp']:+6.1%}  | portfolio CAGR {r['cagr']:5.1%}  maxDD {r['mdd']:4.0%}  taken {r['taken']}", flush=True)
pd.DataFrame(rows).to_csv(out / "experiments.csv", index=False)
