package market

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestReadCSVGroupsBySymbolSortedByDate(t *testing.T) {
	in := `symbol,date,open,high,low,close,volume
TCS,2024-01-03,3,4,2,3.5,300
INFY,2024-01-02,10,11,9,10.5,1000
TCS,2024-01-01,1,2,0.5,1.5,100
TCS,2024-01-02,2,3,1,2.5,200
`
	got, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]Bar{
		"TCS": {
			{Date: "2024-01-01", Open: 1, High: 2, Low: 0.5, Close: 1.5, Volume: 100},
			{Date: "2024-01-02", Open: 2, High: 3, Low: 1, Close: 2.5, Volume: 200},
			{Date: "2024-01-03", Open: 3, High: 4, Low: 2, Close: 3.5, Volume: 300},
		},
		"INFY": {{Date: "2024-01-02", Open: 10, High: 11, Low: 9, Close: 10.5, Volume: 1000}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ReadCSV mismatch (-want +got):\n%s", diff)
	}
}

func TestReadCSVRejectsBadRows(t *testing.T) {
	cases := map[string]string{
		"missing column": "symbol,date,open,high,low,close\nTCS,2024-01-01,1,2,0.5,1.5\n",
		"bad number":     "symbol,date,open,high,low,close,volume\nTCS,2024-01-01,x,2,0.5,1.5,100\n",
		"bad date":       "symbol,date,open,high,low,close,volume\nTCS,01/01/2024,1,2,0.5,1.5,100\n",
		"NaN price":      "symbol,date,open,high,low,close,volume\nTCS,2024-01-01,1,2,0.5,NaN,100\n",
		"Inf volume":     "symbol,date,open,high,low,close,volume\nTCS,2024-01-01,1,2,0.5,1.5,+Inf\n",
		"duplicate date": "symbol,date,open,high,low,close,volume\nTCS,2024-01-01,1,2,0.5,1.5,100\nTCS,2024-01-01,1,2,0.5,1.6,100\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadCSV(strings.NewReader(in)); err == nil {
				t.Fatal("want error, got nil")
			}
		})
	}
}

func TestDateTime(t *testing.T) {
	d := Date("2024-12-30")
	if got := d.Time().Format("2006-01-02"); got != "2024-12-30" {
		t.Fatalf("Time() = %s", got)
	}
	if !d.Valid() || Date("2024-13-01").Valid() {
		t.Fatal("Valid() wrong")
	}
}

func TestReadCSVAllowsSymbolsToShareADate(t *testing.T) {
	in := "symbol,date,open,high,low,close,volume\nTCS,2024-01-01,1,2,0.5,1.5,100\nINFY,2024-01-01,1,2,0.5,1.5,100\n"
	got, err := ReadCSV(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	if len(got["TCS"]) != 1 || len(got["INFY"]) != 1 {
		t.Fatalf("got %v", got)
	}
}
