package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	openfoodlog "github.com/openexercise/open-food-log"
)

// foodLog is an in-memory MCP server serving the profile's core tools, as the profile has them,
// for a writer and a read-only reader. A broken field turns one behaviour off.
type foodLog struct {
	mu      sync.Mutex
	next    int
	meals   map[string]map[string]any
	deleted map[string]bool
	keys    map[string]string
	undos   map[string]string

	skip       string // a core tool left out
	noRestore  bool
	confirmAll bool // nothing goes to needs_confirmation

	saved         map[string]map[string]any
	againUntitled bool // log_meal_again leaves out the saved meal's name
}

func newFoodLog() *foodLog {
	return &foodLog{meals: map[string]map[string]any{}, deleted: map[string]bool{}, keys: map[string]string{}, undos: map[string]string{},
		saved: map[string]map[string]any{}}
}

func (f *foodLog) handler(t *testing.T) http.Handler {
	var profile struct {
		Tools []*mcp.Tool `json:"tools"`
	}
	if err := json.Unmarshal(openfoodlog.Profile, &profile); err != nil {
		t.Fatal(err)
	}
	server := func(readOnly bool) *mcp.Server {
		s := mcp.NewServer(&mcp.Implementation{Name: "food-log", Version: "0"}, nil)
		for _, tool := range profile.Tools {
			if tool.Meta[profileKey] != "core" || tool.Name == f.skip || (readOnly && !tool.Annotations.ReadOnlyHint) {
				continue
			}
			s.AddTool(tool, f.tool(tool.Name))
		}
		return s
	}
	writer, reader := server(false), server(true)
	h := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		if r.Header.Get("Authorization") == "Bearer reader" {
			return reader
		}
		return writer
	}, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a := r.Header.Get("Authorization"); a != "Bearer writer" && a != "Bearer reader" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="http://example.test/.well-known/oauth-protected-resource"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func (f *foodLog) tool(name string) mcp.ToolHandler {
	return func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		var args map[string]any
		_ = json.Unmarshal(req.Params.Arguments, &args)
		switch name {
		case "log_meal":
			return f.logMeal(args)
		case "get_meals":
			list := []any{}
			for id, m := range f.meals {
				if !f.deleted[id] {
					list = append(list, m)
				}
			}
			return ok("Your meals.", map[string]any{"meals": list})
		case "update_meal":
			m, found := f.meals[fmt.Sprint(args["meal_id"])]
			if !found || f.deleted[fmt.Sprint(args["meal_id"])] {
				return failed("not_found: no meal with that ID")
			}
			if title, ok := args["title"].(string); ok {
				m["title"] = title
			}
			return ok("Updated.", map[string]any{"meal": m, "needs_confirmation": []any{}})
		case "delete_meal":
			id := fmt.Sprint(args["meal_id"])
			if _, found := f.meals[id]; !found || f.deleted[id] {
				return failed("not_found: no meal with that ID")
			}
			f.deleted[id] = true
			token := "undo-" + id
			f.undos[token] = id
			return ok("Deleted.", map[string]any{"undo_token": token, "deleted_at": "2026-11-12T00:00:00Z"})
		case "undo":
			id, found := f.undos[fmt.Sprint(args["undo_token"])]
			if !found {
				return failed("undo_unavailable: unknown token")
			}
			delete(f.undos, fmt.Sprint(args["undo_token"]))
			if !f.noRestore {
				delete(f.deleted, id)
			}
			return ok("Restored.", map[string]any{"meal": f.meals[id], "needs_confirmation": []any{}})
		case "search_foods":
			return ok("No foods.", map[string]any{"foods": []any{}})
		case "save_meal":
			from, found := f.meals[fmt.Sprint(args["meal_id"])]
			if !found {
				return failed("not_found: no meal with that ID")
			}
			f.next++
			id := fmt.Sprintf("saved:%d", f.next)
			f.saved[id] = map[string]any{"id": id, "name": args["name"], "items": from["items"]}
			return ok("Saved.", f.saved[id])
		case "get_saved_meals":
			list := []any{}
			for _, sm := range f.saved {
				list = append(list, sm)
			}
			return ok("Your saved meals.", map[string]any{"saved_meals": list})
		case "delete_saved_meal":
			id := fmt.Sprint(args["saved_meal_id"])
			if _, found := f.saved[id]; !found {
				return failed("not_found: no saved meal with that ID")
			}
			delete(f.saved, id)
			return ok("Deleted.", map[string]any{"id": id})
		case "log_meal_again":
			sm, found := f.saved[fmt.Sprint(args["saved_meal_id"])]
			if !found {
				return failed("not_found: no saved meal with that ID")
			}
			items := []any{}
			for _, it := range sm["items"].([]any) {
				it := it.(map[string]any)
				items = append(items, map[string]any{"text": it["text"], "nutrients": map[string]any{"energy_kcal": 1}})
			}
			again := map[string]any{"items": items, "idempotency_key": args["idempotency_key"]}
			if !f.againUntitled {
				again["title"] = sm["name"]
			}
			return f.logMeal(again)
		}
		return failed("invalid_input: unknown tool")
	}
}

