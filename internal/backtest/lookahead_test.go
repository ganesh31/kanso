package backtest

import (
	"context"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
	"kanso/internal/patterns"
	"kanso/internal/rules"
)

// randomWalk returns n daily bars on weekdays from Monday 2015-01-05: a
// drifting log-normal walk with whole weeks of tripled volume now and
// then, so the default system's breakout and volume conditions both fire
// on daily and on weekly bars.
func randomWalk(r *rand.Rand, n int, drift float64) []market.Bar {
	bars := make([]market.Bar, 0, n)
	d := time.Date(2015, 1, 5, 0, 0, 0, 0, time.UTC)
	price := 100.0
	spike := 1.0
	for len(bars) < n {
		if wd := d.Weekday(); wd != time.Saturday && wd != time.Sunday {
			if wd == time.Monday {
				spike = 1
				if r.Float64() < 0.2 {
					spike = 3
				}
			}
			open := price * math.Exp(r.NormFloat64()*0.005)
			price *= math.Exp(drift + r.NormFloat64()*0.02)
			vol := 1e6 * spike * math.Exp(r.NormFloat64()*0.4)
			bars = append(bars, market.Bar{
				Date: market.Date(d.Format(market.DateLayout)), Open: open, Close: price,
				High:   max(open, price) * (1 + math.Abs(r.NormFloat64())*0.01),
				Low:    min(open, price) * (1 - math.Abs(r.NormFloat64())*0.01),
				Volume: math.Round(vol),
			})
		}
		d = d.AddDate(0, 0, 1)
	}
	return bars
}

// signalDates loads RW through the production loading path with data up
// to and including date to, and returns the default system's signal
// dates, plus the date of the last complete bar.
func signalDates(t *testing.T, prov market.Provider, c *rules.Compiled, tf market.Timeframe, to market.Date) ([]market.Date, market.Date) {
	t.Helper()
	ctx := context.Background()
	bi, _ := prov.Instrument(ctx, market.Benchmark)
	bb, err := prov.Daily(ctx, bi.ID, "", to)
	if err != nil {
		t.Fatal(err)
	}
	inst, _ := prov.Instrument(ctx, "RW")
	pctx, _, err := patterns.Load(ctx, prov, inst, market.NewDaily(bi, bb), tf, "", to)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Prepare(pctx); err != nil {
		t.Fatal(err)
	}
	dates := []market.Date{}
	for _, i := range Signals(c, pctx) {
		dates = append(dates, pctx.Series.Bars[i].Date)
	}
	var lastComplete market.Date
	if s := pctx.Series; s.Complete > 0 {
		lastComplete = s.Bars[s.Complete-1].Date
	}
	return dates, lastComplete
}

// TestNoLookAheadProperty: signals on data cut at a random bar equal the
// full data's signals up to the cut (spec §8). On weekly, only weeks
// complete in the cut data are compared.
func TestNoLookAheadProperty(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	const n = 2000
	stock, bench := randomWalk(r, n, 0.0015), randomWalk(r, n, 0.0003)
	prov := market.MemProviderFromBars(map[string][]market.Bar{"RW": stock, market.Benchmark: bench})

	for _, tf := range []market.Timeframe{market.Daily, market.Weekly} {
		t.Run(string(tf), func(t *testing.T) {
			sys := rules.DefaultSystem()
			sys.Timeframe = tf
			if err := rules.Validate(&sys, patterns.Default()); err != nil {
				t.Fatal(err)
			}
			c, err := rules.Compile(sys.Entry, patterns.Default())
			if err != nil {
				t.Fatal(err)
			}
			full, _ := signalDates(t, prov, c, tf, stock[n-1].Date)
			if len(full) < 3 {
				t.Fatalf("only %d signals on the full walk; the property would be vacuous", len(full))
			}
			t.Logf("%d signals on the full walk", len(full))
			cuts := rand.New(rand.NewPCG(42, uint64(len(tf))))
			for range 200 {
				cut := 1 + cuts.IntN(n-1) // bars 0..cut-1 survive
				to := stock[cut-1].Date
				got, lastComplete := signalDates(t, prov, c, tf, to)
				want := []market.Date{}
				for _, d := range full {
					if d <= lastComplete {
						want = append(want, d)
					}
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Fatalf("cut after %s: prefix signals differ from the full run (-full +prefix):\n%s", to, diff)
				}
			}
		})
	}
}
