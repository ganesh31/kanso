package market

import "sort"

// MarkGaps records data gaps against calendar, the benchmark's trading
// dates sorted ascending (spec §6.1). A gap is more than maxMissing
// consecutive calendar dates, within the instrument's own date range, on
// which the instrument has no bar. It is recorded at the first instrument
// bar after the missing dates, so days before listing never count.
//
// Daily series only; a weekly series takes its gaps from its daily series.
func (s *Series) MarkGaps(calendar []Date, maxMissing int) {
	s.gapPrefix = make([]int, len(s.Bars)+1)
	for i := range s.Bars {
		gap := 0
		if i > 0 && missingBetween(calendar, s.Bars[i-1].Date, s.Bars[i].Date) > maxMissing {
			gap = 1
		}
		s.gapPrefix[i+1] = s.gapPrefix[i] + gap
	}
}

// missingBetween counts calendar dates strictly between a and b.
func missingBetween(calendar []Date, a, b Date) int {
	lo := sort.Search(len(calendar), func(i int) bool { return calendar[i] > a })
	hi := sort.Search(len(calendar), func(i int) bool { return calendar[i] >= b })
	return max(hi-lo, 0)
}

// GapsIn counts the gaps recorded at bars from+1 … to, in O(1): the gaps
// inside a window that starts at bar from. It is 0 before MarkGaps runs.
func (s *Series) GapsIn(from, to int) int {
	if s.gapPrefix == nil {
		return 0
	}
	from = min(max(from, -1), len(s.Bars)-1)
	to = min(max(to, -1), len(s.Bars)-1)
	if to <= from {
		return 0
	}
	return s.gapPrefix[to+1] - s.gapPrefix[from+1]
}
