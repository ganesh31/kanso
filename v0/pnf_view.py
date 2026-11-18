"""Kanso V0: print a Point & Figure chart for one symbol. THROWAWAY.
Usage: python -I pnf_view.py RELIANCE.NS
"""
import sys

import yfinance as yf
from pypnf import PointFigureChart

df = yf.download(sys.argv[1], start="2010-01-01", progress=False, auto_adjust=False)
df.columns = [c[0] if isinstance(c, tuple) else c for c in df.columns]
ts = {
    "date": df.index.strftime("%Y-%m-%d").tolist(),
    "open": df["Open"].tolist(), "high": df["High"].tolist(),
    "low": df["Low"].tolist(), "close": df["Close"].tolist(),
}
chart = PointFigureChart(ts=ts, method="h/l", reversal=3, boxsize=2, scaling="log", title=sys.argv[1])
print(chart)
