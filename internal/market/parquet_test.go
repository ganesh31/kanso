package market

import (
	"bytes"
	"math"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParquetRoundTripExactFloat64(t *testing.T) {
	in := map[string][]Bar{
		"TCS": {
			{Date: "2024-01-01", Open: 0.1, High: 0.2, Low: 0.30000000000000004, Close: 1.0 / 3, Volume: 0},
			{Date: "2024-01-02", Open: math.Nextafter(100, 101), High: 1e300, Low: 5e-324, Close: 3456.7890123456789, Volume: 12345678},
		},
		"^NSEI": {{Date: "2024-01-01", Open: 21000.05, High: 21100.1, Low: 20950.95, Close: 21050.5, Volume: 0}},
	}
	var buf bytes.Buffer
	if err := WriteParquet(&buf, in); err != nil {
		t.Fatal(err)
	}
	got, err := ReadParquet(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(in, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}
