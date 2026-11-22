package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kanso/internal/data"
	"kanso/internal/market"
)

// Freshness is the summary line of a run (spec §6.4): the benchmark's
// latest date and the instruments whose latest bar is older.
func (s Summary) Freshness() string {
	noun := "instruments"
	if len(s.Behind) == 1 {
		noun = "instrument"
	}
	line := fmt.Sprintf("data as of %s; %d %s behind", s.AsOf, len(s.Behind), noun)
	if len(s.Behind) > 0 {
		line += ": " + strings.Join(s.Behind, ", ")
	}
	return line
}

// NeedsUpdate decides the -only-if-behind run on the morning of today: an
// update is needed if the benchmark has no bar for the previous weekday,
// or some member of the universe is behind the benchmark. The reason says
// which. A store not yet holding the benchmark or the universe needs one.
// Exchange holidays aren't known, so the morning after one runs an update
// that finds nothing new.
func NeedsUpdate(ctx context.Context, st *data.Store, universe string, today market.Date) (bool, string, error) {
	asOf, behind, err := Behind(ctx, st, universe)
	if errors.Is(err, market.ErrNotFound) {
		return true, err.Error(), nil
	}
	if err != nil {
		return false, "", err
	}
	if want := previousWeekday(today); asOf < want {
		return true, fmt.Sprintf("%s has no bar for %s (latest %s)", market.Benchmark, want, asOf), nil
	}
	if len(behind) > 0 {
		return true, Summary{AsOf: asOf, Behind: behind}.Freshness(), nil
	}
	return false, "", nil
}

// previousWeekday is the last Monday-to-Friday date before d.
func previousWeekday(d market.Date) market.Date {
	t := d.Time().AddDate(0, 0, -1)
	for t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		t = t.AddDate(0, 0, -1)
	}
	return market.Date(t.Format(market.DateLayout))
}

// Behind returns the benchmark's latest stored date and the members of the
// universe whose latest bar is older (or who have none), sorted by
// symbol. Data is "as of" the benchmark, never today's calendar.
func Behind(ctx context.Context, st *data.Store, universe string) (market.Date, []string, error) {
	bench, err := st.Instrument(ctx, market.Benchmark)
	if err != nil {
		return "", nil, err
	}
	asOf, err := st.LatestBarDate(ctx, bench.ID)
	if err != nil {
		return "", nil, err
	}
	members, _, err := st.Universe(ctx, universe)
	if err != nil {
		return "", nil, err
	}
	behind := []string{}
	for _, in := range members {
		latest, err := st.LatestBarDate(ctx, in.ID)
		if err != nil {
			return "", nil, err
		}
		if latest < asOf {
			behind = append(behind, in.Symbol)
		}
	}
	return asOf, behind, nil
}
