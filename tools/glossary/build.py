"""Build docs/v0/glossary.html (the hover-popup glossary page) from docs/v0/glossary.md.

Usage: python3 -I tools/glossary/build.py
The page template is tools/glossary/template.html; the terms are injected as JSON.
"""
import html
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SRC = ROOT / "docs/v0/glossary.md"
TPL = Path(__file__).with_name("template.html")
OUT = ROOT / "docs/v0/glossary.html"

# Aliases too common to auto-link inside other definitions (they'd link everywhere).
NOLINK = {"open", "close", "high", "low", "bar", "return", "entry", "exit", "fill", "index", "stock",
          "share", "equity", "position", "long", "short", "symbol", "ticker", "system", "strategy",
          "sector", "broker", "leader", "laggard", "trading day", "volatility", "benchmark", "options",
          "futures", "call", "put", "margin", "leverage", "expiry", "rollover", "intraday", "line chart",
          "liquidity", "fill price", "market order", "tradingview", "chartink", "pkscreener", "vectorbt"}


def slug(s: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", s.lower()).strip("-")


def inline(md: str) -> str:
    """Escape, then apply the small subset of markdown the glossary uses."""
    s = html.escape(md, quote=False)
    s = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", s)
    s = re.sub(r"(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)", r"<em>\1</em>", s)
    s = re.sub(r"`(.+?)`", r"<code>\1</code>", s)
    return s


def aliases(term: str) -> list[str]:
    out = []
    for part in re.split(r"\s*/\s*", term):
        m = re.match(r"^(.*?)\s*\((.+)\)\s*$", part)
        if m:
            out += [m.group(1), m.group(2)]
        else:
            out.append(part)
    if ":" in term:  # "Options: call / put"
        head, rest = term.split(":", 1)
        out = [head] + [a.strip() for a in re.split(r"\s*/\s*", rest)]
    return [a.strip() for a in out if a.strip()]


terms, sections, section = [], [], None
for line in SRC.read_text().splitlines():
    if m := re.match(r"^## (\d+)\. (.+)$", line):
        section = m.group(2)
        sections.append(section)
        continue
    m = re.match(r"^\| \*\*(.+?)\*\* \| (.+) \|$", line)
    if not m or section is None:
        continue
    term, defn = m.group(1), m.group(2)
    v1 = "🔷" in defn
    defn = defn.replace("🔷", "").strip()
    al = aliases(term)
    terms.append({
        "id": slug(al[0] if al else term),
        "term": term,
        "aliases": al,
        "link": [a for a in al if a.lower() not in NOLINK and len(a) >= 2],
        "section": section,
        "v1": v1,
        "def": inline(defn),
    })

# An alias links to the first term that claims it (e.g. "P&F" → Point & Figure, not Double top buy).
seen = set()
for t in terms:
    t["link"] = [a for a in t["link"] if a.lower() not in seen and a.lower() != "kanso"]
    seen.update(a.lower() for a in t["link"])

ids = [t["id"] for t in terms]
dupes = {i for i in ids if ids.count(i) > 1}
for t in terms:
    if t["id"] in dupes:
        t["id"] = slug(t["term"])

data = json.dumps({"sections": sections, "terms": terms}, ensure_ascii=False)
OUT.write_text(TPL.read_text().replace("/*__DATA__*/null", data))
print(f"{len(terms)} terms in {len(sections)} sections → {OUT.relative_to(ROOT)}")
