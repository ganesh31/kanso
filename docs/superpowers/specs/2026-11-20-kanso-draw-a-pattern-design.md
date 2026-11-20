# Kanso: Draw a Pattern — Design (short)

Status: draft, 2026-11-20. Add-on to the V1 design (`2026-11-19-kanso-breakout-scanner-design.md`). It is built **after V1 ships** (author's decision).

## Intent
You sketch a price shape with the mouse, and Kanso lists the stocks whose recent chart looks most like it, each with a match %. This is a **show piece** for the demo. The author knows the trading value is low. Prior art: Downstox's "Flight plans: draw a flight path" panel on its breakouts page.

## Goals / non-goals / anti-goals
- **Goal:** draw → ranked matches in under a second on the demo's Nifty 50, each match drawn over your sketch so the % is believable at a glance.
- **Non-goals (now):** searching all of history, "what happened next" returns, using a drawn pattern as a rule condition in the System Builder, weekly bars, intraday.
- **Anti-goals:** dressing a weak match up as a strong one; any hint that a match is a buy signal. The page says plainly that it matches shape only and predicts nothing.

## Approaches considered
| Approach | Trade-off | Decision |
|---|---|---|
| 1. Shape correlation over the latest window | Small, unit-testable, and the % is easy to explain ("how closely the lines move together"). Strict about timing. | **Chosen now** |
| 2. Dynamic time warping | Forgives a sketch whose dip is in the wrong spot, but the % is harder to explain, it can match shapes that look clearly wrong, and it needs tuning | Rejected |
| 3. Search all of history + show what happened next | Most honest and most interesting, but a much bigger job (sliding search, overlapping matches, look-ahead guards) and small samples on Nifty 50 | **Later** (author's decision). Added to the V2 backlog |

## Design
- **Window:** 1, 3 or 6 months (21 / 63 / 126 daily bars), default 3 months. *(my call)*
- **Sketch input:** a canvas the width of the window. As you drag, each x column keeps the last y drawn there, so the sketch is one value per x. A sketch has to span at least 80% of the width; gaps are filled by straight lines between drawn points. Clear and redraw are allowed. *(my call)*
- **Matching (Go, `internal/shape`):**
  1. Resample both the sketch and the stock's daily **closes** over the window to N = 64 points by linear interpolation.
  2. Z-normalise each (subtract the mean, divide by the standard deviation) so only the shape counts.
  3. Score = Pearson correlation r. Match % = `round(max(0, r) × 100)`.
  4. A flat sketch (std ≈ 0) is rejected with a clear message. A stock is skipped if its std ≈ 0.
- **Which stocks:** a stock list (universe) from `/api/universes`; the demo has Nifty 50 only. A stock is excluded if it has fewer bars than the window, or if it is **stale** (its last bar is older than the benchmark's), using the same freshness rule as the V1 scanner.
- **API:** `POST /api/shape-match` with body `{points: number[] (≥ 2, sketch y top-down already flipped to price-up), window: "1m"|"3m"|"6m", universe: string, limit?: number (default 10, max 25)}`. It returns `{as_of, window_bars, matches: [{symbol, name, score, closes: number[], dates: [from, to]}]}`, sorted by score descending. Matches scoring 0 are left out.
- **Page:** a 4th screen called "Draw a Pattern":
  - Top: the drawing canvas, plus the window picker, stock-list picker and Find button.
  - Below: result cards. Each card shows the symbol, the match % and a mini chart with the stock's normalised line drawn over your sketch.
  - Clicking a card opens the Stock Report.
  - The header says "as of <date>". A one-line note says: "Shape match only; past shape says nothing about what comes next."
- **Labels:** match % shows as "Strong" at 80% or more, "Loose" from 50 to 79%, and "Weak" below 50%. Weak results stay visible but greyed out, so a poor best match looks poor. *(my call)*

## Testing
- Unit tests for resampling (lengths, endpoints), z-normalisation, and the score:
  - a sketch identical to a series → 100%
  - the inverted series → 0%
  - a scaled or shifted copy → 100%
  - a flat sketch → error
- Endpoint test against a small fixture universe: ordering, `limit`, and the stale/short-history exclusions.

## Left to the builder
Canvas implementation details, card layout and styling, and whether the mini charts use Lightweight Charts or plain SVG.
