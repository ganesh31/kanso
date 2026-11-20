package market

// InstrumentType says what kind of instrument a symbol is.
type InstrumentType string

// Instrument types. V1 trades equities and uses indices as benchmarks;
// futures and options are seams for later (spec §9).
const (
	Equity InstrumentType = "equity"
	Index  InstrumentType = "index"
	Future InstrumentType = "future"
	Option InstrumentType = "option"
)

// Instrument is one tradable or reference symbol.
type Instrument struct {
	ID       int64          `json:"id"`
	Exchange string         `json:"exchange"`
	Symbol   string         `json:"symbol"`
	Type     InstrumentType `json:"type"`
	LotSize  int            `json:"lot_size"`
}
