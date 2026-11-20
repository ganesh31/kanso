package market

// Bar is one split-adjusted OHLCV bar.
type Bar struct {
	Date   Date    `json:"date"`
	Open   float64 `json:"open"`
	High   float64 `json:"high"`
	Low    float64 `json:"low"`
	Close  float64 `json:"close"`
	Volume float64 `json:"volume"`
}
