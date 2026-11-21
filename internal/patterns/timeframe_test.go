package patterns

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
)

// TestSameParamsMeanSameDurationOnBothTimeframes flips one saved set of
// params from daily to weekly (grill Q6): the params are unchanged, and
// "1 year" covers the same stretch of calendar on both timeframes.
func TestSameParamsMeanSameDurationOnBothTimeframes(t *testing.T) {
	const last = 103 // last complete week of 104
	highs := map[int]float64{
		last - 48: 130, // 48 weeks before the evaluated week: inside 1 year
		last - 55: 150, // 55 weeks before: outside 1 year
	}
	daily := dailyWeeks(last+1, highs)
	weekly := market.ToWeekly(daily)

	p, err := Normalize(NYearHigh{}.Meta(), Params{"years": 1.0})
	if err != nil {
		t.Fatal(err)
	}
	saved := Params{"years": 1.0, "all_time": false}

	cases := []struct {
		s  *market.Series
		at int
	}{
		{daily, weekly.LastDailyIdx[last]}, // the same Friday on both
		{weekly, last},
	}
	for _, c := range cases {
		r, _ := evalNYH(t, c.s, p, c.at)
		if ev, ok := r.Evidence.(NYearHighEvidence); !ok || ev.Level != 130 {
			t.Errorf("%s: got %+v, want level 130 (48 weeks ago in, 55 weeks ago out)", c.s.Timeframe, r)
		}
		if diff := cmp.Diff(saved, p); diff != "" {
			t.Errorf("%s: params changed (-want +got):\n%s", c.s.Timeframe, diff)
		}
	}
}
