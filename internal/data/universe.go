package data

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ReadNSEConstituents reads the Symbol column of an NSE index constituent
// CSV (niftyindices.com, e.g. ind_nifty500list.csv), in file order. Empty
// symbols are skipped.
func ReadNSEConstituents(r io.Reader) ([]string, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	header, err := cr.Read()
	if err != nil {
		return nil, fmt.Errorf("constituents header: %w", err)
	}
	col := -1
	for i, h := range header {
		if strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")), "Symbol") {
			col = i
		}
	}
	if col < 0 {
		return nil, errors.New("constituents: no Symbol column")
	}
	var out []string
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("constituents: %w", err)
		}
		if col < len(rec) {
			if s := strings.TrimSpace(rec[col]); s != "" {
				out = append(out, s)
			}
		}
	}
}
