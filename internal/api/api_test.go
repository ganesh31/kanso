package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"kanso/internal/market"
)

func date(i int) market.Date {
	return market.Date(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, i).Format(market.DateLayout))
}

// server serves BREAK: 800 flat bars with a 130 high at bar 300 and a
// close of 131 on the last bar (a new 3-year high).
func server(t *testing.T) *httptest.Server {
	t.Helper()
	bars := make([]market.Bar, 800)
	for i := range bars {
		bars[i] = market.Bar{Date: date(i), Open: 99, High: 100, Low: 98, Close: 99, Volume: 1}
	}
	bars[300].High = 130
	bars[799].Close, bars[799].High = 131, 132
	prov := market.NewMemProvider(
		[]market.Instrument{{ID: 1, Exchange: "NSE", Symbol: "BREAK", Type: market.Equity, LotSize: 1}},
		map[string][]market.Bar{"BREAK": bars}, nil)
	static := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html><title>Kanso</title>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	srv := httptest.NewServer(New(prov, Config{Static: static}))
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
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
	return res.StatusCode, b
}

func TestReportReturnsNYearHighLineWithEvidence(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/report/BREAK", "{}")
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		Symbol    string  `json:"symbol"`
		AsOf      string  `json:"as_of"`
		Level     float64 `json:"level"`
		LevelDate string  `json:"level_date"`
		Lines     []struct {
			Type   string `json:"type"`
			Result struct {
				Pass      bool `json:"pass"`
				Evaluable bool `json:"evaluable"`
				Evidence  struct {
					Type      string  `json:"type"`
					Level     float64 `json:"level"`
					LevelDate string  `json:"level_date"`
					BarsSince int     `json:"bars_since"`
					Close     float64 `json:"close"`
				} `json:"evidence"`
			} `json:"result"`
		} `json:"lines"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Symbol != "BREAK" || got.AsOf != string(date(799)) || got.Level != 130 || got.LevelDate != string(date(300)) {
		t.Fatalf("header = %+v", got)
	}
	if len(got.Lines) != 1 {
		t.Fatalf("lines = %+v", got.Lines)
	}
	l := got.Lines[0]
	ev := l.Result.Evidence
	if l.Type != "n_year_high" || !l.Result.Pass || !l.Result.Evaluable ||
		ev.Type != "n_year_high" || ev.Level != 130 || ev.LevelDate != string(date(300)) || ev.BarsSince != 499 || ev.Close != 131 {
		t.Fatalf("line = %+v", l)
	}
}

func TestReportWithoutBodyUsesLatestBar(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/report/BREAK", "")
	if status != 200 || !strings.Contains(string(body), string(date(799))) {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestReportAsOf(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/report/BREAK", `{"as_of":"`+string(date(500))+`"}`)
	if status != 200 || !strings.Contains(string(body), `"as_of":"`+string(date(500))+`"`) {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestReportBadAsOf400WithField(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/report/BREAK", `{"as_of":"2024-13-45"}`)
	if status != 400 {
		t.Fatalf("status %d: %s", status, body)
	}
	var problem struct {
		Errors []struct {
			Location string `json:"location"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &problem); err != nil {
		t.Fatal(err)
	}
	if len(problem.Errors) == 0 || problem.Errors[0].Location != "body.as_of" {
		t.Fatalf("problem = %s", body)
	}
}

func TestUnknownSymbol404(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/report/NOPE", "{}")
	if status != 404 {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestPatternsEndpointListsRegistry(t *testing.T) {
	status, body := do(t, server(t), "GET", "/api/patterns", "")
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		Patterns []struct {
			Type   string `json:"type"`
			Params []struct {
				Name    string `json:"name"`
				Unit    string `json:"unit"`
				Default any    `json:"default"`
			} `json:"params"`
		} `json:"patterns"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Patterns) != 1 || got.Patterns[0].Type != "n_year_high" ||
		got.Patterns[0].Params[0].Name != "years" || got.Patterns[0].Params[0].Unit != "years" || got.Patterns[0].Params[0].Default != 3.0 {
		t.Fatalf("patterns = %s", body)
	}
}

func TestBarsEndpoint(t *testing.T) {
	status, body := do(t, server(t), "GET", "/api/bars/BREAK?from="+string(date(798)), "")
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		Bars []market.Bar `json:"bars"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Bars) != 2 || got.Bars[1].Close != 131 {
		t.Fatalf("bars = %+v", got.Bars)
	}
	if status, _ := do(t, server(t), "GET", "/api/bars/NOPE", ""); status != 404 {
		t.Fatalf("unknown symbol status %d", status)
	}
}

func TestDocsServed(t *testing.T) {
	srv := server(t)
	for _, p := range []string{"/docs", "/openapi.json"} {
		if status, body := do(t, srv, "GET", p, ""); status != 200 {
			t.Fatalf("GET %s: %d %s", p, status, body)
		}
	}
}

func TestOpenAPIEvidenceIsDiscriminatedUnion(t *testing.T) {
	spec, err := OpenAPIJSON()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]struct {
				OneOf []struct {
					Ref string `json:"$ref"`
				} `json:"oneOf"`
				Discriminator *struct {
					PropertyName string            `json:"propertyName"`
					Mapping      map[string]string `json:"mapping"`
				} `json:"discriminator"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		t.Fatal(err)
	}
	ev, ok := doc.Components.Schemas["Evidence"]
	if !ok || ev.Discriminator == nil || ev.Discriminator.PropertyName != "type" {
		t.Fatalf("Evidence schema = %+v", ev)
	}
	want := map[string]string{
		"n_year_high":   "#/components/schemas/NYearHighEvidence",
		"not_evaluable": "#/components/schemas/NotEvaluable",
	}
	if len(ev.OneOf) != len(want) || len(ev.Discriminator.Mapping) != len(want) {
		t.Fatalf("oneOf = %+v mapping = %+v", ev.OneOf, ev.Discriminator.Mapping)
	}
	for typ, ref := range want {
		if ev.Discriminator.Mapping[typ] != ref {
			t.Fatalf("mapping[%s] = %q, want %q", typ, ev.Discriminator.Mapping[typ], ref)
		}
	}
	// The served spec is the same document.
	status, served := do(t, server(t), "GET", "/openapi.json", "")
	if status != 200 || !strings.Contains(string(served), `"discriminator"`) {
		t.Fatalf("served spec lacks the discriminator")
	}
}

func TestOpenAPIJSONIsDeterministic(t *testing.T) {
	a, err := OpenAPIJSON()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := OpenAPIJSON()
	if string(a) != string(b) {
		t.Fatal("spec differs between runs")
	}
}

func TestSPAFallbackServesIndex(t *testing.T) {
	srv := server(t)
	for _, p := range []string{"/", "/report/BREAK"} {
		status, body := do(t, srv, "GET", p, "")
		if status != 200 || !strings.Contains(string(body), "<title>Kanso</title>") {
			t.Fatalf("GET %s: %d %s", p, status, body)
		}
	}
	if status, body := do(t, srv, "GET", "/assets/app.js", ""); status != 200 || string(body) != "console.log(1)" {
		t.Fatalf("asset: %d %s", status, body)
	}
	if status, _ := do(t, srv, "GET", "/api/nope", ""); status != 404 {
		t.Fatalf("unknown API route should 404, got %d", status)
	}
}
