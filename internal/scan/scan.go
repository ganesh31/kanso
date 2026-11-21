// Package scan runs an entry rule over a stock list at one date: the
// Scanner, and Quick Scan for a single pattern.
package scan

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
	"sync"

	"golang.org/x/sync/errgroup"

	"kanso/internal/backtest"
	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// Mode picks which instruments a scan reports.
type Mode string

// Modes (design §7.5).
const (
	// Fresh reports instruments whose entry rule signalled (turned
	// false→true) within the last FreshBars bars.
	Fresh Mode = "fresh"
	// All reports every instrument whose rule is true on the as-of bar.
	All Mode = "all"
)

// Default fresh windows, in bars.
const (
	DefaultFreshDaily  = 5
	DefaultFreshWeekly = 1
)

// Match is one instrument that qualifies. The column pointers are nil when
// their condition is not in the rule or not evaluable.
type Match struct {
	Symbol string
	// Date is the bar evaluated: the as-of day, or on weekly the last
	// complete week (dated by its last trading day).
	Date market.Date
	// SignalDate is the latest signal within the fresh window (fresh mode).
	SignalDate market.Date
	// TrueSince is the first bar of the current unbroken pass streak (all
	// mode).
	TrueSince market.Date
	Outcome   rules.Outcome

	Level        *float64
	LevelDate    *market.Date
	LevelAgeBars *int
	PctAbove     *float64
	VolRatio     *float64
	RSDiff       *float64

	// Provisional is the rule on the week in progress (weekly only). It
	// never feeds a backtest.
	Provisional *rules.Outcome
}

// Request is a scan of a system's universe.
type Request struct {
	System rules.System
	// AsOf "" scans the latest benchmark date.
	AsOf market.Date
	// Mode "" is Fresh.
	Mode Mode
	// FreshBars 0 is 5 daily / 1 weekly.
	FreshBars int
}

// ScanResult is a scan's matches, sorted by % above the level (nulls
// last), then symbol.
type ScanResult struct {
	// AsOf is the date scanned, taken from the data.
	AsOf    market.Date
	Matches []Match
	// Stale lists instruments whose last bar is before AsOf (delisted,
	// suspended or behind); they are never matched.
	Stale []string
	// Skipped lists instruments with no bars up to AsOf.
	Skipped []string
	// AutoInserted lists condition types Quick added to the rule.
	AutoInserted []string
}

