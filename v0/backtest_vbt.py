"""Kanso V0: honest multi-stock backtest of the N-year-high breakout (daily mode). THROWAWAY.

Usage:  python -I backtest_vbt.py nifty50.csv [years]
  nifty50.csv = niftyindices.com constituents CSV (needs a 'Symbol' column).

Honest parts: the signal is computed at the close; entry at the NEXT day's open; trend exit at the open after the
completed week; 0.30% cost per side; equal-weight slots (10) with shared cash.
Limitations vs the Kanso spec: vectorbt's stop fills at the stop price (no gap-to-open handling);
no RS-based slot ranking; uses TODAY's constituents (survivorship bias); Yahoo data.
"""
import sys
from pathlib import Path

import pandas as pd
import vectorbt as vbt
import yfinance as yf

YEARS = int(sys.argv[2]) if len(sys.argv) > 2 else 3
L = YEARS * 252
VOL_K, VOL_WIN, RS_BARS, MAX_EXT = 2.0, 50, 126, 0.08
STOP, SMA_WEEKS, COST, SLOTS = 0.08, 30, 0.003, 10

symbols = [s.strip() + ".NS" for s in pd.read_csv(sys.argv[1])["Symbol"]]
raw = yf.download(symbols + ["^NSEI"], start="2005-01-01", auto_adjust=False, actions=False, progress=False)
o, h, lo, c, v = (raw[f][symbols] for f in ("Open", "High", "Low", "Close", "Volume"))
bench = raw["Close"]["^NSEI"]

level = h.rolling(L, min_periods=L).max().shift(1)
vol_ok = v >= VOL_K * v.rolling(VOL_WIN).mean().shift(1)
rs = c.pct_change(RS_BARS).sub(bench.pct_change(RS_BARS), axis=0)
rule = (c > level) & vol_ok & (rs > 0) & (c <= level * (1 + MAX_EXT))
signal = rule & ~rule.shift(1, fill_value=False)
entries = signal.shift(1, fill_value=False)  # act at next day's open

wc = c.resample("W-FRI").last()
w_exit = (wc < wc.rolling(SMA_WEEKS).mean()).reindex(c.index, method="ffill").shift(1, fill_value=False)
exits = w_exit.astype(bool)

pf = vbt.Portfolio.from_signals(
    # open/high/low are required for intraday stop checks (see backtest_slots.py). This script does NOT enforce
    # the 10-slot limit (up to 31 positions were open at once); use backtest_slots.py for Kanso-style results.
    close=c, open=o, high=h, low=lo, entries=entries, exits=exits, price=o, sl_stop=STOP,
    # from_signals can't size by % of equity; use a fixed slice of the starting capital (Kanso: equity / slots).
    fees=COST, size=1_000_000 / SLOTS, size_type="value",
    init_cash=1_000_000, cash_sharing=True, group_by=True, freq="1D",
)

out = Path("out"); out.mkdir(exist_ok=True)
trades = pf.trades.records_readable
trades.to_csv(out / "trades.csv", index=False)
print(f"Lookback {YEARS}y | trades {len(trades)} | win rate {pf.trades.win_rate():.1%}")
print(pf.stats())
pf.plot().write_html(out / "equity.html")
print("Saved out/trades.csv and out/equity.html")
