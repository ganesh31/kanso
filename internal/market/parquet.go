package market

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"github.com/parquet-go/parquet-go"
)

// parquetRow is one bar in the Parquet export: columns
// symbol,date,open,high,low,close,volume (ADR 0002: Parquet is an export
// format only, never a source).
type parquetRow struct {
	Symbol string  `parquet:"symbol"`
	Date   string  `parquet:"date"`
	Open   float64 `parquet:"open"`
	High   float64 `parquet:"high"`
	Low    float64 `parquet:"low"`
	Close  float64 `parquet:"close"`
	Volume float64 `parquet:"volume"`
}

// WriteParquet writes bars keyed by symbol as one Parquet file, rows sorted
// by symbol then date. Dates are YYYY-MM-DD strings; numbers are float64.
func WriteParquet(w io.Writer, bars map[string][]Bar) error {
	syms := make([]string, 0, len(bars))
	for s := range bars {
		syms = append(syms, s)
	}
	sort.Strings(syms)
	var rows []parquetRow
	for _, s := range syms {
		for _, b := range bars[s] {
			rows = append(rows, parquetRow{
				Symbol: s, Date: string(b.Date),
				Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume,
			})
		}
	}
	if err := parquet.Write(w, rows); err != nil {
		return fmt.Errorf("write parquet: %w", err)
	}
	return nil
}

// ReadParquet reads a file written by WriteParquet back into bars keyed by
// symbol. It exists to check exports and to build test fixture databases
// (data.Store.ImportParquet); Kanso never ingests real data from Parquet.
func ReadParquet(r io.Reader) (map[string][]Bar, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read parquet: %w", err)
	}
	rows, err := parquet.Read[parquetRow](bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, fmt.Errorf("read parquet: %w", err)
	}
	out := map[string][]Bar{}
	for _, row := range rows {
		out[row.Symbol] = append(out[row.Symbol], Bar{
			Date: Date(row.Date), Open: row.Open, High: row.High, Low: row.Low, Close: row.Close, Volume: row.Volume,
		})
	}
	return out, nil
}
