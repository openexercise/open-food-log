package conformance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestA2ACoreSkills(t *testing.T) {
	core, err := A2ACoreSkills()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"log-meal", "food-lookup", "get-meals", "nutrition-summary", "targets"} {
		if !slices.Contains(core, id) {
			t.Errorf("core skills %v lack %s", core, id)
		}
	}
}

// TestRunA2AWithoutTheProfile: an agent whose card doesn't name the profile fails card, and every
// check after it is skipped rather than run against nothing.
func TestRunA2AWithoutTheProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"name": "x", "supportedInterfaces": [{"url": "http://x/a2a", "protocolBinding": "JSONRPC", "protocolVersion": "1.0"}], "skills": []}`))
	}))
	defer srv.Close()
	report, err := RunA2A(context.Background(), A2AConfig{URL: srv.URL, Token: "t", ReadOnlyToken: "r"})
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed() || len(report.Results) != len(A2AChecks) || report.Results[0].Status != Fail {
		t.Fatalf("report: %+v", report)
	}
	for _, r := range report.Results[2:] {
		if r.Status == Pass {
			t.Errorf("%s passed against an agent that serves nothing", r.Name)
		}
	}
}

func TestRunA2ANeedsAToken(t *testing.T) {
	if _, err := RunA2A(context.Background(), A2AConfig{URL: "https://example.test"}); err == nil {
		t.Error("no error without a token")
	}
}
