package backtest_test

// The vectorbt oracle benchmark (spec §8). tools/vbt-benchmark re-implements
// the §5 rules in pandas + vectorbt and writes golden signals and trades for
// the systems in testdata/benchmark/systems from the frozen bars fixture.
// These tests run Kanso on the same fixture in single-stock mode and compare.
// They skip when the fixture is absent, so the suite never needs Python.
// Regenerate everything with `make benchmark KANSO_DB=…`.

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"kanso/internal/backtest"
	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

const benchDir = "../../testdata/benchmark"

// allData is the last date of a load that wants every bar.
const allData market.Date = "9999-12-31"

// benchSystems are the golden systems, by file name without ".json".
var benchSystems = []string{"daily_defaults", "weekly_defaults", "no_filters", "stop_only"}

// allowedDiffs are differences between Kanso and the oracle that the spec
// explains, keyed "system/symbol/signal_date". Each entry's comment cites
// the § that makes Kanso's behaviour the intended one. Every other
// difference fails the test.
//
// vectorbt's from_signals places one order per bar and checks a stop before
// any signal, and only from the bar after the entry. §5.4 differs in two
// ways, and Kanso follows §5.4:
var allowedDiffs = map[string]string{
	// Trend exit and stop on the same day. §5.4: "whichever triggers first
	// wins". The trend exit fills at the open, which comes before any
	// intraday low, so it wins; vectorbt takes the stop.
	//   HINDUNILVR 2021-10-25: open 2454.10, stop 2417.42, low 2410.00.
	//   BHARTIARTL 2018-01-29: open 400.16, stop 394.24, low 392.09.
	"daily_defaults/HINDUNILVR/2021-08-20":  "§5.4 trend exit at the open comes before the intraday stop",
	"no_filters/HINDUNILVR/2021-08-20":      "§5.4 trend exit at the open comes before the intraday stop",
	"weekly_defaults/BHARTIARTL/2017-10-19": "§5.4 trend exit at the open comes before the intraday stop",
	// ICICIBANK 2023-01-30 opened at 800.00, below the 811.74 stop, on a
	// trend-exit day. Both fill at the same open (799.20 after slippage);
	// only the reason differs. Kanso checks the trend exit first (§5.4,
	// TradePath), so the reason is "trend".
	"no_filters/ICICIBANK/2022-08-12": "§5.4 trend exit and gapped stop at the same open; Kanso reports trend",
	// Stop hit on the entry day. §5.4 checks the stop "on every daily bar
	// after entry", and the entry fills at the open, before the day's low.
	// LT entered 2024-06-04 (election results day) at 3855.85, stop
	// 3547.38, and fell to a 3320.00 low that day; vectorbt only checks
	// from the next bar, so it stops out at 2024-06-05's open instead.
	"daily_defaults/LT/2024-06-03": "§5.4 the stop is checked on the entry day too",
	"no_filters/LT/2024-06-03":     "§5.4 the stop is checked on the entry day too",
}

// benchTrade is one trade, as the golden CSV lists it.
type benchTrade struct {
	SignalDate, EntryDate, ExitDate market.Date
	ExitReason                      string
	EntryPrice, ExitPrice           float64
	// Gapped is Kanso's GappedStop (always false for the oracle's rows).
	Gapped bool
}

// golden is the oracle's output: signal dates and trades keyed
// "system/symbol".
type golden struct {
	signals map[string][]market.Date
	trades  map[string][]benchTrade
}

