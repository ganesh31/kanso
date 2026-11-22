package market

import (
	"testing"
	"time"
)

// calendar returns n consecutive benchmark dates from 2024-01-01.
func calendar(n int) []Date {
	out := make([]Date, n)
	for i := range out {
		out[i] = Date(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(DateLayout))
	}
	return out
}

// seriesOn builds a daily series with one bar on each of the given
// calendar days.
func seriesOn(cal []Date, days ...int) *Series {
	bars := make([]Bar, len(days))
	for i, d := range days {
		bars[i] = Bar{Date: cal[d], Close: 1}
	}
	return NewDaily(Instrument{Symbol: "TEST"}, bars)
}

// daysExcept returns 0..n-1 without the days in [lo, hi].
func daysExcept(n, lo, hi int) []int {
	var out []int
	for d := range n {
		if d < lo || d > hi {
			out = append(out, d)
		}
	}
	return out
}

func TestMarkGapsSixMissingDaysIsGap(t *testing.T) {
	cal := calendar(20)
	s := seriesOn(cal, daysExcept(20, 8, 13)...)
	s.MarkGaps(cal, 5)
	if got := s.GapsIn(0, len(s.Bars)-1); got != 1 {
		t.Fatalf("GapsIn = %d, want 1", got)
	}
}

func TestMarkGapsFiveMissingDaysIsNotGap(t *testing.T) {
	cal := calendar(20)
	s := seriesOn(cal, daysExcept(20, 8, 12)...)
	s.MarkGaps(cal, 5)
	if got := s.GapsIn(0, len(s.Bars)-1); got != 0 {
		t.Fatalf("GapsIn = %d, want 0", got)
	}
}

func TestMarkGapsIgnoresDaysBeforeListing(t *testing.T) {
	cal := calendar(20)
	s := seriesOn(cal, daysExcept(20, 0, 9)...) // listed on calendar day 10
	s.MarkGaps(cal, 5)
	if got := s.GapsIn(0, len(s.Bars)-1); got != 0 {
		t.Fatalf("GapsIn = %d, want 0", got)
	}
}

func TestGapsInWindowExcludesOlderGap(t *testing.T) {
	cal := calendar(27)
	// Bars on days 0..6, then a 6-day gap, then days 13..26: the gap is
	// recorded at bar 7 (day 13), and the last bar is index 20.
	s := seriesOn(cal, daysExcept(27, 7, 12)...)
	s.MarkGaps(cal, 5)
	if got := s.GapsIn(7, 20); got != 0 {
		t.Fatalf("GapsIn(7, 20) = %d, want 0", got)
	}
	if got := s.GapsIn(6, 20); got != 1 {
		t.Fatalf("GapsIn(6, 20) = %d, want 1", got)
	}
}

func TestToWeeklyCarriesDailyGaps(t *testing.T) {
	// 2024-01-01 is a Monday. Bars every day of weeks 1–2 and 4–5; the
	// whole of week 3 (7 days) is missing, so the gap is recorded at the
	// first bar of week 4, and the weekly bar of week 4 carries it.
	cal := calendar(35)
	s := seriesOn(cal, daysExcept(35, 14, 20)...)
	s.MarkGaps(cal, 5)
	w := ToWeekly(s)
	if len(w.Bars) != 4 {
		t.Fatalf("weekly bars = %d, want 4", len(w.Bars))
	}
	for _, c := range []struct{ from, to, want int }{{0, 3, 1}, {1, 3, 1}, {2, 3, 0}, {0, 1, 0}} {
		if got := w.GapsIn(c.from, c.to); got != c.want {
			t.Errorf("weekly GapsIn(%d, %d) = %d, want %d", c.from, c.to, got, c.want)
		}
	}
	if got := ToWeekly(seriesOn(cal, 0, 1)).GapsIn(-1, 0); got != 0 {
		t.Fatalf("unmarked weekly GapsIn = %d", got)
	}
}

func TestGapsInUnmarkedSeriesIsZero(t *testing.T) {
	s := seriesOn(calendar(3), 0, 2)
	if got := s.GapsIn(0, 1); got != 0 {
		t.Fatalf("GapsIn = %d, want 0", got)
	}
}
