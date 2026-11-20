package patterns

import (
	"math"

	"kanso/internal/market"
)

// Unit is the calendar unit a duration param is stored in (spec §5.3).
// No param is ever stored in bars.
type Unit string

// Units. NoUnit marks a param that is not a duration.
const (
	NoUnit Unit = ""
	Weeks  Unit = "weeks"
	Months Unit = "months"
	Years  Unit = "years"
)

// ToBars converts n calendar units to a bar count on a timeframe, rounded
// to the nearest bar. Daily: week 5, month 21, year 252 bars. Weekly:
// week 1, month round(52/12 × n), year 52 bars.
func ToBars(n float64, u Unit, tf market.Timeframe) int {
	var perUnit float64
	switch tf {
	case market.Weekly:
		switch u {
		case Weeks:
			perUnit = 1
		case Months:
			perUnit = 52.0 / 12.0
		case Years:
			perUnit = 52
		}
	default:
		switch u {
		case Weeks:
			perUnit = 5
		case Months:
			perUnit = 21
		case Years:
			perUnit = 252
		}
	}
	if perUnit == 0 {
		return int(math.Round(n))
	}
	return int(math.Round(n * perUnit))
}
