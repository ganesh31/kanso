package market

// ToWeekly resamples a daily series into ISO-week bars (spec §5.1). A
// weekly bar opens at its first daily open, closes at its last daily
// close, spans the max high and min low, and sums the volume; it is dated
// by its last daily bar. The last week is complete only once a later
// week's first bar exists, so Complete is len(Bars)-1 when there are bars.
func ToWeekly(daily *Series) *Series {
	w := &Series{Instrument: daily.Instrument, Timeframe: Weekly, Bars: []Bar{}, LastDailyIdx: []int{}}
	var prevY, prevW int
	for i, b := range daily.Bars {
		y, wk := b.Date.ISOWeek()
		n := len(w.Bars)
		if n == 0 || y != prevY || wk != prevW {
			w.Bars = append(w.Bars, b)
			w.LastDailyIdx = append(w.LastDailyIdx, i)
			prevY, prevW = y, wk
			continue
		}
		cur := &w.Bars[n-1]
		cur.Date = b.Date
		cur.High = max(cur.High, b.High)
		cur.Low = min(cur.Low, b.Low)
		cur.Close = b.Close
		cur.Volume += b.Volume
		w.LastDailyIdx[n-1] = i
	}
	w.Complete = max(len(w.Bars)-1, 0)
	return w
}
