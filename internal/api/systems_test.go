package api

import (
	"encoding/json"
	"testing"
)

type systemJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Universe  string `json:"universe"`
	Timeframe string `json:"timeframe"`
	Entry     struct {
		All []struct {
			Type   string         `json:"type"`
			Params map[string]any `json:"params"`
		} `json:"all"`
	} `json:"entry"`
	Exits struct {
		StopPct float64 `json:"stop_pct"`
	} `json:"exits"`
}

const breakoutSys = `{"name":"Breakout","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}]}}`

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return v
}

func TestSystemsCRUDEndpoints(t *testing.T) {
	srv := server(t)

	status, body := do(t, srv, "POST", "/api/systems", breakoutSys)
	if status != 201 {
		t.Fatalf("create: status %d: %s", status, body)
	}
	created := decode[systemJSON](t, body)
	if len(created.ID) != 36 || created.Timeframe != "daily" || created.Exits.StopPct != 8 ||
		created.Entry.All[0].Params["years"] != 3.0 {
		t.Fatalf("created = %s", body)
	}

	status, body = do(t, srv, "GET", "/api/systems/"+created.ID, "")
	if got := decode[systemJSON](t, body); status != 200 || got.Name != "Breakout" || got.ID != created.ID {
		t.Fatalf("get: status %d: %s", status, body)
	}

	status, body = do(t, srv, "PUT", "/api/systems/"+created.ID,
		`{"id":"ignored","name":"Weekly breakout","universe":"NIFTY50","timeframe":"weekly","entry":{"all":[{"type":"n_year_high"}]}}`)
	if got := decode[systemJSON](t, body); status != 200 || got.ID != created.ID || got.Timeframe != "weekly" {
		t.Fatalf("put: status %d: %s", status, body)
	}

	if status, body = do(t, srv, "POST", "/api/systems", `{"name":"Another","universe":"NIFTY50","entry":{"all":[{"type":"volume_surge"}]}}`); status != 201 {
		t.Fatalf("create 2: status %d: %s", status, body)
	}
	status, body = do(t, srv, "GET", "/api/systems", "")
	list := decode[struct {
		Systems []systemJSON `json:"systems"`
	}](t, body)
	if status != 200 || len(list.Systems) != 2 || list.Systems[0].Name != "Another" || list.Systems[1].Name != "Weekly breakout" {
		t.Fatalf("list: status %d: %s", status, body)
	}

	if status, body = do(t, srv, "DELETE", "/api/systems/"+created.ID, ""); status != 204 {
		t.Fatalf("delete: status %d: %s", status, body)
	}
	for _, method := range []string{"GET", "DELETE"} {
		if status, body = do(t, srv, method, "/api/systems/"+created.ID, ""); status != 404 {
			t.Fatalf("%s deleted: status %d: %s", method, status, body)
		}
	}
	if status, body = do(t, srv, "PUT", "/api/systems/"+created.ID, breakoutSys); status != 404 {
		t.Fatalf("put deleted: status %d: %s", status, body)
	}
}

func TestSystemsListEmpty(t *testing.T) {
	status, body := do(t, server(t), "GET", "/api/systems", "")
	if status != 200 || string(body) != `{"systems":[]}`+"\n" {
		t.Fatalf("status %d: %q", status, body)
	}
}

func TestCreateSystemValidation400(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/systems",
		`{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}],"any":[{"type":"volume_surge"}]}}`)
	if status != 400 {
		t.Fatalf("status %d: %s", status, body)
	}
	if locs := problemLocations(t, body); len(locs) == 0 || locs[0] != "body.entry.any" {
		t.Fatalf("locations %v", locs)
	}
}

type scanJSON struct {
	AsOf    string `json:"as_of"`
	Matches []struct {
		Symbol     string `json:"symbol"`
		SignalDate string `json:"signal_date"`
		TrueSince  string `json:"true_since"`
	} `json:"matches"`
}

func TestScanSystem(t *testing.T) {
	srv := server(t)
	for _, mode := range []string{"fresh", "all"} {
		status, body := do(t, srv, "POST", "/api/scan", `{"system":`+breakoutSys+`,"mode":"`+mode+`"}`)
		got := decode[scanJSON](t, body)
		if status != 200 || got.AsOf != string(date(799)) || len(got.Matches) != 1 || got.Matches[0].Symbol != "BREAK" {
			t.Fatalf("%s: status %d: %s", mode, status, body)
		}
		if m := got.Matches[0]; (mode == "fresh") != (m.SignalDate != "") || (mode == "all") != (m.TrueSince != "") {
			t.Fatalf("%s: match %+v", mode, m)
		}
	}
}

func TestScanAsOf(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/scan", `{"system":`+breakoutSys+`,"as_of":"`+string(date(500))+`"}`)
	if got := decode[scanJSON](t, body); status != 200 || got.AsOf != string(date(500)) || len(got.Matches) != 0 {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestScanInvalidSystem400(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/scan",
		`{"system":{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"nope"}]}}}`)
	if status != 400 {
		t.Fatalf("status %d: %s", status, body)
	}
	if locs := problemLocations(t, body); len(locs) == 0 || locs[0] != "body.system.entry.all[0].type" {
		t.Fatalf("locations %v", locs)
	}
}

func TestScanUnknownUniverse404(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/scan",
		`{"system":{"name":"x","universe":"NOPE","entry":{"all":[{"type":"n_year_high"}]}}}`)
	if status != 404 {
		t.Fatalf("status %d: %s", status, body)
	}
}

// TestReportPostWithSystemBody reports a system that was never saved.
func TestReportPostWithSystemBody(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/report/BREAK",
		`{"system":{"name":"draft","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"},{"type":"not_extended"}]}}}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	got := decode[struct {
		Lines []struct {
			Type string `json:"type"`
		} `json:"lines"`
		SystemOutcome *struct {
			Pass       bool `json:"pass"`
			Evaluable  bool `json:"evaluable"`
			Conditions []struct {
				Condition struct {
					Type string `json:"type"`
				} `json:"condition"`
			} `json:"conditions"`
		} `json:"system_outcome"`
	}](t, body)
	if len(got.Lines) != 2 || got.Lines[0].Type != "n_year_high" || got.Lines[1].Type != "not_extended" {
		t.Fatalf("lines: %s", body)
	}
	if o := got.SystemOutcome; o == nil || !o.Pass || !o.Evaluable || len(o.Conditions) != 2 {
		t.Fatalf("outcome: %s", body)
	}
}

func TestReportWithoutSystemHasNoOutcome(t *testing.T) {
	_, body := do(t, server(t), "POST", "/api/report/BREAK", "{}")
	if got := decode[map[string]any](t, body); got["system_outcome"] != nil {
		t.Fatalf("got %s", body)
	}
}

func TestReportInvalidSystem400(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/report/BREAK",
		`{"system":{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}],"any":[{"type":"volume_surge"}]}}}`)
	if status != 400 {
		t.Fatalf("status %d: %s", status, body)
	}
	if locs := problemLocations(t, body); len(locs) == 0 || locs[0] != "body.system.entry.any" {
		t.Fatalf("locations %v", locs)
	}
}
