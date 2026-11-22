package ingest

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/data"
	"kanso/internal/market"
)

// fakeSource serves fixed bars per Yahoo ticker and fails the tickers in
// fail. It records the tickers and ranges asked for.
type fakeSource struct {
	mu    sync.Mutex
	bars  map[string][]market.Bar
	split map[string][]data.Split
	fail  map[string]bool
	asked []string
}

func (f *fakeSource) Daily(_ context.Context, symbol string, from, to market.Date) ([]market.Bar, []data.Split, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, fmt.Sprintf("%s %s..%s", symbol, from, to))
	if f.fail[symbol] {
		return nil, nil, errors.New("HTTP 503")
	}
	var bars []market.Bar
	for _, b := range f.bars[symbol] {
		if b.Date >= from && b.Date <= to {
			bars = append(bars, b)
		}
	}
	var splits []data.Split
	for _, sp := range f.split[symbol] {
		if sp.Date >= from && sp.Date <= to {
			splits = append(splits, sp)
		}
	}
	return bars, splits, nil
}

func twoBars(c float64) []market.Bar {
	return []market.Bar{
		{Date: "2024-01-01", Open: c, High: c + 1, Low: c - 1, Close: c, Volume: 10},
		{Date: "2024-01-02", Open: c, High: c + 1, Low: c - 1, Close: c + 0.5, Volume: 0},
	}
}

func openStore(t *testing.T) *data.Store {
	t.Helper()
	s, err := data.Open(filepath.Join(t.TempDir(), "kanso.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func opts(symbols ...string) Options {
	return Options{Universe: "NIFTY50", Symbols: symbols, From: "2005-01-01", To: "2024-01-31"}
}

func TestBackfillSeedsUniverseAndBenchmark(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	src := &fakeSource{
		bars:  map[string][]market.Bar{"TCS.NS": twoBars(100), "INFY.NS": twoBars(50), "^NSEI": twoBars(21000)},
		split: map[string][]data.Split{"TCS.NS": {{Date: "2024-01-02", Numerator: 2, Denominator: 1}}},
	}
	sum, err := Backfill(ctx, st, src, opts("TCS", "INFY"))
	if err != nil {
		t.Fatal(err)
	}
	if sum.Instruments != 3 || len(sum.Failed) != 0 || sum.Bars != 6 || sum.TooManyFailures() {
		t.Fatalf("summary = %+v", sum)
	}
	if diff := cmp.Diff([]string{"^NSEI 2005-01-01..2024-01-31", "TCS.NS 2005-01-01..2024-01-31", "INFY.NS 2005-01-01..2024-01-31"}, src.asked); diff != "" {
		t.Fatalf("asked (-want +got):\n%s", diff)
	}

	members, hasHistory, err := st.Universe(ctx, "NIFTY50")
	if err != nil || hasHistory || len(members) != 2 || members[0].Symbol != "INFY" || members[1].Symbol != "TCS" {
		t.Fatalf("universe = %+v %v %v", members, hasHistory, err)
	}
	if members[1].Exchange != "NSE" || members[1].Type != market.Equity {
		t.Fatalf("TCS = %+v", members[1])
	}
	bench, err := st.Instrument(ctx, market.Benchmark)
	if err != nil || bench.Type != market.Index || bench.Exchange != "NSE" {
		t.Fatalf("benchmark = %+v, %v", bench, err)
	}
	got, err := st.Daily(ctx, members[1].ID, "", "9999-12-31")
	if err != nil || !cmp.Equal(twoBars(100), got) {
		t.Fatalf("TCS bars = %+v, %v", got, err)
	}
	if sp, _ := st.Splits(ctx, members[1].ID); len(sp) != 1 {
		t.Fatalf("TCS splits = %+v", sp)
	}
	log, err := st.IngestLog(ctx)
	if err != nil || len(log) != 3 {
		t.Fatalf("ingest log = %+v, %v", log, err)
	}
	for _, e := range log {
		if e.Status != StatusOK || e.LatestBarDate != "2024-01-02" {
			t.Fatalf("log entry = %+v", e)
		}
	}
}

func TestBackfillFailureWritesNothingForSymbol(t *testing.T) {
	ctx := context.Background()
	st := openStore(t)
	src := &fakeSource{
		bars: map[string][]market.Bar{"TCS.NS": twoBars(100), "^NSEI": twoBars(21000)},
		fail: map[string]bool{"INFY.NS": true},
	}
	sum, err := Backfill(ctx, st, src, opts("TCS", "INFY"))
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(sum.Failed, []string{"INFY"}) {
		t.Fatalf("failed = %v", sum.Failed)
	}
	infy, err := st.Instrument(ctx, "INFY")
	if err != nil {
		t.Fatal(err)
	}
	if bars, _ := st.Daily(ctx, infy.ID, "", "9999-12-31"); len(bars) != 0 {
		t.Fatalf("INFY bars written: %+v", bars)
	}
	log, _ := st.IngestLog(ctx)
	var infyLog []data.IngestEntry
	for _, e := range log {
		if e.Symbol == "INFY" {
			infyLog = append(infyLog, e)
		}
	}
	if len(infyLog) != 1 || infyLog[0].Status != StatusError || infyLog[0].Message == "" {
		t.Fatalf("INFY log = %+v", infyLog)
	}
}

func TestBackfillEmptyHistoryIsAFailure(t *testing.T) {
	st := openStore(t)
	src := &fakeSource{bars: map[string][]market.Bar{"^NSEI": twoBars(21000)}}
	sum, err := Backfill(context.Background(), st, src, opts("GONE"))
	if err != nil {
		t.Fatal(err)
	}
	if !cmp.Equal(sum.Failed, []string{"GONE"}) {
		t.Fatalf("failed = %v", sum.Failed)
	}
}

func TestBackfillFailsRunAboveFivePercent(t *testing.T) {
	syms := make([]string, 19) // 19 stocks + ^NSEI = 20 symbols
	src := &fakeSource{bars: map[string][]market.Bar{"^NSEI": twoBars(21000)}, fail: map[string]bool{}}
	for i := range syms {
		syms[i] = fmt.Sprintf("S%02d", i)
		src.bars[syms[i]+".NS"] = twoBars(10)
	}
	src.fail["S00.NS"] = true // 1 of 20 = 5%: not more than 5%
	sum, err := Backfill(context.Background(), openStore(t), src, opts(syms...))
	if err != nil || sum.TooManyFailures() {
		t.Fatalf("1/20 failed: summary %+v, %v; want success", sum, err)
	}
	src.fail["S01.NS"] = true // 2 of 20 = 10%
	sum, err = Backfill(context.Background(), openStore(t), src, opts(syms...))
	if err != nil || !sum.TooManyFailures() {
		t.Fatalf("2/20 failed: summary %+v, %v; want too many failures", sum, err)
	}
}
