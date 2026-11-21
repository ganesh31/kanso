package patterns

import (
	"testing"
	"time"

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

	// volume_surge's window_weeks is the same stretch of calendar too: a
	// 2-week window holds only the quiet weeks on both timeframes, a 3-week
	// window reaches the loud week before them on both.
	vdaily := volumeWeeks(last+1, func(w, d int) float64 {
		switch {
		case w == last && d == 4:
			return 10000 // the evaluated Friday / week
		case w < last-2:
			return 100000 // loud weeks before the window
		}
		return 1000
	})
	vweekly := market.ToWeekly(vdaily)
	for _, tc := range []struct {
		weeks float64
		pass  bool
	}{{2, true}, {3, false}} {
		vp, err := Normalize(VolumeSurge{}.Meta(), Params{"k": 2.0, "window_weeks": tc.weeks})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range []struct {
			s  *market.Series
			at int
		}{
			{vdaily, vweekly.LastDailyIdx[last]},
			{vweekly, last},
		} {
			ctx := NewContext(c.s, nil)
			if err := (VolumeSurge{}).Prepare(ctx, vp); err != nil {
				t.Fatal(err)
			}
			r := VolumeSurge{}.Eval(ctx, vp, c.at)
			if !r.Evaluable || r.Pass != tc.pass {
				t.Errorf("%s, window_weeks %v: got %+v, want pass %v", c.s.Timeframe, tc.weeks, r, tc.pass)
			}
		}
	}
}

// volumeWeeks is n complete Mon–Fri weeks plus a forming one-day week, with
// day d of week w having volume vol(w, d).
func volumeWeeks(n int, vol func(w, d int) float64) *market.Series {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var bars []market.Bar
	for w := 0; w <= n; w++ {
		days := 5
		if w == n {
			days = 1
		}
		for d := range days {
			dt := market.Date(start.AddDate(0, 0, 7*w+d).Format(market.DateLayout))
			bars = append(bars, market.Bar{Date: dt, Open: 99, High: 100, Low: 99, Close: 99, Volume: vol(w, d)})
		}
	}
	return market.NewDaily(market.Instrument{ID: 1, Symbol: "TEST"}, bars)
}