// fixture loads the bars fixture into a provider, or skips.
func fixture(t *testing.T) *market.MemProvider {
	t.Helper()
	f, err := os.Open(filepath.Join(benchDir, "bars.parquet"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip("no benchmark fixture; run make benchmark")
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bars, err := market.ReadParquet(f)
	if err != nil {
		t.Fatal(err)
	}
	return market.MemProviderFromBars(bars)
}

// benchSystem reads a golden system, run over every equity in the fixture.
func benchSystem(t *testing.T, name string) rules.System {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(benchDir, "systems", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var sys rules.System
	if err := json.Unmarshal(b, &sys); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	sys.Universe = "ALL" // MemProviderFromBars lists every equity under ALL
	return sys
}

// kansoRun is Kanso's single-stock output for a system: signal dates (from
// the run's data start, as Run filters them) and AllTrades, keyed by
// symbol.
func kansoRun(t *testing.T, prov *market.MemProvider, name string) (map[string][]market.Date, map[string][]benchTrade) {
	t.Helper()
	ctx := context.Background()
	sys := benchSystem(t, name)
	res, err := backtest.Run(ctx, prov, sys, "", "")
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	trades := map[string][]benchTrade{}
	for _, tr := range res.AllTrades {
		trades[tr.Symbol] = append(trades[tr.Symbol], benchTrade{
			SignalDate: tr.SignalDate, EntryDate: tr.EntryDate, ExitDate: tr.ExitDate,
			ExitReason: string(tr.ExitReason), EntryPrice: tr.EntryPrice, ExitPrice: tr.ExitPrice, Gapped: tr.GappedStop,
		})
	}

	sys = res.System
	compiled, err := rules.Compile(sys.Entry, patterns.Default())
	if err != nil {
		t.Fatal(err)
	}
	benchInst, err := prov.Instrument(ctx, sys.Entry.Benchmark())
	if err != nil {
		t.Fatal(err)
	}
	benchBars, err := prov.Daily(ctx, benchInst.ID, "", allData)
	if err != nil {
		t.Fatal(err)
	}
	bench := market.NewDaily(benchInst, benchBars)
	members, _, err := prov.Universe(ctx, sys.Universe)
	if err != nil {
		t.Fatal(err)
	}
	signals := map[string][]market.Date{}
	for _, inst := range members {
		pctx, _, err := patterns.Load(ctx, prov, inst, bench, sys.Timeframe, "", allData)
		if err != nil {
			t.Fatal(err)
		}
		if err := compiled.Prepare(pctx); err != nil {
			t.Fatal(err)
		}
		for _, s := range backtest.Signals(compiled, pctx) {
			if d := pctx.Series.Bars[s].Date; d >= res.Honesty.DataStart {
				signals[inst.Symbol] = append(signals[inst.Symbol], d)
			}
		}
	}
	return signals, trades
}

// readGolden reads the oracle's two CSVs.
func readGolden(t *testing.T) golden {
	t.Helper()
	g := golden{signals: map[string][]market.Date{}, trades: map[string][]benchTrade{}}
	for _, row := range readCSV(t, "expected_signals.csv", []string{"system", "symbol", "signal_date"}) {
		k := row[0] + "/" + row[1]
		g.signals[k] = append(g.signals[k], market.Date(row[2]))
	}
	header := []string{"system", "symbol", "signal_date", "entry_date", "exit_date", "exit_reason", "entry_price", "exit_price"}
	for _, row := range readCSV(t, "expected_trades.csv", header) {
		entry, err1 := strconv.ParseFloat(row[6], 64)
		exit, err2 := strconv.ParseFloat(row[7], 64)
		if err := errors.Join(err1, err2); err != nil {
			t.Fatalf("expected_trades.csv: %v", err)
		}
		k := row[0] + "/" + row[1]
		g.trades[k] = append(g.trades[k], benchTrade{
			SignalDate: market.Date(row[2]), EntryDate: market.Date(row[3]), ExitDate: market.Date(row[4]),
			ExitReason: row[5], EntryPrice: entry, ExitPrice: exit,
		})
	}
	return g
}

// readCSV reads a golden CSV and checks its header.
func readCSV(t *testing.T, name string, header []string) [][]string {
	t.Helper()
	f, err := os.Open(filepath.Join(benchDir, name))
	if err != nil {
		t.Fatalf("fixture present but %s missing (run make benchmark): %v", name, err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if len(rows) == 0 || !slices.Equal(rows[0], header) {
		t.Fatalf("%s: header %v, want %v", name, rows[0], header)
	}
	return rows[1:]
}

// relEq is the oracle tolerance: 1e-9 relative (ADR 0004).
func relEq(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*max(math.Abs(a), math.Abs(b))
}

// diffTrades lists the differences between Kanso's and the oracle's trades
// for one system and symbol, matched by signal date.
func diffTrades(key string, kanso, oracle []benchTrade) []string {
	var out []string
	byDate := map[market.Date]benchTrade{}
	for _, o := range oracle {
		byDate[o.SignalDate] = o
	}
	seen := map[market.Date]bool{}
	for _, k := range kanso {
		seen[k.SignalDate] = true
		o, ok := byDate[k.SignalDate]
		if !ok {
			out = append(out, fmt.Sprintf("%s/%s\tonly in Kanso: entry %s exit %s %s", key, k.SignalDate, k.EntryDate, k.ExitDate, k.ExitReason))
			continue
		}
		var d []string
		if k.EntryDate != o.EntryDate {
			d = append(d, fmt.Sprintf("entry date %s vs %s", k.EntryDate, o.EntryDate))
		}
		if k.ExitDate != o.ExitDate || k.ExitReason != o.ExitReason {
			d = append(d, fmt.Sprintf("exit %s %s vs %s %s", k.ExitDate, k.ExitReason, o.ExitDate, o.ExitReason))
		}
		if !relEq(k.EntryPrice, o.EntryPrice) {
			d = append(d, fmt.Sprintf("entry price %.10g vs %.10g", k.EntryPrice, o.EntryPrice))
		}
		// A gapped stop fills at the open in Kanso (§5.4); the oracle's
		// price is not held to it.
		if !k.Gapped && !relEq(k.ExitPrice, o.ExitPrice) {
			d = append(d, fmt.Sprintf("exit price %.10g vs %.10g", k.ExitPrice, o.ExitPrice))
		}
		if len(d) > 0 {
			out = append(out, fmt.Sprintf("%s/%s\t%s", key, k.SignalDate, strings.Join(d, "; ")))
		}
	}
	for _, o := range oracle {
		if !seen[o.SignalDate] {
			out = append(out, fmt.Sprintf("%s/%s\tonly in oracle: entry %s exit %s %s", key, o.SignalDate, o.EntryDate, o.ExitDate, o.ExitReason))
		}
	}
	return out
}

// symbolsOf is the sorted union of the symbols in two maps.
func symbolsOf[V any](a, b map[string]V) []string {
	var out []string
	for k := range a {
		out = append(out, k)
	}
	for k := range b {
		out = append(out, k)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestBenchmarkAgainstVectorbt(t *testing.T) {
	prov := fixture(t)
	g := readGolden(t)
	var diffs, allowed []string
	used := map[string]bool{}
	for _, name := range benchSystems {
		signals, trades := kansoRun(t, prov, name)
		oracleSignals := map[string][]market.Date{}
		oracleTrades := map[string][]benchTrade{}
		for k, v := range g.signals {
			if sym, ok := strings.CutPrefix(k, name+"/"); ok {
				oracleSignals[sym] = v
			}
		}
		for k, v := range g.trades {
			if sym, ok := strings.CutPrefix(k, name+"/"); ok {
				oracleTrades[sym] = v
			}
		}
		if len(oracleSignals) == 0 {
			t.Errorf("%s: no golden signals", name)
		}
		for _, sym := range symbolsOf(signals, oracleSignals) {
			key := name + "/" + sym
			if k, o := signals[sym], oracleSignals[sym]; !slices.Equal(k, o) {
				diffs = append(diffs, fmt.Sprintf("%s\tsignals differ:\n\t\tonly Kanso  %v\n\t\tonly oracle %v", key, minus(k, o), minus(o, k)))
			}
		}
		for _, sym := range symbolsOf(trades, oracleTrades) {
			for _, d := range diffTrades(name+"/"+sym, trades[sym], oracleTrades[sym]) {
				id, _, _ := strings.Cut(d, "\t")
				if reason, ok := allowedDiffs[id]; ok {
					allowed = append(allowed, d+"\t(allowed: "+reason+")")
					used[id] = true
					continue
				}
				diffs = append(diffs, d)
			}
		}
	}
	for _, a := range allowed {
		t.Log(a)
	}
	for id := range allowedDiffs {
		if !used[id] {
			t.Errorf("allowedDiffs %s no longer differs; remove it", id)
		}
	}
	if len(diffs) > 0 {
		t.Errorf("%d differences from the vectorbt oracle:\n%s", len(diffs), strings.Join(diffs, "\n"))
	}
}

// minus is the dates in a that are not in b.
func minus(a, b []market.Date) []market.Date {
	out := []market.Date{}
	for _, d := range a {
		if !slices.Contains(b, d) {
			out = append(out, d)
		}
	}
	return out
}

// kansoTrade is Kanso's trade in sym entered on entry, under a system.
func kansoTrade(t *testing.T, prov *market.MemProvider, name, sym string, entry market.Date) benchTrade {
	t.Helper()
	_, trades := kansoRun(t, prov, name)
	for _, tr := range trades[sym] {
		if tr.EntryDate == entry {
			return tr
		}
	}
	t.Fatalf("%s: no Kanso %s trade entered %s; trades %+v", name, sym, entry, trades[sym])
	return benchTrade{}
}

func TestBenchmarkDIVISLABStopAug2025(t *testing.T) {
	prov := fixture(t)
	// V0's hand calculation said 2025-08-06, TradingView 08-07 and vectorbt
	// 08-08. §5.4 settles it from the fixture bars:
	//   - Signal 2025-05-19, entry at the next open, 2025-05-20: 6580.00.
	//   - Fill incl. 0.10% slippage = 6580 × 1.001 = 6586.58.
	//   - Stop = fill × 0.92 = 6059.6536.
	//   - The stop is checked on every daily bar from the entry on: the
	//     first low at or below it is 2025-08-06 (low 6054.50; 08-05's low
	//     was 6363.50). That day opened at 6410.50, above the stop, so it
	//     fills at the stop, 6059.6536 less slippage, not at the open.
	// V0's 08-07 and 08-08 came from stops placed off the signal close or
	// checked against the close only (§8 setup traps).
	tr := kansoTrade(t, prov, "daily_defaults", "DIVISLAB", "2025-05-20")
	if tr.ExitDate != "2025-08-06" || tr.ExitReason != string(backtest.ExitStop) || tr.Gapped {
		t.Fatalf("exit %s %s gapped=%v, want 2025-08-06 stop, not gapped", tr.ExitDate, tr.ExitReason, tr.Gapped)
	}
	if want := 6580 * 1.001 * 0.92 * 0.999; !relEq(tr.ExitPrice, want) {
		t.Fatalf("exit price %v, want %v", tr.ExitPrice, want)
	}
}

func TestBenchmarkOracleUsesIntradayStops(t *testing.T) {
	prov := fixture(t)
	// DIVISLAB entered 2020-04-13 at 2385 × 1.001 = 2387.385, so the stop
	// is 2196.39. 2020-07-02 opened at 2100, below it, and closed at
	// 2196.65, just above it. An oracle that checks stops against the
	// close only would hold on; one given open/high/low exits at the open.
	want := 2100 * 0.999
	k := kansoTrade(t, prov, "daily_defaults", "DIVISLAB", "2020-04-13")
	if k.ExitDate != "2020-07-02" || k.ExitReason != string(backtest.ExitStop) || !k.Gapped || !relEq(k.ExitPrice, want) {
		t.Fatalf("Kanso exit %s %s %v gapped=%v, want 2020-07-02 stop %v gapped", k.ExitDate, k.ExitReason, k.ExitPrice, k.Gapped, want)
	}
	for _, o := range readGolden(t).trades["daily_defaults/DIVISLAB"] {
		if o.EntryDate != "2020-04-13" {
			continue
		}
		if o.ExitDate != "2020-07-02" || o.ExitReason != string(backtest.ExitStop) || !relEq(o.ExitPrice, want) {
			t.Fatalf("oracle exit %s %s %v, want 2020-07-02 stop %v", o.ExitDate, o.ExitReason, o.ExitPrice, want)
		}
		return
	}
	t.Fatal("oracle has no DIVISLAB trade entered 2020-04-13")
}