// Run scans req.System's universe. An invalid system returns a
// *rules.ValidationError.
func Run(ctx context.Context, prov market.Provider, req Request) (*ScanResult, error) {
	sys := req.System
	rules.ApplyDefaults(&sys)
	reg := patterns.Default()
	if err := rules.Validate(&sys, reg); err != nil {
		return nil, err
	}
	compiled, err := rules.Compile(sys.Entry, reg)
	if err != nil {
		return nil, err
	}
	mode := req.Mode
	if mode == "" {
		mode = Fresh
	}
	freshBars := req.FreshBars
	if freshBars <= 0 {
		freshBars = DefaultFreshDaily
		if sys.Timeframe == market.Weekly {
			freshBars = DefaultFreshWeekly
		}
	}

	members, _, err := prov.Universe(ctx, sys.Universe)
	if err != nil {
		return nil, err
	}
	bench, asOf, err := loadBenchmark(ctx, prov, benchmarkOf(sys.Entry), req.AsOf)
	if err != nil {
		return nil, err
	}
	if asOf == "" {
		if asOf, err = latestDate(ctx, prov, members); err != nil {
			return nil, err
		}
	}

	res := &ScanResult{AsOf: asOf, Matches: []Match{}, Stale: []string{}, Skipped: []string{}, AutoInserted: []string{}}
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(runtime.GOMAXPROCS(0))
	for _, inst := range members {
		g.Go(func() error {
			out, err := scanOne(gctx, prov, inst, bench, compiled, sys.Timeframe, asOf, mode, freshBars)
			if err != nil {
				return err
			}
			mu.Lock()
			defer mu.Unlock()
			switch {
			case out.skipped:
				res.Skipped = append(res.Skipped, inst.Symbol)
			case out.stale:
				res.Stale = append(res.Stale, inst.Symbol)
			case out.match != nil:
				res.Matches = append(res.Matches, *out.match)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	sortMatches(res.Matches)
	sort.Strings(res.Stale)
	sort.Strings(res.Skipped)
	return res, nil
}

type oneResult struct {
	match   *Match
	stale   bool
	skipped bool
}

// scanOne evaluates one instrument on bars up to asOf.
func scanOne(ctx context.Context, prov market.Provider, inst market.Instrument, bench *market.Series,
	c *rules.Compiled, tf market.Timeframe, asOf market.Date, mode Mode, freshBars int,
) (oneResult, error) {
	pctx, daily, err := patterns.Load(ctx, prov, inst, bench, tf, "", asOf)
	if err != nil {
		return oneResult{}, err
	}
	if len(daily.Bars) == 0 {
		return oneResult{skipped: true}, nil
	}
	if daily.Bars[len(daily.Bars)-1].Date < asOf {
		return oneResult{stale: true}, nil
	}
	if err := c.Prepare(pctx); err != nil {
		return oneResult{}, fmt.Errorf("%s: %w", inst.Symbol, err)
	}
	s := pctx.Series
	t := s.Complete - 1
	if t < 0 {
		return oneResult{}, nil
	}
	out := c.Eval(pctx, t)
	m := Match{Symbol: inst.Symbol, Date: s.Bars[t].Date, Outcome: out}

	switch mode {
	case Fresh:
		sigs := backtest.Signals(c, pctx)
		if len(sigs) == 0 || sigs[len(sigs)-1] <= t-freshBars {
			return oneResult{}, nil
		}
		m.SignalDate = s.Bars[sigs[len(sigs)-1]].Date
	default:
		if !out.Pass {
			return oneResult{}, nil
		}
		since := t
		for since > 0 && c.Eval(pctx, since-1).Pass {
			since--
		}
		m.TrueSince = s.Bars[since].Date
	}
	fillColumns(&m, out)

	if s.Complete < len(s.Bars) {
		p, err := provisional(pctx, c)
		if err != nil {
			return oneResult{}, fmt.Errorf("%s: %w", inst.Symbol, err)
		}
		m.Provisional = &p
	}
	return oneResult{match: &m}, nil
}

// provisional evaluates the rule on the bar in progress (the partial
// week) as if it were complete. Its context is separate, so the real one
// keeps never looking at incomplete bars.
func provisional(pctx *patterns.Context, c *rules.Compiled) (rules.Outcome, error) {
	s := *pctx.Series
	s.Complete = len(s.Bars)
	pc := patterns.NewContext(&s, pctx.Bench)
	if err := c.Prepare(pc); err != nil {
		return rules.Outcome{}, err
	}
	return c.Eval(pc, len(s.Bars)-1), nil
}

// fillColumns copies the table columns from the evaluated conditions'
// evidence.
func fillColumns(m *Match, out rules.Outcome) {
	for _, co := range out.Conditions {
		if !co.Result.Evaluable {
			continue
		}
		switch ev := co.Result.Evidence.(type) {
		case patterns.NYearHighEvidence:
			m.Level, m.LevelDate, m.LevelAgeBars = ptr(ev.Level), ptr(ev.LevelDate), ptr(ev.BarsSince)
			m.PctAbove = ptr(ev.PctAbove)
		case patterns.NotExtendedEvidence:
			if m.PctAbove == nil {
				m.PctAbove = ptr(ev.PctAbove)
			}
		case patterns.LevelAgeEvidence:
			if m.LevelAgeBars == nil {
				m.LevelDate, m.LevelAgeBars = ptr(ev.LevelDate), ptr(ev.BarsSince)
			}
		case patterns.VolumeEvidence:
			m.VolRatio = ptr(ev.Ratio)
		case patterns.RSEvidence:
			m.RSDiff = ptr(ev.Diff)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// sortMatches orders by % above the level ascending (nil last), then
// symbol.
func sortMatches(ms []Match) {
	sort.SliceStable(ms, func(i, j int) bool {
		a, b := ms[i].PctAbove, ms[j].PctAbove
		if (a == nil) != (b == nil) {
			return a != nil
		}
		if a != nil && *a != *b {
			return *a < *b
		}
		return ms[i].Symbol < ms[j].Symbol
	})
}

// benchmarkOf is the benchmark the rule's relative strength uses, or the
// default (Nifty 50).
func benchmarkOf(e rules.Entry) string {
	for _, c := range append(append([]rules.Condition{}, e.All...), e.Any...) {
		if c.Type == patterns.TypeRelativeStrength {
			if b := c.Params.String("benchmark"); b != "" {
				return b
			}
		}
	}
	return patterns.DefaultBenchmark
}

// loadBenchmark loads the benchmark's daily bars up to the as-of date and
// resolves that date: the benchmark's last bar on or before want (its
// latest bar when want is ""). A missing benchmark is not an error: the
// bench is nil and the date is want, or "" to be resolved from members.
func loadBenchmark(ctx context.Context, prov market.Provider, symbol string, want market.Date) (*market.Series, market.Date, error) {
	inst, err := prov.Instrument(ctx, symbol)
	if errors.Is(err, market.ErrNotFound) {
		return nil, want, nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("benchmark %s: %w", symbol, err)
	}
	to := want
	if to == "" {
		to = "9999-12-31"
	}
	bars, err := prov.Daily(ctx, inst.ID, "", to)
	if err != nil {
		return nil, "", fmt.Errorf("benchmark %s: %w", symbol, err)
	}
	bench := market.NewDaily(inst, bars)
	if len(bars) == 0 {
		return bench, want, nil
	}
	return bench, bars[len(bars)-1].Date, nil
}

// latestDate is the latest bar date across instruments, for a scan with
// no benchmark data.
func latestDate(ctx context.Context, prov market.Provider, insts []market.Instrument) (market.Date, error) {
	var latest market.Date
	for _, in := range insts {
		bars, err := prov.Daily(ctx, in.ID, "", "9999-12-31")
		if err != nil {
			return "", err
		}
		if n := len(bars); n > 0 && bars[n-1].Date > latest {
			latest = bars[n-1].Date
		}
	}
	return latest, nil
}
