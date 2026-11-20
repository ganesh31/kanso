"""Kanso V0: N-year-high breakout with a Kanso-style portfolio (max 10 slots, equity/10 sizing). THROWAWAY.

Usage:  python -I backtest_slots.py nifty50.csv [extra symbols...]

Step 1 uses vectorbt per stock (no shared cash) to get every trade the rule would take in single-stock mode.
Step 2 simulates the portfolio like Kanso spec §5.6: each day exits fill first, then entries ranked by
relative strength at the signal (desc), symbol (asc); at most 10 positions; each new position gets
current equity / 10 (capped by cash). Equity is marked at daily closes. Benchmark: Nifty 50 buy-and-hold.
Survivorship bias: uses TODAY's constituents.
"""
import sys
from pathlib import Path

import numpy as np
import pandas as pd
import vectorbt as vbt
import yfinance as yf

YEARS, VOL_K, VOL_WIN, RS_BARS, MAX_EXT = 3, 2.0, 50, 126, 0.08
STOP, SMA_WEEKS, COST, SLIP, SLOTS, CAPITAL = 0.08, 30, 0.002, 0.001, 10, 1_000_000
L = YEARS * 252

syms = [s.strip() + ".NS" for s in pd.read_csv(sys.argv[1])["Symbol"]] + [s + ".NS" for s in sys.argv[2:]]
raw = yf.download(syms + ["^NSEI"], start="2005-01-01", auto_adjust=False, actions=False, progress=False)
syms = [s for s in syms if raw["Close"][s].notna().sum() > L]
o, h, lo, c, v = (raw[f][syms] for f in ("Open", "High", "Low", "Close", "Volume"))
bench = raw["Close"]["^NSEI"]

level = h.rolling(L, min_periods=L).max().shift(1)
vol_ok = v >= VOL_K * v.rolling(VOL_WIN).mean().shift(1)
rs = c.pct_change(RS_BARS, fill_method=None).sub(bench.pct_change(RS_BARS, fill_method=None), axis=0)
rule = (c > level) & vol_ok & (rs > 0) & (c <= level * (1 + MAX_EXT))
signal = rule & ~rule.shift(1, fill_value=False)
entries = signal.shift(1, fill_value=False)
wc = c.resample("W-FRI").last()
exits = (wc < wc.rolling(SMA_WEEKS).mean()).reindex(c.index, method="ffill").shift(1, fill_value=False).astype(bool)

# open/high/low MUST be passed: without them vectorbt checks stops against the close only and misses
# intraday hits and gap-downs (found on DIVISLAB 2020-07-02 during V0 Day 3).
pf = vbt.Portfolio.from_signals(close=c, open=o, high=h, low=lo, entries=entries, exits=exits,
                                price=o * (1 + SLIP), sl_stop=STOP,
                                fees=COST, init_cash=CAPITAL, freq="1D")  # one independent account per stock
tr = pf.trades.records_readable.rename(columns={"Column": "sym", "Entry Timestamp": "entry", "Exit Timestamp": "exit",
                                                "Avg Entry Price": "px_in", "Avg Exit Price": "px_out", "Return": "ret"})
sig_day = {(s, d): rs.at[d0, s] for s in syms for d0, d in zip(c.index[:-1], c.index[1:]) if entries.at[d, s]}
tr["rank"] = [sig_day.get((r.sym, r.entry), 0.0) for r in tr.itertuples()]

# --- portfolio with slots -------------------------------------------------------------------------------
cash, held, taken, curve = CAPITAL, {}, [], []
by_entry = tr.groupby("entry")
by_exit = {k: g for k, g in tr.groupby("exit")}
closes = c.ffill()
for d in c.index:
    for r in (by_exit[d].itertuples() if d in by_exit else []):
        if (r.sym, r.entry) in held:
            qty = held.pop((r.sym, r.entry))
            cash += qty * r.px_out * (1 - COST)
    equity_prev = cash + sum(q * closes.at[d, s] for (s, _), q in held.items())
    if d in by_entry.groups:
        for r in by_entry.get_group(d).sort_values(["rank", "sym"], ascending=[False, True]).itertuples():
            if len(held) >= SLOTS or any(s == r.sym for s, _ in held):
                continue
            alloc = min(equity_prev / SLOTS, cash)
            if alloc <= 0:
                continue
            qty = alloc / (r.px_in * (1 + COST))
            cash -= qty * r.px_in * (1 + COST)
            held[(r.sym, r.entry)] = qty
            taken.append(r)
    curve.append((d, cash + sum(q * closes.at[d, s] for (s, _), q in held.items()), len(held)))

eq = pd.DataFrame(curve, columns=["date", "equity", "positions"]).set_index("date")
eq = eq[eq.index >= bench.first_valid_index()]
eq["equity"] = eq["equity"] / eq["equity"].iloc[0] * CAPITAL
b = bench.reindex(eq.index).ffill()
b = b / b.iloc[0] * CAPITAL
yrs = (eq.index[-1] - eq.index[0]).days / 365.25
cagr = lambda s: (s.iloc[-1] / s.iloc[0]) ** (1 / yrs) - 1
mdd = lambda s: (s / s.cummax() - 1).min()

closed = tr[tr["Status"] == "Closed"]
win = closed[closed.ret > 0]; loss = closed[closed.ret <= 0]
print(f"Stocks tested: {len(syms)}  |  window {eq.index[0].date()} → {eq.index[-1].date()} ({yrs:.1f}y)")
print(f"TRADE LEVEL (every signal, single-stock mode): {len(closed)} closed trades, win rate {len(win)/len(closed):.0%}, "
      f"avg win {win.ret.mean():+.1%}, avg loss {loss.ret.mean():+.1%}, expectancy {closed.ret.mean():+.1%}/trade")
print(f"PORTFOLIO (10 slots, equity/10): CAGR {cagr(eq.equity):.1%}, max drawdown {mdd(eq.equity):.0%}, "
      f"final ₹{eq.equity.iloc[-1]:,.0f}, trades taken {len(taken)} of {len(tr)}, "
      f"avg positions {eq.positions.mean():.1f}, invested {np.mean(eq.positions > 0):.0%} of days")
print(f"NIFTY 50 buy & hold: CAGR {cagr(b):.1%}, max drawdown {mdd(b):.0%}, final ₹{b.iloc[-1]:,.0f}")
Path("out").mkdir(exist_ok=True)
tr.to_csv("out/trades_all.csv", index=False)
pd.DataFrame([t._asdict() for t in taken]).to_csv("out/trades_taken.csv", index=False)
pd.DataFrame({"system": eq.equity, "nifty50": b}).to_csv("out/equity_slots.csv")
for s in sys.argv[2:]:
    print(f"\n{s} trades:"); print(tr[tr.sym == s + ".NS"][["entry", "px_in", "exit", "px_out", "ret"]].to_string(index=False))
