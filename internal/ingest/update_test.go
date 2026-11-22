package ingest

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/data"
	"kanso/internal/market"
)

// weekdays returns n weekday bars from start (a weekday) at close c.
func weekdays(start market.Date, n int, c float64) []market.Bar {
	out := make([]market.Bar, 0, n)
	for d := start.Time(); len(out) < n; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd == time.Saturday || wd == time.Sunday {
			continue
		}
		out = append(out, market.Bar{Date: market.Date(d.Format(market.DateLayout)), Open: c, High: c, Low: c, Close: c, Volume: 1000})
	}
	return out
}

func storedBars(t *testing.T, st *data.Store, symbol string) []market.Bar {
	t.Helper()
	in, err := st.Instrument(context.Background(), symbol)
	if err != nil {
		t.Fatal(err)
	}
	bars, err := st.Daily(context.Background(), in.ID, "", "9999-12-31")
	if err != nil {
		t.Fatal(err)
	}
	return bars
}

func lastLog(t *testing.T, st *data.Store, symbol string) data.IngestEntry {
	t.Helper()
	log, err := st.IngestLog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := len(log) - 1; i >= 0; i-- {
		if log[i].Symbol == symbol {
			return log[i]
		}
	}
	t.Fatalf("no ingest log for %s", symbol)
	return data.IngestEntry{}
}

func optsTo(to market.Date, symbols ...string) Options {
	o := opts(symbols...)
	o.To = to
	return o
}

// backfilled stores TCS and ^NSEI from src up to to and returns the store.
func backfilled(t *testing.T, src *fakeSource, to market.Date) *data.Store {
	t.Helper()
	st := openStore(t)
	if _, err := Backfill(context.Background(), st, src, optsTo(to, "TCS")); err != nil {
		t.Fatal(err)
	}
	src.asked = nil
	return st
}

func TestIngestSummaryReportsBehindInstruments(t *testing.T) {
	src := &fakeSource{bars: map[string][]market.Bar{
		"^NSEI":   weekdays("2026-11-02", 13, 24000), // through 2026-11-18
		"INFY.NS": weekdays("2026-11-02", 13, 1500),
		"TCS.NS":  weekdays("2026-11-02", 12, 4000), // Yahoo lags: through 2026-11-17
	}}
	sum, err := Update(context.Background(), openStore(t), src, optsTo("2026-11-18", "TCS", "INFY"))
	if err != nil {
		t.Fatal(err)
	}
	if sum.AsOf != "2026-11-18" || !cmp.Equal([]string{"TCS"}, sum.Behind) {
		t.Fatalf("as of %q, behind %v", sum.AsOf, sum.Behind)
	}
	if got, want := sum.Freshness(), "data as of 2026-11-18; 1 instrument behind: TCS"; got != want {
		t.Fatalf("Freshness() = %q, want %q", got, want)
	}
}

func TestBackfillRerunWithLaterFromKeepsOneBasis(t *testing.T) {
	src := &fakeSource{bars: map[string][]market.Bar{"TCS.NS": weekdays("2016-01-01", 2000, 200), "^NSEI": weekdays("2016-01-01", 2000, 21000)}}
	st := backfilled(t, src, "2023-08-31")

	// A split since: Yahoo now serves all history at half. Rerunning the
	// backfill from 2020 must not keep the pre-2020 bars at the old scale.
	src.bars["TCS.NS"] = weekdays("2016-01-01", 2000, 100)
	o := optsTo("2023-08-31", "TCS")
	o.From = "2020-01-01"
	if _, err := Backfill(context.Background(), st, src, o); err != nil {
		t.Fatal(err)
	}
	for _, b := range storedBars(t, st, "TCS") {
		if b.Close != 100 {
			t.Fatalf("bar %s close %v: history mixes adjustment bases", b.Date, b.Close)
		}
	}
	if got := storedBars(t, st, "TCS"); got[0].Date != "2020-01-01" {
		t.Fatalf("first bar %s, want 2020-01-01 (history is what the backfill fetched)", got[0].Date)
	}
}

