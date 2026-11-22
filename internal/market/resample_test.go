package market

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// day is a daily bar with distinct OHLCV values.
func day(d Date, o, h, l, c, v float64) Bar {
	return Bar{Date: d, Open: o, High: h, Low: l, Close: c, Volume: v}
}

// closes returns daily bars on the given dates, each with close = 10×(i+1).
func closes(dates ...Date) []Bar {
	bars := make([]Bar, len(dates))
	for i, d := range dates {
		c := float64(10 * (i + 1))
		bars[i] = day(d, c, c, c, c, 1)
	}
	return bars
}

func weekly(bars []Bar) *Series {
	return ToWeekly(NewDaily(Instrument{ID: 1, Symbol: "TCS"}, bars))
}

func TestWeeklyResampleOHLCV(t *testing.T) {
	w := weekly([]Bar{
		day("2024-01-01", 100, 105, 98, 104, 10),
		day("2024-01-02", 104, 112, 101, 110, 20),
		day("2024-01-03", 110, 111, 95, 96, 30),
		day("2024-01-04", 96, 100, 94, 99, 40),
		day("2024-01-05", 99, 103, 97, 102, 50),
		day("2024-01-08", 102, 104, 101, 103, 60),
	})
	if w.Timeframe != Weekly {
		t.Errorf("Timeframe = %q, want weekly", w.Timeframe)
	}
	want := []Bar{
		{Date: "2024-01-05", Open: 100, High: 112, Low: 94, Close: 102, Volume: 150},
		{Date: "2024-01-08", Open: 102, High: 104, Low: 101, Close: 103, Volume: 60},
	}
	if diff := cmp.Diff(want, w.Bars); diff != "" {
		t.Errorf("Bars (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]int{4, 5}, w.LastDailyIdx); diff != "" {
		t.Errorf("LastDailyIdx (-want +got):\n%s", diff)
	}
}

func TestWeeklyLastWeekIncompleteUntilNextWeekBar(t *testing.T) {
	week1 := closes("2024-01-01", "2024-01-02", "2024-01-03", "2024-01-04", "2024-01-05")
	w := weekly(week1)
	if len(w.Bars) != 1 || w.Complete != 0 {
		t.Fatalf("through Fri: len(Bars)=%d Complete=%d, want 1, 0", len(w.Bars), w.Complete)
	}
	w = weekly(append(week1, closes("2024-01-08")...))
	if len(w.Bars) != 2 || w.Complete != 1 {
		t.Fatalf("with next Mon: len(Bars)=%d Complete=%d, want 2, 1", len(w.Bars), w.Complete)
	}
}

func TestWeeklyHolidayFridayAndShortWeek(t *testing.T) {
	w := weekly(closes("2024-03-25", "2024-03-26", "2024-03-27", "2024-03-28", "2024-04-01"))
	if got := w.Bars[0].Close; got != 40 {
		t.Errorf("week 1 Close = %v, want Thu close 40", got)
	}
	if got := w.Bars[0].Date; got != "2024-03-28" {
		t.Errorf("week 1 Date = %q, want 2024-03-28", got)
	}
	if w.LastDailyIdx[0] != 3 || w.Complete != 1 {
		t.Errorf("LastDailyIdx[0]=%d Complete=%d, want 3, 1", w.LastDailyIdx[0], w.Complete)
	}
}

func TestWeeklyYearBoundaryISOWeek(t *testing.T) {
	w := weekly(closes("2024-12-27", "2024-12-30", "2024-12-31", "2025-01-01", "2025-01-02", "2025-01-06"))
	if len(w.Bars) != 3 {
		t.Fatalf("len(Bars) = %d, want 3 (2024-W52, 2025-W01, 2025-W02)", len(w.Bars))
	}
	if diff := cmp.Diff([]int{0, 4, 5}, w.LastDailyIdx); diff != "" {
		t.Errorf("LastDailyIdx (-want +got):\n%s", diff)
	}
	if b := w.Bars[1]; b.Open != 20 || b.Close != 50 || b.Volume != 4 {
		t.Errorf("2025-W01 = %+v, want open 20 (Mon 30 Dec), close 50 (Thu 2 Jan), volume 4", b)
	}
}

func TestWeeklyEmpty(t *testing.T) {
	w := weekly(nil)
	if len(w.Bars) != 0 || w.Complete != 0 {
		t.Errorf("empty: len(Bars)=%d Complete=%d", len(w.Bars), w.Complete)
	}
}
