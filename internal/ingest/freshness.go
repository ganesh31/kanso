package ingest

import (
	"context"
	"fmt"
	"strings"

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
