package api

import (
	"encoding/json"
	"strings"
	"testing"
)

// problemLocations decodes a problem JSON body's error locations.
func problemLocations(t *testing.T, body []byte) []string {
	t.Helper()
	var problem struct {
		Errors []struct {
			Location string `json:"location"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(body, &problem); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	out := []string{}
	for _, e := range problem.Errors {
		out = append(out, e.Location)
	}
	return out
}

func TestUniversesEndpoint(t *testing.T) {
	status, body := do(t, server(t), "GET", "/api/universes", "")
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		Universes []struct {
			Name       string `json:"name"`
			Members    int    `json:"members"`
			HasHistory bool   `json:"has_history"`
		} `json:"universes"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Universes) != 1 || got.Universes[0].Name != "NIFTY50" || got.Universes[0].Members != 1 || got.Universes[0].HasHistory {
		t.Fatalf("got %s", body)
	}
}

func TestQuickscanReturnsAutoInserted(t *testing.T) {
	// BREAK closes at 131 over its 3-year high of 130 on the last bar: a
	// fresh signal, 0.77% above the level, so not extended.
	status, body := do(t, server(t), "POST", "/api/quickscan",
		`{"pattern":"not_extended","params":{"pct":8},"universe":"NIFTY50","timeframe":"daily","mode":"fresh"}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		AsOf         string   `json:"as_of"`
		AutoInserted []string `json:"auto_inserted"`
		Stale        []string `json:"stale"`
		Skipped      []string `json:"skipped"`
		Matches      []struct {
			Symbol     string   `json:"symbol"`
			SignalDate string   `json:"signal_date"`
			Level      *float64 `json:"level"`
			PctAbove   *float64 `json:"pct_above"`
			Outcome    struct {
				Pass       bool `json:"pass"`
				Conditions []struct {
					Group     string `json:"group"`
					Condition struct {
						Type string `json:"type"`
					} `json:"condition"`
					Result struct {
						Evidence struct {
							Type string `json:"type"`
						} `json:"evidence"`
					} `json:"result"`
				} `json:"conditions"`
			} `json:"outcome"`
		} `json:"matches"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.AutoInserted) != 1 || got.AutoInserted[0] != "n_year_high" {
		t.Fatalf("auto_inserted = %v", got.AutoInserted)
	}
	if got.AsOf != string(date(799)) || got.Stale == nil || got.Skipped == nil {
		t.Fatalf("got %s", body)
	}
	if len(got.Matches) != 1 {
		t.Fatalf("matches: %s", body)
	}
	m := got.Matches[0]
	if m.Symbol != "BREAK" || m.SignalDate != string(date(799)) || m.Level == nil || *m.Level != 130 || m.PctAbove == nil || !m.Outcome.Pass {
		t.Fatalf("match: %s", body)
	}
	if len(m.Outcome.Conditions) != 2 || m.Outcome.Conditions[0].Condition.Type != "n_year_high" ||
		m.Outcome.Conditions[1].Result.Evidence.Type != "not_extended" {
		t.Fatalf("conditions: %s", body)
	}
}

func TestQuickscanAllModeTrueSince(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/quickscan", `{"pattern":"n_year_high","universe":"NIFTY50","mode":"all"}`)
	if status != 200 || !strings.Contains(string(body), `"true_since":"`+string(date(799))+`"`) {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestQuickscanUnknownPattern400(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/quickscan", `{"pattern":"cup_and_handle","universe":"NIFTY50"}`)
	if status != 400 {
		t.Fatalf("status %d: %s", status, body)
	}
	if locs := problemLocations(t, body); len(locs) != 1 || locs[0] != "body.pattern" {
		t.Fatalf("locations %v", locs)
	}
}

func TestQuickscanBadParams400(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/quickscan", `{"pattern":"n_year_high","params":{"years":99},"universe":"NIFTY50"}`)
	if locs := problemLocations(t, body); status != 400 || len(locs) != 1 || locs[0] != "body.params" {
		t.Fatalf("status %d: %s", status, body)
	}
}

func TestQuickscanUnknownUniverse404(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/quickscan", `{"pattern":"n_year_high","universe":"NOPE"}`)
	if status != 404 || !strings.Contains(string(body), "NOPE") {
		t.Fatalf("status %d: %s", status, body)
	}
}

// TestBacktestValidationError400WithFields checks the system validation
// contract every system endpoint shares, through POST /api/backtest.
func TestBacktestValidationError400WithFields(t *testing.T) {
	sys := `{"name":"x","universe":"NIFTY50","entry":{
		"all":[{"type":"n_year_high","params":{"years":3}}],
		"any":[{"type":"volume_surge","params":{}}]}}`
	status, body := do(t, server(t), "POST", "/api/backtest", sys)
	if status != 400 {
		t.Fatalf("status %d: %s", status, body)
	}
	if locs := problemLocations(t, body); len(locs) == 0 || locs[0] != "body.entry.any" {
		t.Fatalf("locations %v", locs)
	}
}

func TestValidateReturnsNormalizedSystem(t *testing.T) {
	status, body := do(t, server(t), "POST", "/api/validate",
		`{"name":"x","universe":"NIFTY50","entry":{"all":[{"type":"n_year_high"}]}}`)
	if status != 200 {
		t.Fatalf("status %d: %s", status, body)
	}
	var got struct {
		System struct {
			Timeframe string `json:"timeframe"`
			Entry     struct {
				All []struct {
					Params map[string]any `json:"params"`
				} `json:"all"`
			} `json:"entry"`
			Exits struct {
				StopPct float64 `json:"stop_pct"`
			} `json:"exits"`
		} `json:"system"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.System.Timeframe != "daily" || got.System.Exits.StopPct != 8 || got.System.Entry.All[0].Params["years"] != 3.0 {
		t.Fatalf("got %s", body)
	}
}
