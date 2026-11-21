package backtest

import (
	"cmp"
	"slices"

	"kanso/internal/market"
	"kanso/internal/rules"
)

// RankingRule is the honesty-panel wording for crowded days (spec §5.8).
const RankingRule = "crowded days ranked by 6-month RS, then symbol"

// Skip reasons for signals.
const (
	// SkipPriceAboveSlot: one share costs more than the slot (spec §5.6).
	SkipPriceAboveSlot = "price_above_slot"
	// SkipInsufficientCash: one share fits the slot, but the cash left
	// (slots are capped by cash, spec §5.6) can't buy it.
	SkipInsufficientCash = "insufficient_cash"
)

// EquityPoint is a curve's value at one close.
type EquityPoint struct {
	Date  market.Date
	Value float64
}

// SkippedSignal is a portfolio candidate that had a free slot but could
// not buy one whole share.
type SkippedSignal struct {
	Symbol    string
	EntryDate market.Date
	Reason    string
}

// Simulation is the portfolio's side of a run.
type Simulation struct {
	// Taken are the trades the portfolio held, by entry date then rank,
	// with Quantity set.
	Taken []Trade
	// Equity is cash plus positions marked at each calendar date's close.
	Equity  []EquityPoint
	Skipped []SkippedSignal
	// Exposure is the fraction of calendar dates that close with at least
	// one position held, 0–1.
	Exposure float64
}

// position is a trade the portfolio holds.
type position struct {
	tr Trade
	// last is the index into the symbol's closes of the latest close on or
	// before the current date, or -1.
	last int
}

// Simulate runs the portfolio over the candidates (a trade path for every
// signal) on calendar, ascending dates (spec §5.6). On each date:
//
//   - Exits dated that day fill first and release their slots and cash.
//   - Entries dated that day are ranked by RankKey (descending) then symbol
//     (ascending). A candidate is passed over while the portfolio holds its
//     symbol or no slot is free. Otherwise it gets min(equity ÷
//     MaxPositions, cash), equity marked at the previous close, and buys
//     whole shares at its fill price plus the buy cost. Zero shares is
//     recorded as a skipped signal and the slot stays free.
//   - A trade that exits on its entry day leaves straight after entering.
//   - Equity is marked at the close; a symbol with no bar that day keeps
//     its last close. Trades still open at the end of data stay held.
//
// Candidates must use dates on the calendar.
func Simulate(cands []Trade, closes map[string]*market.Series, calendar []market.Date, p rules.Portfolio, ex Executor, cm CostModel) Simulation {
	order := slices.Clone(cands)
	slices.SortStableFunc(order, func(a, b Trade) int {
		return cmp.Or(
			cmp.Compare(a.EntryDate, b.EntryDate),
			cmp.Compare(b.RankKey, a.RankKey),
			cmp.Compare(a.Symbol, b.Symbol),
			cmp.Compare(a.SignalDate, b.SignalDate),
		)
	})

	sim := Simulation{Taken: []Trade{}, Equity: make([]EquityPoint, 0, len(calendar)), Skipped: []SkippedSignal{}}
	cash, prevEquity := p.Capital, p.Capital
	var held []position
	next, invested := 0, 0
	sell := func(pos position) {
		notional := float64(pos.tr.Quantity) * pos.tr.ExitPrice
		cash += notional - cm.Cost(notional, Sell)
	}
	exitDue := func(d market.Date) {
		kept := held[:0]
		for _, pos := range held {
			if pos.tr.ExitDate == d && pos.tr.ExitReason != ExitOpen {
				sell(pos)
				continue
			}
			kept = append(kept, pos)
		}
		held = kept
	}
	holds := func(sym string) bool {
		return slices.ContainsFunc(held, func(pos position) bool { return pos.tr.Symbol == sym })
	}

	for _, d := range calendar {
		for next < len(order) && order[next].EntryDate < d {
			next++ // not on the calendar
		}
		exitDue(d)
		for ; next < len(order) && order[next].EntryDate == d; next++ {
			tr := order[next]
			if len(held) >= p.MaxPositions || holds(tr.Symbol) {
				continue
			}
			slot := prevEquity / float64(p.MaxPositions)
			perShare := tr.EntryPrice + cm.Cost(tr.EntryPrice, Buy)
			inst := closes[tr.Symbol].Instrument
			qty := ex.Quantity(min(slot, cash), perShare, inst)
			if qty == 0 {
				reason := SkipInsufficientCash
				if ex.Quantity(slot, perShare, inst) == 0 {
					reason = SkipPriceAboveSlot
				}
				sim.Skipped = append(sim.Skipped, SkippedSignal{Symbol: tr.Symbol, EntryDate: tr.EntryDate, Reason: reason})
				continue
			}
			tr.Quantity = qty
			notional := float64(qty) * tr.EntryPrice
			cash -= notional + cm.Cost(notional, Buy)
			held = append(held, position{tr: tr, last: -1})
			sim.Taken = append(sim.Taken, tr)
		}
		exitDue(d)

		equity := cash
		for i := range held {
			pos := &held[i]
			s := closes[pos.tr.Symbol]
			for pos.last+1 < len(s.Bars) && s.Bars[pos.last+1].Date <= d {
				pos.last++
			}
			px := pos.tr.EntryPrice
			if pos.last >= 0 {
				px = s.Bars[pos.last].Close
			}
			equity += float64(pos.tr.Quantity) * px
		}
		if len(held) > 0 {
			invested++
		}
		sim.Equity = append(sim.Equity, EquityPoint{Date: d, Value: equity})
		prevEquity = equity
	}
	if len(calendar) > 0 {
		sim.Exposure = float64(invested) / float64(len(calendar))
	}
	return sim
}
