package report

import (
	"context"
	"encoding/csv"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"kanso/internal/data"
	"kanso/internal/market"
	"kanso/internal/patterns"
)

// Real-data tests (spec §8) check the engine against NSE breakouts verified
// by eye in V0 (docs/v0/reference-results.md). They need an ingested
// database and skip unless KANSO_DB names one; it is opened read-only. The
// symbols they need are listed in testdata/realdata/symbols.csv.

// realDB opens KANSO_DB read-only, or skips the test.
func realDB(t *testing.T) *data.Store {
	t.Helper()
	path := os.Getenv("KANSO_DB")
	if path == "" {
		t.Skip("KANSO_DB not set")
	}
	st, err := data.Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

// realReport is the default Stock Report (every pattern, default params,
// daily) for symbol on date, which must be a trading day in the data.
func realReport(t *testing.T, st *data.Store, symbol string, date market.Date) *Report {
	t.Helper()
	r, err := Run(context.Background(), st, symbol, nil, date, market.Daily)
	if errors.Is(err, market.ErrNotFound) {
		t.Fatalf("%s is not in KANSO_DB (DIVISLAB, HEROMOTOCO and NAVINFLUOR are not in today's Nifty 50). "+
			"Ingest the real-data symbols: go run ./cmd/ingest -db $KANSO_DB -universe REALDATA "+
			"-constituents testdata/realdata/symbols.csv: %v", symbol, err)
	}
	if err != nil {
		t.Fatalf("%s %s: %v", symbol, date, err)
	}
	if r.AsOf != date {
		t.Fatalf("%s: last bar on or before %s is %s; the data lacks that day", symbol, date, r.AsOf)
	}
	return r
}

// line returns the report line of condition type typ.
func line(t *testing.T, r *Report, typ string) Line {
	t.Helper()
	for _, l := range r.Lines {
		if l.Type == typ {
			return l
		}
	}
	t.Fatalf("%s: no %s line", r.Symbol, typ)
	return Line{}
}

func TestRealDataBreakouts(t *testing.T) {
	st := realDB(t)
	f, err := os.Open(filepath.Join("..", "..", "testdata", "realdata", "breakouts.csv"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		symbol, date := row[0], market.Date(row[1])
		oldHigh, err1 := strconv.ParseFloat(row[2], 64)
		oldHighDate := market.Date(row[3])
		volRatio, err2 := strconv.ParseFloat(row[4], 64)
		volPasses, err3 := strconv.ParseBool(row[5])
		if err1 != nil || err2 != nil || err3 != nil {
			t.Fatalf("bad row %v", row)
		}

		t.Run(symbol+"/"+string(date), func(t *testing.T) {
			r := realReport(t, st, symbol, date)

			nyh := line(t, r, patterns.TypeNYearHigh)
			if !nyh.Result.Evaluable || !nyh.Result.Pass {
				t.Fatalf("n_year_high: %+v", nyh.Result)
			}
			ev := nyh.Result.Evidence.(patterns.NYearHighEvidence)
			if math.Abs(ev.Level/oldHigh-1) > 0.005 {
				t.Errorf("level = %.2f, want %.2f ±0.5%%", ev.Level, oldHigh)
			}
			if ev.LevelDate != oldHighDate {
				t.Errorf("level date = %s, want %s", ev.LevelDate, oldHighDate)
			}

			vol := line(t, r, patterns.TypeVolumeSurge)
			if !vol.Result.Evaluable {
				t.Fatalf("volume_surge not evaluable: %+v", vol.Result)
			}
			ratio := vol.Result.Evidence.(patterns.VolumeEvidence).Ratio
			if math.Abs(ratio/volRatio-1) > 0.10 {
				t.Errorf("volume ratio = %.2f, want %.2f ±10%%", ratio, volRatio)
			}
			if vol.Result.Pass != volPasses {
				t.Errorf("volume_surge pass = %v, want %v (ratio %.2f)", vol.Result.Pass, volPasses, ratio)
			}
		})
	}
}

// NAVINFLUOR 2026-11-18 closed above a 3-year high set on 2026-11-13, only
// 3 bars earlier: a continuation high, not a base breakout. The 1-year
// level_age filter must reject it.
func TestRealDataContinuationHighFailsLevelAge(t *testing.T) {
	r := realReport(t, realDB(t), "NAVINFLUOR", "2026-11-18")

	nyh := line(t, r, patterns.TypeNYearHigh)
	if !nyh.Result.Evaluable || !nyh.Result.Pass {
		t.Fatalf("n_year_high: %+v", nyh.Result)
	}
	age := line(t, r, patterns.TypeLevelAge)
	if got := age.Params.Float("min_age_years"); got != 1 {
		t.Fatalf("level_age min_age_years = %v, want the default 1", got)
	}
	if !age.Result.Evaluable || age.Result.Pass {
		t.Fatalf("level_age: %+v, want evaluable and failing", age.Result)
	}
	ev := age.Result.Evidence.(patterns.LevelAgeEvidence)
	if ev.LevelDate != "2026-11-13" || ev.BarsSince != 3 {
		t.Errorf("level age = %+v, want set 2026-11-13, 3 bars ago", ev)
	}
}

// BAJFINANCE's ex-date 2025-06-16 (a 1:2 split and a 4:1 bonus, 10x the
// shares) must show an ordinary volume ratio. Unadjusted volume (eod2 in
// V0) shows a fake 6x surge here.
func TestRealDataSplitVolumeAdjusted(t *testing.T) {
	r := realReport(t, realDB(t), "BAJFINANCE", "2025-06-16")

	vol := line(t, r, patterns.TypeVolumeSurge)
	if !vol.Result.Evaluable {
		t.Fatalf("volume_surge not evaluable: %+v", vol.Result)
	}
	if ratio := vol.Result.Evidence.(patterns.VolumeEvidence).Ratio; ratio < 0.5 || ratio > 1.5 {
		t.Errorf("volume ratio = %.2f, want 0.5 to 1.5 (split-adjusted volume)", ratio)
	}
}
