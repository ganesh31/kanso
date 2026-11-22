package backtest

import "kanso/internal/market"

// Honesty-panel wording (spec §5.8).
const (
	FillModel  = "next-day open"
	DataSource = "Yahoo Finance"
	Adjustment = "split-adjusted, not dividend-adjusted"
)

// Skip reasons for instruments.
const (
	SkipNoData = "no_data"
)

// SkippedInstrument is a stock list member the backtest could not use.
type SkippedInstrument struct {
	Symbol string
	Reason string
}

// Honesty is what the honesty panel shows next to the results (spec §5.8).
// The variations counter is client-side.
type Honesty struct {
	FillModel   string
	PerSidePct  float64
	SlippagePct float64
	// Survivorship is true when the stock list is today's members only.
	Survivorship bool
	DataSource   string
	Adjustment   string
	// DataStart and DataEnd are the window actually used: the start is
	// the later of the requested start and the benchmark's first date.
	DataStart market.Date
	DataEnd   market.Date
	// RankingRule says how crowded entry days are ranked.
	RankingRule string
	Skipped     []SkippedInstrument
	// SkippedSignals are portfolio candidates with a free slot that could
	// not buy one whole share, by entry date.
	SkippedSignals []SkippedSignal
}
