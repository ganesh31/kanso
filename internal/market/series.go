package market

// Timeframe is the bar size a series or system runs on.
type Timeframe string

// Timeframes. Weekly bars are built from daily bars on load (spec §5.1).
const (
	Daily  Timeframe = "daily"
	Weekly Timeframe = "weekly"
)

// Series is one instrument's bars on one timeframe.
type Series struct {
	Instrument Instrument
	Timeframe  Timeframe
	Bars       []Bar
	// Complete is the number of complete bars. Only bars with index
	// < Complete may be evaluated. For daily series it is len(Bars).
	Complete int
	// LastDailyIdx[i] is the index of bar i's last daily bar; nil for daily.
	LastDailyIdx []int
	// gapPrefix[i] is the number of data gaps recorded at bars 0..i-1;
	// nil until MarkGaps runs.
	gapPrefix []int
}

// NewDaily wraps daily bars (sorted ascending by date) in a Series.
func NewDaily(inst Instrument, bars []Bar) *Series {
	return &Series{Instrument: inst, Timeframe: Daily, Bars: bars, Complete: len(bars)}
}

// IndexAtOrBefore returns the index of the last bar dated on or before d,
// or -1 if every bar is after d.
func (s *Series) IndexAtOrBefore(d Date) int {
	lo, hi := 0, len(s.Bars) // first index with Date > d
	for lo < hi {
		mid := (lo + hi) / 2
		if s.Bars[mid].Date <= d {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo - 1
}