func (f *foodLog) logMeal(args map[string]any) (*mcp.CallToolResult, error) {
	key, _ := args["idempotency_key"].(string)
	if id, seen := f.keys[key]; seen && key != "" {
		return ok("Already logged.", map[string]any{"meal": f.meals[id], "needs_confirmation": []any{}})
	}
	f.next++
	id := fmt.Sprintf("meal_%d", f.next)
	items, confirm := []any{}, []any{}
	for i, it := range args["items"].([]any) {
		it := it.(map[string]any)
		item := map[string]any{"id": fmt.Sprintf("%s_%d", id, i), "text": it["text"]}
		if n, ok := it["nutrients"]; ok {
			item["food"] = map[string]any{"id": nil, "name": it["text"], "match": "none"}
			item["nutrients"] = n
		} else {
			item["food"] = map[string]any{"id": nil, "name": it["text"], "match": "unmatched"}
			if !f.confirmAll {
				confirm = append(confirm, map[string]any{"item_id": item["id"], "text": it["text"], "candidates": []any{}})
			}
		}
		items = append(items, item)
	}
	m := map[string]any{
		"$schema": "https://openfoodlog.org/schema/v0/meal.json", "id": id, "date": "2026-11-12",
		"source": map[string]any{"kind": "mcp", "external_id": nil}, "items": items, "estimates": []any{},
	}
	if title, ok := args["title"]; ok {
		m["title"] = title
	}
	f.meals[id] = m
	if key != "" {
		f.keys[key] = id
	}
	return ok("Logged.", map[string]any{"meal": m, "needs_confirmation": confirm})
}

func ok(summary string, v any) (*mcp.CallToolResult, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}, &mcp.TextContent{Text: string(raw)}}}, nil
}

func failed(text string) (*mcp.CallToolResult, error) {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil
}

func runMCP(t *testing.T, f *foodLog, readToken string) MCPReport {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	report, err := RunMCP(context.Background(), MCPConfig{URL: srv.URL, Token: "writer", ReadOnlyToken: readToken, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestFoodLogPassesMCP(t *testing.T) {
	f := newFoodLog()
	report := runMCP(t, f, "reader")
	for _, r := range report.Results {
		if r.Status != Pass {
			t.Errorf("%s %s: %s", r.Status, r.Name, r.Detail)
		}
	}
	if !report.Passed() {
		t.Error("not conformant")
	}
	for id := range f.meals {
		if !f.deleted[id] {
			t.Errorf("%s left stored", id)
		}
	}
}

func TestMCPChecksCatchTheirBehaviour(t *testing.T) {
	cases := []struct {
		name   string
		breaks func(*foodLog)
		fails  []string
	}{
		// A missing core read fails read-only too, which looks for the core reads.
		{"a core read missing", func(f *foodLog) { f.skip = "search_foods" }, []string{"tools", "read-only"}},
		{"a core write missing", func(f *foodLog) { f.skip = "update_meal" }, []string{"tools", "update"}},
		{"an unknown food logged as sure", func(f *foodLog) { f.confirmAll = true }, []string{"needs-confirmation"}},
		{"undo restores nothing", func(f *foodLog) { f.noRestore = true }, []string{"undo"}},
		{"a saved meal logged without its name", func(f *foodLog) { f.againUntitled = true }, []string{"again"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newFoodLog()
			c.breaks(f)
			got := map[string]Status{}
			for _, r := range runMCP(t, f, "reader").Results {
				got[r.Name] = r.Status
			}
			for _, name := range MCPChecks {
				want := Pass
				for _, failing := range c.fails {
					if name == failing {
						want = Fail
					}
				}
				if got[name] != want {
					t.Errorf("%s = %s, want %s", name, got[name], want)
				}
			}
		})
	}
}

// Without a read-only token, read-only is skipped and the server can't pass.
func TestMCPNeedsAReadOnlyToken(t *testing.T) {
	report := runMCP(t, newFoodLog(), "")
	if report.Passed() {
		t.Error("passed without checking a read-only credential")
	}
	for _, r := range report.Results {
		if r.Name == "read-only" && r.Status != Skip {
			t.Errorf("read-only = %s, want skip", r.Status)
		}
	}
}
