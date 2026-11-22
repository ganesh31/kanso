package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"

	"kanso/internal/market"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseChartDropsNullsAndDedupes(t *testing.T) {
	bars, splits, err := ParseChart(fixture(t, "chart_nulls_dupes.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []market.Bar{
		{Date: "2024-01-05", Open: 100.5, High: 101.25, Low: 99.75, Close: 100, Volume: 0}, // zero volume kept
		// 2024-01-08 has a null close: dropped.
		{Date: "2024-01-09", Open: 103, High: 104, Low: 102, Close: 103.5, Volume: 7000}, // duplicate: last kept
	}
	if diff := cmp.Diff(want, bars); diff != "" {
		t.Fatalf("bars (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]Split{{Date: "2024-01-09", Numerator: 2, Denominator: 1}}, splits); diff != "" {
		t.Fatalf("splits (-want +got):\n%s", diff)
	}
}

func TestParseChartISTDate(t *testing.T) {
	// 2024-01-04 19:00 UTC is 00:30 on the 5th in IST; 03:45 UTC on the
	// 5th is 09:15 IST. Both are IST trading date 2024-01-05, and the
	// duplicate keeps the later row. A null volume reads as 0.
	bars, _, err := ParseChart(fixture(t, "chart_ist.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []market.Bar{{Date: "2024-01-05", Open: 21700, High: 21800, Low: 21600, Close: 21750, Volume: 0}}
	if diff := cmp.Diff(want, bars); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestParseChartError(t *testing.T) {
	body := `{"chart":{"result":null,"error":{"code":"Not Found","description":"No data found, symbol may be delisted"}}}`
	if _, _, err := ParseChart([]byte(body)); err == nil || !strings.Contains(err.Error(), "delisted") {
		t.Fatalf("err = %v, want the chart error", err)
	}
}

// chartServer answers with statuses in turn (200 serves body), repeating
// the last one, and counts requests.
func chartServer(t *testing.T, body []byte, statuses ...int) (*httptest.Server, *atomic.Int32, *atomic.Value) {
	t.Helper()
	var n atomic.Int32
	var last atomic.Value // *http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(n.Add(1)) - 1
		last.Store(r.Clone(context.Background()))
		st := statuses[min(i, len(statuses)-1)]
		w.WriteHeader(st)
		if st == http.StatusOK {
			_, _ = w.Write(body)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &n, &last
}

func testYahoo(srv *httptest.Server, retries int) *Yahoo {
	return &Yahoo{BaseURL: srv.URL, Client: srv.Client(), MaxRetries: retries, BaseDelay: 1}
}

func TestYahooRetriesOn429(t *testing.T) {
	srv, n, _ := chartServer(t, fixture(t, "chart_nulls_dupes.json"), 429, 429, 200)
	bars, splits, err := testYahoo(srv, 3).Daily(context.Background(), "TCS.NS", "2024-01-01", "2024-01-31")
	if err != nil {
		t.Fatal(err)
	}
	if n.Load() != 3 || len(bars) != 2 || len(splits) != 1 {
		t.Fatalf("requests = %d, bars = %d, splits = %d; want 3, 2, 1", n.Load(), len(bars), len(splits))
	}
}

func TestYahooGivesUpAfterMaxRetries(t *testing.T) {
	srv, n, _ := chartServer(t, nil, 503)
	bars, splits, err := testYahoo(srv, 3).Daily(context.Background(), "TCS.NS", "2024-01-01", "2024-01-31")
	if err == nil || bars != nil || splits != nil {
		t.Fatalf("got %v, %v, %v; want an error and no data", bars, splits, err)
	}
	if n.Load() != 4 {
		t.Fatalf("requests = %d, want 4 (1 try + 3 retries)", n.Load())
	}
}

func TestYahooDoesNotRetryNotFound(t *testing.T) {
	srv, n, _ := chartServer(t, nil, 404)
	if _, _, err := testYahoo(srv, 3).Daily(context.Background(), "NOPE.NS", "2024-01-01", "2024-01-31"); err == nil {
		t.Fatal("want error")
	}
	if n.Load() != 1 {
		t.Fatalf("requests = %d, want 1", n.Load())
	}
}

func TestYahooSendsGenericUserAgent(t *testing.T) {
	srv, _, last := chartServer(t, fixture(t, "chart_ist.json"), 200)
	if _, _, err := testYahoo(srv, 0).Daily(context.Background(), "^NSEI", "2024-01-01", "2024-01-31"); err != nil {
		t.Fatal(err)
	}
	r := last.Load().(*http.Request)
	if ua := r.Header.Get("User-Agent"); ua != "kanso/0.1" {
		t.Fatalf("User-Agent = %q", ua)
	}
	if r.URL.EscapedPath() != "/v8/finance/chart/%5ENSEI" {
		t.Fatalf("path = %s", r.URL.EscapedPath())
	}
	q := r.URL.Query()
	// period1 is 2024-01-01 00:00 IST; period2 is the end of 2024-01-31 IST.
	if q.Get("period1") != "1704047400" || q.Get("period2") != "1706725800" || q.Get("interval") != "1d" || q.Get("events") != "split" {
		t.Fatalf("query = %v", q)
	}
}

func TestReadNSEConstituents(t *testing.T) {
	f, err := os.Open("testdata/ind_nifty50list.csv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	got, err := ReadNSEConstituents(f)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"RELIANCE", "TCS", "BAJAJ-AUTO", "M&M"}, got); diff != "" {
		t.Fatalf("(-want +got):\n%s", diff)
	}
}

func TestReadNSEConstituentsBOMAndMissingColumn(t *testing.T) {
	got, err := ReadNSEConstituents(strings.NewReader("\ufeffCompany Name, Symbol \nTata,TCS\n,\n"))
	if err != nil || !cmp.Equal(got, []string{"TCS"}) {
		t.Fatalf("got %v, %v", got, err)
	}
	if _, err := ReadNSEConstituents(strings.NewReader("Company Name\nTata\n")); err == nil {
		t.Fatal("missing Symbol column: want error")
	}
}
