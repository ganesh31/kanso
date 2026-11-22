package scan

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// days returns n weekday dates starting Monday 2024-01-01.
func days(n int) []market.Date {
	out := make([]market.Date, 0, n)
	for d := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC); len(out) < n; d = d.AddDate(0, 0, 1) {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			out = append(out, market.Date(d.Format(market.DateLayout)))
		}
	}
	return out
}

// flat returns n bars on days(n) at 100 with volume 100.
func flat(n int) []market.Bar {
	ds := days(n)
	bars := make([]market.Bar, n)
	for i := range bars {
		bars[i] = market.Bar{Date: ds[i], Open: 100, High: 100, Low: 100, Close: 100, Volume: 100}
	}
	return bars
}

// withVolume sets the volume of bars from..to (inclusive).
func withVolume(bars []market.Bar, from, to int, v float64) []market.Bar {
	for i := from; i <= to; i++ {
		bars[i].Volume = v
	}
	return bars
}

// provider serves the given equities as universe "U" plus a flat ^NSEI
// benchmark of benchBars bars.
func provider(benchBars int, stocks map[string][]market.Bar) market.Provider {
	all := map[string][]market.Bar{patterns.DefaultBenchmark: flat(benchBars)}
	for s, b := range stocks {
		all[s] = b
	}
	p := market.MemProviderFromBars(all)
	syms, _, _ := p.Universe(context.Background(), "ALL")
	names := make([]string, len(syms))
	for i, in := range syms {
		names[i] = in.Symbol
	}
	insts := make([]market.Instrument, 0, len(all))
	for s := range all {
		in, _ := p.Instrument(context.Background(), s)
		insts = append(insts, in)
	}
	return market.NewMemProvider(insts, all, map[string][]string{"U": names})
}

func system(tf market.Timeframe, all ...rules.Condition) rules.System {
	s := rules.System{Name: "t", Universe: "U", Timeframe: tf, Entry: rules.Entry{All: all}}
	rules.ApplyDefaults(&s)
	return s
}

var volumeOnly = rules.Condition{Type: patterns.TypeVolumeSurge}

var cmpApprox = cmpopts.EquateApprox(1e-9, 0)

func asValidation(err error, target **rules.ValidationError) bool { return errors.As(err, target) }

