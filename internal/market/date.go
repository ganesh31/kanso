// Package market holds the core market data types: dates, bars,
// instruments and series, plus the Provider that loads them.
package market

import "time"

// DateLayout is the layout of a Date: an IST trading date.
const DateLayout = "2006-01-02"

// Date is an IST trading date in YYYY-MM-DD form. Dates compare as strings.
type Date string

// Time parses the date as midnight UTC. The zero time is returned for an
// invalid date.
func (d Date) Time() time.Time {
	t, err := time.Parse(DateLayout, string(d))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Valid reports whether d is a real calendar date in YYYY-MM-DD form.
func (d Date) Valid() bool {
	t, err := time.Parse(DateLayout, string(d))
	return err == nil && t.Format(DateLayout) == string(d)
}
