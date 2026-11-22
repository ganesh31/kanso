package data

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kanso.db")
	s, err := Open(path, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

func addInstrument(t *testing.T, s *Store, symbol string) int64 {
	t.Helper()
	id, err := s.UpsertInstrument(context.Background(), market.Instrument{Exchange: "NSE", Symbol: symbol, Type: market.Equity, LotSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// bars returns n consecutive daily bars from 2024-01-01 with close c+i.
func bars(n int, c float64) []market.Bar {
	out := make([]market.Bar, n)
	for i := range out {
		d := market.Date(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(market.DateLayout))
		out[i] = market.Bar{Date: d, Open: c + float64(i), High: c + float64(i) + 1, Low: c + float64(i) - 1, Close: c + float64(i), Volume: 1000}
	}
	return out
}

func TestUpsertBarsIdempotent(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	if err := s.UpsertBars(ctx, id, bars(3, 10)); err != nil {
		t.Fatal(err)
	}
	want := bars(3, 20)
	if err := s.UpsertBars(ctx, id, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Daily(ctx, id, "", "9999-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	if again, _ := s.UpsertInstrument(ctx, market.Instrument{Exchange: "NSE", Symbol: "TCS", Type: market.Equity, LotSize: 1}); again != id {
		t.Fatalf("re-upserted instrument id = %d, want %d", again, id)
	}
}

func TestDailyRangeInclusiveSorted(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	all := bars(5, 10)
	// Written out of order; read back sorted.
	if err := s.UpsertBars(ctx, id, []market.Bar{all[4], all[0], all[2], all[1], all[3]}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Daily(ctx, id, all[1].Date, all[3].Date)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(all[1:4], got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	if got, err := s.Daily(ctx, id, "2030-01-01", "2030-12-31"); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty range: got %v, %v; want empty non-nil slice", got, err)
	}
	if _, err := s.Daily(ctx, 999, "", "9999-12-31"); !errors.Is(err, market.ErrNotFound) {
		t.Fatalf("unknown id err = %v, want ErrNotFound", err)
	}
}

func TestInstrumentLookup(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id, err := s.UpsertInstrument(ctx, market.Instrument{Exchange: "NSE", Symbol: "^NSEI", Type: market.Index, LotSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Instrument(ctx, "^NSEI")
	want := market.Instrument{ID: id, Exchange: "NSE", Symbol: "^NSEI", Type: market.Index, LotSize: 1}
	if err != nil || got != want {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
	if _, err := s.Instrument(ctx, "NOPE"); !errors.Is(err, market.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUniverseHasHistoryFalseWhenFromDateNull(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	tcs, infy := addInstrument(t, s, "TCS"), addInstrument(t, s, "INFY")
	if err := s.ReplaceUniverse(ctx, "NIFTY50", []int64{tcs, infy}); err != nil {
		t.Fatal(err)
	}
	members, hasHistory, err := s.Universe(ctx, "NIFTY50")
	if err != nil {
		t.Fatal(err)
	}
	if hasHistory {
		t.Fatal("hasHistory = true, want false for NULL from_date")
	}
	if len(members) != 2 || members[0].Symbol != "INFY" || members[1].Symbol != "TCS" {
		t.Fatalf("members = %+v", members)
	}
	// Replacing drops old members.
	if err := s.ReplaceUniverse(ctx, "NIFTY50", []int64{tcs}); err != nil {
		t.Fatal(err)
	}
	if members, _, _ := s.Universe(ctx, "NIFTY50"); len(members) != 1 || members[0].ID != tcs {
		t.Fatalf("after replace: %+v", members)
	}
	names, err := s.Universes(ctx)
	if err != nil || !cmp.Equal(names, []string{"NIFTY50"}) {
		t.Fatalf("Universes = %v, %v", names, err)
	}
	if _, _, err := s.Universe(ctx, "NOPE"); !errors.Is(err, market.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestReadOnlyRejectsWrites(t *testing.T) {
	ctx := context.Background()
	rw, path := openTemp(t)
	id := addInstrument(t, rw, "TCS")
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if err := ro.UpsertBars(ctx, id, bars(1, 10)); err == nil {
		t.Fatal("UpsertBars on a read-only store: want error")
	}
	if _, err := ro.Instrument(ctx, "TCS"); err != nil {
		t.Fatalf("read on read-only store: %v", err)
	}
}

func TestExportParquetRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	tcs, infy := addInstrument(t, s, "TCS"), addInstrument(t, s, "INFY")
	tcsBars := bars(4, 0.1) // 0.1 + i are not exact in binary: exercises float64 fidelity
	tcsBars[2].Close = 1234.5678901234567
	if err := s.UpsertBars(ctx, tcs, tcsBars); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertBars(ctx, infy, bars(2, 1.0/3)); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := s.ExportParquet(ctx, []string{"TCS", "INFY"}, tcsBars[1].Date, "9999-12-31", &buf); err != nil {
		t.Fatal(err)
	}
	got, err := market.ReadParquet(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]market.Bar{"TCS": tcsBars[1:], "INFY": bars(2, 1.0/3)[1:]}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestImportParquetBuildsStoreAndUniverse(t *testing.T) {
	ctx := context.Background()
	src, _ := openTemp(t)
	tcs, nsei := addInstrument(t, src, "TCS"), addInstrument(t, src, market.Benchmark)
	if err := src.UpsertBars(ctx, tcs, bars(3, 10)); err != nil {
		t.Fatal(err)
	}
	if err := src.UpsertBars(ctx, nsei, bars(3, 100)); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := src.ExportParquet(ctx, []string{"TCS", market.Benchmark}, "", "9999-12-31", &buf); err != nil {
		t.Fatal(err)
	}

	dst, _ := openTemp(t)
	if err := dst.ImportParquet(ctx, bytes.NewReader(buf.Bytes()), "NIFTY50"); err != nil {
		t.Fatal(err)
	}
	members, _, err := dst.Universe(ctx, "NIFTY50")
	if err != nil {
		t.Fatal(err)
	}
	if len(members) != 1 || members[0].Symbol != "TCS" || members[0].Type != market.Equity {
		t.Fatalf("universe members = %+v, want only TCS as an equity", members)
	}
	bench, err := dst.Instrument(ctx, market.Benchmark)
	if err != nil {
		t.Fatal(err)
	}
	if bench.Type != market.Index {
		t.Fatalf("benchmark type = %q, want index", bench.Type)
	}
	got, err := dst.Daily(ctx, members[0].ID, "", "9999-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(bars(3, 10), got); diff != "" {
		t.Fatalf("TCS bars (-want +got):\n%s", diff)
	}
}

func TestWALReaderDuringWrite(t *testing.T) {
	ctx := context.Background()
	rw, path := openTemp(t)
	id := addInstrument(t, rw, "TCS")
	before := bars(3, 10)
	if err := rw.UpsertBars(ctx, id, before); err != nil {
		t.Fatal(err)
	}
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()

	// A second process holds a write transaction open: the read-only store
	// still reads the last committed bars.
	other, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(100)")
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	tx, err := other.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM bars_daily"); err != nil {
		t.Fatal(err)
	}
	got, err := ro.Daily(ctx, id, "", "9999-12-31")
	if err != nil {
		t.Fatalf("read during write: %v", err)
	}
	if diff := cmp.Diff(before, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// A reader holds a read transaction open: the writer still commits.
	rtx, err := other.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer rtx.Rollback()
	var n int
	if err := rtx.QueryRowContext(ctx, "SELECT count(*) FROM bars_daily").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if err := rw.UpsertBars(ctx, id, bars(5, 10)); err != nil {
		t.Fatalf("write during read: %v", err)
	}
}

func TestReplaceBarsIsAtomic(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	old := bars(3, 10)
	if err := s.UpsertBars(ctx, id, old); err != nil {
		t.Fatal(err)
	}
	// A duplicate date makes the second insert fail mid-transaction.
	bad := bars(2, 50)
	bad = append(bad, bad[1])
	if err := s.ReplaceBars(ctx, id, bad); err == nil {
		t.Fatal("ReplaceBars with a duplicate date: want error")
	}
	got, err := s.Daily(ctx, id, "", "9999-12-31")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(old, got); diff != "" {
		t.Fatalf("old bars changed (-want +got):\n%s", diff)
	}
	// A good replace drops bars it doesn't contain.
	if err := s.ReplaceBars(ctx, id, bars(2, 50)); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Daily(ctx, id, "", "9999-12-31"); !cmp.Equal(bars(2, 50), got) {
		t.Fatalf("after replace: %+v", got)
	}
}

func TestLatestBarDate(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	if d, err := s.LatestBarDate(ctx, id); err != nil || d != "" {
		t.Fatalf("no bars: %q, %v", d, err)
	}
	b := bars(4, 10)
	if err := s.UpsertBars(ctx, id, b); err != nil {
		t.Fatal(err)
	}
	if d, err := s.LatestBarDate(ctx, id); err != nil || d != b[3].Date {
		t.Fatalf("got %q, %v; want %s", d, err, b[3].Date)
	}
}

func TestEarliestBarDate(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	if d, err := s.EarliestBarDate(ctx, id); err != nil || d != "" {
		t.Fatalf("no bars: %q, %v", d, err)
	}
	if err := s.UpsertBars(ctx, id, bars(4, 10)); err != nil {
		t.Fatal(err)
	}
	if d, err := s.EarliestBarDate(ctx, id); err != nil || d != "2024-01-01" {
		t.Fatalf("got %q, %v; want 2024-01-01", d, err)
	}
}

func TestSaveSplitsAndLogIngest(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	sp := []Split{{Date: "2024-01-05", Numerator: 2, Denominator: 1}}
	if err := s.SaveSplits(ctx, id, sp); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveSplits(ctx, id, sp); err != nil {
		t.Fatalf("idempotent: %v", err)
	}
	got, err := s.Splits(ctx, id)
	if err != nil || !cmp.Equal(sp, got) {
		t.Fatalf("Splits = %+v, %v", got, err)
	}
	if err := s.LogIngest(ctx, id, "ok", "3 bars"); err != nil {
		t.Fatal(err)
	}
	log, err := s.IngestLog(ctx)
	if err != nil || len(log) != 1 || log[0].Symbol != "TCS" || log[0].Status != "ok" || log[0].Message != "3 bars" {
		t.Fatalf("IngestLog = %+v, %v", log, err)
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kanso.db")
	for range 2 {
		s, err := Open(path, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOpenReadOnlyMissingFileFails(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing.db"), true); err == nil {
		t.Fatal("want error opening a missing file read-only")
	}
}

func TestUpsertDailyIsAtomic(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	// SQLite stores NaN as NULL, which the NOT NULL numerator rejects: the
	// split write fails after the bars were written in the same transaction.
	bad := []Split{{Date: "2024-01-02", Numerator: math.NaN(), Denominator: 1}}
	if err := s.UpsertDaily(ctx, id, bars(3, 10), bad); err == nil {
		t.Fatal("UpsertDaily with an invalid split: want error")
	}
	got, err := s.Daily(ctx, id, "", "9999-12-31")
	if err != nil || len(got) != 0 {
		t.Fatalf("bars after a failed UpsertDaily = %d, %v; want none", len(got), err)
	}
	good := []Split{{Date: "2024-01-02", Numerator: 2, Denominator: 1}}
	if err := s.UpsertDaily(ctx, id, bars(3, 10), good); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Daily(ctx, id, "", "9999-12-31"); len(got) != 3 {
		t.Fatalf("bars = %d, want 3", len(got))
	}
	if sp, _ := s.Splits(ctx, id); !cmp.Equal(good, sp) {
		t.Fatalf("Splits = %+v", sp)
	}
}

func TestReplaceDailyIsAtomic(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	id := addInstrument(t, s, "TCS")
	old := bars(5, 200)
	if err := s.UpsertBars(ctx, id, old); err != nil {
		t.Fatal(err)
	}
	bad := []Split{{Date: "2024-01-02", Numerator: math.NaN(), Denominator: 1}}
	if err := s.ReplaceDaily(ctx, id, bars(3, 100), bad); err == nil {
		t.Fatal("ReplaceDaily with an invalid split: want error")
	}
	if got, _ := s.Daily(ctx, id, "", "9999-12-31"); !cmp.Equal(old, got) {
		t.Fatalf("old bars changed after a failed ReplaceDaily: %+v", got)
	}
	good := []Split{{Date: "2024-01-02", Numerator: 2, Denominator: 1}}
	if err := s.ReplaceDaily(ctx, id, bars(3, 100), good); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Daily(ctx, id, "", "9999-12-31"); !cmp.Equal(bars(3, 100), got) {
		t.Fatalf("after replace: %+v", got)
	}
	if sp, _ := s.Splits(ctx, id); !cmp.Equal(good, sp) {
		t.Fatalf("Splits = %+v", sp)
	}
}