func run(t *testing.T, prov market.Provider, req Request) *ScanResult {
	t.Helper()
	res, err := Run(context.Background(), prov, req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func symbols(ms []Match) []string {
	out := []string{}
	for _, m := range ms {
		out = append(out, m.Symbol)
	}
	return out
}

// recentAndOld: RECENT surges once 3 bars before the last bar; OLD has
// surged for the last 8 bars (signal 7 bars ago, still true).
func recentAndOld() market.Provider {
	return provider(100, map[string][]market.Bar{
		"RECENT": withVolume(flat(100), 96, 96, 300),
		"OLD":    withVolume(flat(100), 92, 99, 300),
	})
}

func TestScanFreshFindsRecentSignalOnly(t *testing.T) {
	ds := days(100)
	res := run(t, recentAndOld(), Request{System: system(market.Daily, volumeOnly), Mode: Fresh})
	if diff := cmp.Diff([]string{"RECENT"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
	m := res.Matches[0]
	if m.SignalDate != ds[96] || m.Date != ds[99] || res.AsOf != ds[99] {
		t.Fatalf("got signal %s, date %s, as of %s", m.SignalDate, m.Date, res.AsOf)
	}
}

func TestScanAllModeTrueSince(t *testing.T) {
	ds := days(100)
	res := run(t, recentAndOld(), Request{System: system(market.Daily, volumeOnly), Mode: All})
	if diff := cmp.Diff([]string{"OLD"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
	m := res.Matches[0]
	if m.TrueSince != ds[92] || !m.Outcome.Pass {
		t.Fatalf("got true since %s, outcome %+v", m.TrueSince, m.Outcome)
	}
}

func TestScanFreshBarsOverride(t *testing.T) {
	res := run(t, recentAndOld(), Request{System: system(market.Daily, volumeOnly), Mode: Fresh, FreshBars: 8})
	if diff := cmp.Diff([]string{"OLD", "RECENT"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
}

func TestScanAsOfEarlierDate(t *testing.T) {
	ds := days(100)
	// On bar 96 RECENT has just signalled and OLD is still true.
	res := run(t, recentAndOld(), Request{System: system(market.Daily, volumeOnly), Mode: All, AsOf: ds[96]})
	if diff := cmp.Diff([]string{"OLD", "RECENT"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
	if res.AsOf != ds[96] || len(res.Stale) != 0 {
		t.Fatalf("as of %s, stale %v", res.AsOf, res.Stale)
	}
}

func TestScanWeeklyUsesLastCompleteWeekAndProvisional(t *testing.T) {
	// 20 full weeks plus Monday and Tuesday of week 21. Week 19 (the last
	// complete one, bars 95..99) surges; the week in progress does not.
	n := 20*5 + 2
	ds := days(n)
	prov := provider(n, map[string][]market.Bar{"W": withVolume(flat(n), 95, 99, 300)})
	res := run(t, prov, Request{System: system(market.Weekly, volumeOnly), Mode: Fresh})
	if res.AsOf != ds[n-1] {
		t.Fatalf("as of %s, want %s", res.AsOf, ds[n-1])
	}
	if diff := cmp.Diff([]string{"W"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
	m := res.Matches[0]
	if m.Date != ds[99] || m.SignalDate != ds[99] {
		t.Fatalf("date %s signal %s, want the last complete week %s", m.Date, m.SignalDate, ds[99])
	}
	if m.Provisional == nil || m.Provisional.Pass || !m.Provisional.Evaluable {
		t.Fatalf("provisional %+v, want an evaluable fail for the week in progress", m.Provisional)
	}
}

func TestScanExcludesStaleInstrument(t *testing.T) {
	prov := provider(100, map[string][]market.Bar{
		"FRESH": withVolume(flat(100), 99, 99, 300),
		"STALE": withVolume(flat(99), 98, 98, 300), // last bar a day behind the bench
	})
	res := run(t, prov, Request{System: system(market.Daily, volumeOnly), Mode: Fresh})
	if diff := cmp.Diff([]string{"FRESH"}, symbols(res.Matches)); diff != "" {
		t.Fatalf("matches (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"STALE"}, res.Stale); diff != "" {
		t.Fatalf("stale (-want +got):\n%s", diff)
	}
}

func TestScanShortHistoryInstrument(t *testing.T) {
	ds := days(1000)
	short := withVolume(flat(1000)[900:], 99, 99, 300) // 100 bars, listed late
	short[99].Close, short[99].High = 200, 200
	prov := provider(1000, map[string][]market.Bar{
		"SHORT": short,
		"TINY":  flat(1000)[999:], // one bar
		"NONE":  {},
	})
	for _, mode := range []Mode{Fresh, All} {
		res := run(t, prov, Request{System: system(market.Daily, rules.Condition{Type: patterns.TypeNYearHigh}, volumeOnly), Mode: mode})
		if len(res.Matches) != 0 {
			t.Fatalf("%s: matches %+v, want none", mode, symbols(res.Matches))
		}
		if diff := cmp.Diff([]string{"NONE"}, res.Skipped); diff != "" {
			t.Fatalf("%s: skipped (-want +got):\n%s", mode, diff)
		}
		if res.AsOf != ds[999] {
			t.Fatalf("as of %s", res.AsOf)
		}
	}
}

func TestScanColumnsFromEvidence(t *testing.T) {
	// 300 flat bars; the last closes at 105 on triple volume. The 1-year
	// level is 100, set at bar 47 (the earliest of equal highs in the
	// window 47..298). The bench is flat, so RS diff is the stock's 5%.
	bars := withVolume(flat(300), 299, 299, 300)
	bars[299].Close, bars[299].High = 105, 105
	prov := provider(300, map[string][]market.Bar{"COL": bars})
	sys := system(market.Daily,
		rules.Condition{Type: patterns.TypeNYearHigh, Params: patterns.Params{"years": 1.0}},
		rules.Condition{Type: patterns.TypeNotExtended},
		rules.Condition{Type: patterns.TypeRelativeStrength},
		volumeOnly,
	)
	res := run(t, prov, Request{System: sys, Mode: All})
	if len(res.Matches) != 1 {
		t.Fatalf("matches %+v", res.Matches)
	}
	m := res.Matches[0]
	ds := days(300)
	f := func(v float64) *float64 { return &v }
	want := Match{
		Symbol: "COL", Date: ds[299], TrueSince: ds[299],
		Level: f(100), LevelDate: &ds[47], LevelAgeBars: new(252), PctAbove: f(5), VolRatio: f(3), RSDiff: f(0.05),
	}
	got := m
	got.Outcome = rules.Outcome{}
	if diff := cmp.Diff(want, got, cmpApprox); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestScanColumnsNilWhenConditionAbsent(t *testing.T) {
	res := run(t, recentAndOld(), Request{System: system(market.Daily, volumeOnly), Mode: All})
	m := res.Matches[0]
	if m.Level != nil || m.PctAbove != nil || m.RSDiff != nil || m.LevelDate != nil || m.LevelAgeBars != nil || m.VolRatio == nil {
		t.Fatalf("got %+v", m)
	}
}

func TestScanDeterministicOrder(t *testing.T) {
	// Closes 102, 105 and 102 over a 1-year level of 100: A (5%) sorts
	// after M and Z (2%), which tie and sort by symbol.
	mk := func(c float64) []market.Bar {
		b := flat(300)
		b[299].Close, b[299].High = c, c
		return b
	}
	prov := provider(300, map[string][]market.Bar{"Z": mk(102), "A": mk(105), "M": mk(102), "FLAT": flat(300)})
	sys := system(market.Daily, rules.Condition{Type: patterns.TypeNYearHigh, Params: patterns.Params{"years": 1.0}})
	for range 5 {
		res := run(t, prov, Request{System: sys, Mode: All})
		if diff := cmp.Diff([]string{"M", "Z", "A"}, symbols(res.Matches)); diff != "" {
			t.Fatalf("(-want +got):\n%s", diff)
		}
	}
}

func TestSortMatchesNilPctAboveLast(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	ms := []Match{{Symbol: "B"}, {Symbol: "C", PctAbove: f(3)}, {Symbol: "A"}, {Symbol: "D", PctAbove: f(1)}}
	sortMatches(ms)
	if diff := cmp.Diff([]string{"D", "C", "A", "B"}, symbols(ms)); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestScanRejectsInvalidSystem(t *testing.T) {
	sys := system(market.Daily, rules.Condition{Type: patterns.TypeNotExtended})
	_, err := Run(context.Background(), recentAndOld(), Request{System: sys})
	var ve *rules.ValidationError
	if !asValidation(err, &ve) {
		t.Fatalf("got %v, want *rules.ValidationError", err)
	}
}

func TestScanUnknownUniverse(t *testing.T) {
	sys := system(market.Daily, volumeOnly)
	sys.Universe = "NOPE"
	if _, err := Run(context.Background(), recentAndOld(), Request{System: sys}); err == nil {
		t.Fatal("want error")
	}
}
