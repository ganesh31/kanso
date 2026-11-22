package api

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDemoModeSystemWrites403(t *testing.T) {
	srv := serverWith(t, Config{Demo: true})
	for _, c := range []struct{ method, path, body string }{
		{"POST", "/api/systems", breakoutSys},
		{"PUT", "/api/systems/00000000-0000-0000-0000-000000000000", breakoutSys},
		{"DELETE", "/api/systems/00000000-0000-0000-0000-000000000000", ""},
		// Refused before validation: an invalid body is still a 403.
		{"POST", "/api/systems", `{"name":""}`},
	} {
		status, body := do(t, srv, c.method, c.path, c.body)
		if status != 403 || !strings.Contains(string(body), `"status":403`) {
			t.Errorf("%s %s: %d %s", c.method, c.path, status, body)
		}
	}
	// Reads and computations still work.
	if status, body := do(t, srv, "GET", "/api/systems", ""); status != 200 {
		t.Errorf("list: %d %s", status, body)
	}
	if status, body := do(t, srv, "POST", "/api/scan", `{"system":`+breakoutSys+`}`); status != 200 {
		t.Errorf("scan: %d %s", status, body)
	}
}

// doFrom sends a request with an X-Forwarded-For header (none when xff is
// empty) and returns the response with its body read.
func doFrom(t *testing.T, srv *httptest.Server, xff, method, path, body string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if xff != "" {
		req.Header.Set("X-Forwarded-For", xff)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, b
}

func TestDemoRateLimit429(t *testing.T) {
	now := time.Date(2026, 11, 22, 10, 0, 0, 0, time.UTC)
	srv := serverWith(t, Config{Demo: true, RateLimit: 2, TrustedProxies: 1, now: func() time.Time { return now }})
	limited := []struct{ method, path, body string }{
		{"POST", "/api/backtest", breakoutSys},
		{"POST", "/api/scan", `{"system":` + breakoutSys + `}`},
		{"POST", "/api/quickscan", `{"pattern":"n_year_high","universe":"NIFTY50"}`},
		{"POST", "/api/report/BREAK", `{}`},
		{"GET", "/api/bars/BREAK", ""},
	}
	for i, c := range limited {
		ip := fmt.Sprintf("203.0.113.%d", i+1)
		for n := 1; n <= 2; n++ {
			// The proxy's hop is the client, whatever the client put before it.
			res, body := doFrom(t, srv, fmt.Sprintf("10.0.0.%d, %s", n, ip), c.method, c.path, c.body)
			if res.StatusCode != 200 {
				t.Fatalf("%s %s request %d: %d %s", c.method, c.path, n, res.StatusCode, body)
			}
		}
		res, body := doFrom(t, srv, ip, c.method, c.path, c.body)
		if res.StatusCode != 429 || res.Header.Get("Content-Type") != "application/problem+json" ||
			!strings.Contains(string(body), `"status":429`) || res.Header.Get("Retry-After") == "" {
			t.Fatalf("%s %s over the limit: %d %v %s", c.method, c.path, res.StatusCode, res.Header, body)
		}
	}

	// The limit is per client IP across the limited routes: 203.0.113.1
	// used it up on backtest, so bars is refused too, but another IP isn't.
	if res, _ := doFrom(t, srv, "203.0.113.1", "GET", "/api/bars/BREAK", ""); res.StatusCode != 429 {
		t.Fatalf("same IP, other route: %d", res.StatusCode)
	}
	if res, _ := doFrom(t, srv, "198.51.100.7", "GET", "/api/bars/BREAK", ""); res.StatusCode != 200 {
		t.Fatalf("other IP: %d", res.StatusCode)
	}
	// Routes outside the list are never limited.
	for range 3 {
		if res, _ := doFrom(t, srv, "203.0.113.1", "GET", "/api/patterns", ""); res.StatusCode != 200 {
			t.Fatalf("patterns: %d", res.StatusCode)
		}
	}
	// A minute later the IP may call again.
	now = now.Add(time.Minute)
	if res, body := doFrom(t, srv, "203.0.113.1", "GET", "/api/bars/BREAK", ""); res.StatusCode != 200 {
		t.Fatalf("after a minute: %d %s", res.StatusCode, body)
	}
}

func TestDemoRateLimitDefaultsTo60(t *testing.T) {
	srv := serverWith(t, Config{Demo: true})
	for n := 1; n <= 60; n++ {
		if res, _ := doFrom(t, srv, "", "GET", "/api/bars/BREAK", ""); res.StatusCode != 200 {
			t.Fatalf("request %d: %d", n, res.StatusCode)
		}
	}
	if res, _ := doFrom(t, srv, "", "GET", "/api/bars/BREAK", ""); res.StatusCode != 429 {
		t.Fatalf("request 61: %d", res.StatusCode)
	}
}

func TestNoRateLimitOutsideDemo(t *testing.T) {
	srv := serverWith(t, Config{})
	for n := 1; n <= 70; n++ {
		if res, _ := doFrom(t, srv, "", "GET", "/api/bars/BREAK", ""); res.StatusCode != 200 {
			t.Fatalf("request %d: %d", n, res.StatusCode)
		}
	}
}

func TestDemoRateLimitIgnoresForwardedForWithoutTrustedProxies(t *testing.T) {
	srv := serverWith(t, Config{Demo: true, RateLimit: 1})
	if res, _ := doFrom(t, srv, "203.0.113.1", "GET", "/api/bars/BREAK", ""); res.StatusCode != 200 {
		t.Fatalf("first request: %d", res.StatusCode)
	}
	// A new X-Forwarded-For value doesn't buy a new allowance: the header
	// is the client's own claim, so the connection's address is the key.
	if res, _ := doFrom(t, srv, "198.51.100.7", "GET", "/api/bars/BREAK", ""); res.StatusCode != 429 {
		t.Fatalf("spoofed header: %d, want 429", res.StatusCode)
	}
}

func TestClientIP(t *testing.T) {
	for _, tc := range []struct {
		xff     string
		proxies int
		want    string
	}{
		{"", 0, "192.0.2.9"},
		{"203.0.113.1", 0, "192.0.2.9"},
		{"6.6.6.6, 203.0.113.1", 1, "203.0.113.1"},
		{"6.6.6.6, 203.0.113.1, 10.0.0.1", 2, "203.0.113.1"},
		{"203.0.113.1", 2, "192.0.2.9"},
	} {
		if got := clientIP(tc.xff, "192.0.2.9:5555", tc.proxies); got != tc.want {
			t.Errorf("clientIP(%q, %d) = %s, want %s", tc.xff, tc.proxies, got, tc.want)
		}
	}
}