func TestUpdateUpsertsWhenOverlapMatches(t *testing.T) {
	old := weekdays("2024-01-01", 65, 100) // through 2024-03-29
	src := &fakeSource{bars: map[string][]market.Bar{"TCS.NS": old, "^NSEI": weekdays("2024-01-01", 65, 21000)}}
	st := backfilled(t, src, "2024-03-29")

	// Yahoo now has a week more, and the overlap differs by exactly 0.5%.
	fresh := weekdays("2024-01-01", 70, 100.5) // through 2024-04-05
	src.bars["TCS.NS"] = fresh
	src.bars["^NSEI"] = weekdays("2024-01-01", 70, 21000)
	if _, err := Update(context.Background(), st, src, optsTo("2024-04-05", "TCS")); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"^NSEI 2024-03-19..2024-04-05", "TCS.NS 2024-03-19..2024-04-05"}, src.asked); diff != "" {
		t.Fatalf("asked (-want +got):\n%s", diff)
	}
	want := append(append([]market.Bar{}, old[:56]...), fresh[56:]...) // fresh[56] is 2024-03-19
	if diff := cmp.Diff(want, storedBars(t, st, "TCS")); diff != "" {
		t.Fatalf("TCS bars (-want +got):\n%s", diff)
	}
	if e := lastLog(t, st, "TCS"); e.Status != StatusOK || e.LatestBarDate != "2024-04-05" {
		t.Fatalf("TCS log = %+v", e)
	}
}

func TestUpdateRefetchesWhenOverlapRestated(t *testing.T) {
	old := weekdays("2024-01-01", 65, 100) // through 2024-03-29
	src := &fakeSource{bars: map[string][]market.Bar{"TCS.NS": old, "^NSEI": weekdays("2024-01-01", 65, 21000)}}
	st := backfilled(t, src, "2024-03-29")

	// No split event, but Yahoo has restated history: one overlapping
	// close is 0.6% off, and older bars outside the window changed too.
	fresh := weekdays("2024-01-01", 70, 100)
	fresh[60].Close = 100.6 // 2024-03-25, inside the overlap
	for i := range 40 {
		fresh[i].Close = 50
	}
	src.bars["TCS.NS"] = fresh
	src.bars["^NSEI"] = weekdays("2024-01-01", 70, 21000)
	if _, err := Update(context.Background(), st, src, optsTo("2024-04-05", "TCS")); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"^NSEI 2024-03-19..2024-04-05", "TCS.NS 2024-03-19..2024-04-05", "TCS.NS 2005-01-01..2024-04-05"}, src.asked); diff != "" {
		t.Fatalf("asked (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(fresh, storedBars(t, st, "TCS")); diff != "" {
		t.Fatalf("TCS bars not replaced (-want +got):\n%s", diff)
	}
	if e := lastLog(t, st, "TCS"); e.Status != StatusOK || !strings.Contains(e.Message, "refetched: restated") {
		t.Fatalf("TCS log = %+v", e)
	}
}

func TestUpdateRefetchesFullHistoryOnSplit(t *testing.T) {
	old := weekdays("2016-01-01", 2000, 200) // through 2023-08-31
	src := &fakeSource{
		bars:  map[string][]market.Bar{"TCS.NS": old, "^NSEI": weekdays("2016-01-01", 2005, 21000)},
		split: map[string][]data.Split{},
	}
	st := backfilled(t, src, "2023-08-31")

	// A 2-for-1 split on 2023-09-05: Yahoo rescales all history to half.
	src.bars["TCS.NS"] = weekdays("2016-01-01", 2005, 100)
	src.split["TCS.NS"] = []data.Split{{Date: "2023-09-05", Numerator: 2, Denominator: 1}}
	if _, err := Update(context.Background(), st, src, optsTo("2023-09-07", "TCS")); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"^NSEI 2023-08-21..2023-09-07", "TCS.NS 2023-08-21..2023-09-07", "TCS.NS 2005-01-01..2023-09-07"}, src.asked); diff != "" {
		t.Fatalf("asked (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(weekdays("2016-01-01", 2005, 100), storedBars(t, st, "TCS")); diff != "" {
		t.Fatalf("TCS bars not halved (-want +got):\n%s", diff)
	}
	tcs, _ := st.Instrument(context.Background(), "TCS")
	if sp, _ := st.Splits(context.Background(), tcs.ID); !cmp.Equal(src.split["TCS.NS"], sp) {
		t.Fatalf("TCS splits = %+v", sp)
	}
	if e := lastLog(t, st, "TCS"); e.Status != StatusOK || !strings.Contains(e.Message, "refetched: split") {
		t.Fatalf("TCS log = %+v", e)
	}
}
