package market

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
)

var csvColumns = []string{"symbol", "date", "open", "high", "low", "close", "volume"}

// ReadCSV reads bars with the columns symbol,date,open,high,low,close,volume
// (header required, in any order) and groups them by symbol, each slice
// sorted ascending by date.
func ReadCSV(r io.Reader) (map[string][]Bar, error) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}
	col := make(map[string]int, len(header))
	for i, h := range header {
		col[h] = i
	}
	for _, c := range csvColumns {
		if _, ok := col[c]; !ok {
			return nil, fmt.Errorf("csv header: missing column %q", c)
		}
	}

	out := map[string][]Bar{}
	line := 1
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		line++
		if err != nil {
			return nil, fmt.Errorf("csv line %d: %w", line, err)
		}
		d := Date(rec[col["date"]])
		if !d.Valid() {
			return nil, fmt.Errorf("csv line %d: bad date %q", line, d)
		}
		var nums [5]float64
		for i, c := range csvColumns[2:] {
			v, err := strconv.ParseFloat(rec[col[c]], 64)
			if err != nil {
				return nil, fmt.Errorf("csv line %d: bad %s %q", line, c, rec[col[c]])
			}
			nums[i] = v
		}
		sym := rec[col["symbol"]]
		out[sym] = append(out[sym], Bar{
			Date: d, Open: nums[0], High: nums[1], Low: nums[2], Close: nums[3], Volume: nums[4],
		})
	}
	for _, bars := range out {
		sort.SliceStable(bars, func(i, j int) bool { return bars[i].Date < bars[j].Date })
	}
	return out, nil
}
