package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
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

// nutritionAgent is an in-memory A2A agent keeping to the profile's core as far as RunA2A goes: a
// card, a JSON-RPC endpoint at /a2a answering SendMessage, GetTask and CancelTask, for a writer and a
// read-only reader. A broken field turns one behaviour off, so a test can check the right check
// catches it.
type nutritionAgent struct {
	mu        sync.Mutex
	next      int
	meals     map[string]map[string]any
	tasks     map[string]map[string]any
	byMessage map[string]string // messageId → task ID
	waiting   map[string][]any  // task ID → items still to confirm

	dropSkill                               string // a core skill left off the card
	noChallenge, noReplay, invalidMeal      bool
	confirmAll, answerCompletes, keepsTasks bool // keepsTasks: CancelTask changes nothing
	noGetTask, findsAnyTask, listsNothing   bool
	noFoods, noDays, badTargets             bool
	acceptsInvalid, readerWrites            bool
}

func newNutritionAgent() *nutritionAgent {
	return &nutritionAgent{meals: map[string]map[string]any{}, tasks: map[string]map[string]any{}, byMessage: map[string]string{},
		waiting: map[string][]any{}}
}

func (g *nutritionAgent) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if r.Method == http.MethodGet && r.URL.Path == "/.well-known/agent-card.json" {
		writeJSON(w, http.StatusOK, g.card("http://"+r.Host+"/a2a"))
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/a2a" {
		http.NotFound(w, r)
		return
	}
	auth := r.Header.Get("Authorization")
	if auth != "Bearer writer" && auth != "Bearer reader" {
		if !g.noChallenge {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="http://example.test/.well-known/oauth-protected-resource/a2a"`)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req struct {
		ID     any            `json:"id"`
		Method string         `json:"method"`
		Params map[string]any `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, rpcError(nil, -32700, "parse error"))
		return
	}
	result, code, message := g.call(req.Method, req.Params, auth == "Bearer reader")
	if code != 0 {
		writeJSON(w, http.StatusOK, rpcError(req.ID, code, message))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func rpcError(id any, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

func (g *nutritionAgent) card(endpoint string) map[string]any {
	core, _ := A2ACoreSkills()
	listed, skills := []any{}, []any{}
	for _, id := range core {
		listed = append(listed, id)
		if id != g.dropSkill {
			skills = append(skills, map[string]any{"id": id, "name": id, "tags": []any{"openfoodlog:core"}})
		}
	}
	return map[string]any{
		"name":                "nutrition agent",
		"supportedInterfaces": []any{map[string]any{"url": endpoint, "protocolBinding": "JSONRPC", "protocolVersion": "1.0"}},
		"capabilities": map[string]any{"extensions": []any{map[string]any{
			"uri": A2AProfileURI, "params": map[string]any{"core_skills": listed},
		}}},
		"skills": skills,
		"securitySchemes": map[string]any{"oauth": map[string]any{"type": "oauth2", "flows": map[string]any{"authorizationCode": map[string]any{
			"scopes": map[string]any{"meals:read": "read", "meals:write": "write"},
		}}}},
	}
}

// call answers one JSON-RPC method: a result, or a JSON-RPC error code and message.
func (g *nutritionAgent) call(method string, params map[string]any, isReader bool) (map[string]any, int, string) {
	switch method {
	case "SendMessage":
		msg, _ := params["message"].(map[string]any)
		return g.send(msg, isReader), 0, ""
	case "GetTask":
		if g.noGetTask {
			break
		}
		task, ok := g.tasks[fmt.Sprint(params["id"])]
		if !ok && g.findsAnyTask {
			return map[string]any{"id": params["id"], "status": map[string]any{"state": "TASK_STATE_COMPLETED"}}, 0, ""
		}
		if !ok {
			return nil, -32001, "task not found"
		}
		return task, 0, ""
	case "CancelTask":
		task, ok := g.tasks[fmt.Sprint(params["id"])]
		if !ok {
			return nil, -32001, "task not found"
		}
		if taskState(task) != "TASK_STATE_INPUT_REQUIRED" {
			return nil, -32002, "task cannot be canceled"
		}
		if !g.keepsTasks {
			task["status"] = map[string]any{"state": "TASK_STATE_CANCELED"}
		}
		return task, 0, ""
	}
	return nil, -32601, "method not found"
}

func (g *nutritionAgent) send(msg map[string]any, isReader bool) map[string]any {
	var data map[string]any
	if parts := anySlice(msg["parts"]); len(parts) > 0 {
		data, _ = parts[0].(map[string]any)["data"].(map[string]any)
	}
	if taskID, ok := msg["taskId"].(string); ok {
		return g.answer(taskID, data)
	}
	skill, _ := anyMap(msg["metadata"])["skill"].(string)
	switch skill {
	case "log-meal":
		if isReader && !g.readerWrites {
			return map[string]any{"task": g.refuse("insufficient_scope", "logging needs meals:write")}
		}
		return g.logMeal(fmt.Sprint(msg["messageId"]), data)
	case "get-meals":
		parts := []any{}
		for _, m := range g.meals {
			date := fmt.Sprint(m["date"])
			if !g.listsNothing && date >= fmt.Sprint(data["from"]) && date <= fmt.Sprint(data["to"]) {
				parts = append(parts, map[string]any{"data": m})
			}
		}
		return reply(parts...)
	case "food-lookup":
		if g.noFoods {
			return reply(map[string]any{"data": map[string]any{"results": []any{}}})
		}
		return reply(map[string]any{"data": map[string]any{"foods": []any{}}})
	case "nutrition-summary":
		days := []any{}
		if !g.noDays {
			days = append(days, map[string]any{"date": data["from"], "totals": map[string]any{}})
		}
		return reply(map[string]any{"data": map[string]any{"days": days}})
	case "targets":
		doc := map[string]any{"$schema": "https://openfoodlog.org/schema/v0/targets.json", "targets": []any{}}
		if g.badTargets {
			doc["targets"] = "none"
		}
		return reply(map[string]any{"data": doc})
	}
	return reply(map[string]any{"text": "invalid_input: no skill"}, map[string]any{"data": map[string]any{"error": map[string]any{"code": "invalid_input"}}})
}

func reply(parts ...any) map[string]any {
	return map[string]any{"message": map[string]any{"role": "ROLE_AGENT", "messageId": "reply", "parts": parts}}
}

func anyMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// refuse makes a rejected task carrying the profile's error part.
func (g *nutritionAgent) refuse(code, message string) map[string]any {
	g.next++
	return map[string]any{"id": fmt.Sprintf("task_%d", g.next), "contextId": "ctx", "status": map[string]any{
		"state": "TASK_STATE_REJECTED",
		"message": map[string]any{"role": "ROLE_AGENT", "parts": []any{
			map[string]any{"text": code + ": " + message},
			map[string]any{"data": map[string]any{"error": map[string]any{"code": code, "message": message}}},
		}},
	}}
}

func (g *nutritionAgent) logMeal(messageID string, in map[string]any) map[string]any {
	if id, seen := g.byMessage[messageID]; seen && !g.noReplay {
		return map[string]any{"task": g.tasks[id]}
	}
	g.next++
	mealID := fmt.Sprintf("meal_%d", g.next)
	items, confirm := []any{}, []any{}
	for i, raw := range anySlice(in["items"]) {
		it := anyMap(raw)
		text, ok := it["text"].(string)
		if !ok && !g.acceptsInvalid {
			return map[string]any{"task": g.refuse("invalid_input", "an item needs its text")}
		}
		item := map[string]any{"id": fmt.Sprintf("%s_%d", mealID, i), "text": text}
		if n, ok := it["nutrients"]; ok {
			item["food"], item["nutrients"] = it["food"], n
		} else {
			item["food"] = map[string]any{"id": nil, "name": text, "match": "unmatched"}
			if !g.confirmAll {
				confirm = append(confirm, map[string]any{"item_id": item["id"], "text": text, "candidates": []any{}})
			}
		}
		items = append(items, item)
	}
	meal := map[string]any{"$schema": "https://openfoodlog.org/schema/v0/meal.json", "id": mealID, "date": in["date"],
		"source": in["source"], "items": items, "estimates": []any{}}
	if g.invalidMeal {
		delete(meal, "source")
	}
	g.meals[mealID] = meal
	taskID := fmt.Sprintf("task_%d", g.next)
	status := map[string]any{"state": "TASK_STATE_COMPLETED"}
	if len(confirm) > 0 {
		g.waiting[taskID] = confirm
		status = map[string]any{"state": "TASK_STATE_INPUT_REQUIRED", "message": map[string]any{"role": "ROLE_AGENT", "parts": []any{
			map[string]any{"data": map[string]any{"meal_id": mealID, "needs_confirmation": confirm}},
		}}}
	}
	task := map[string]any{"id": taskID, "contextId": "ctx_" + taskID, "status": status, "artifacts": []any{map[string]any{
		"artifactId": "meal", "parts": []any{map[string]any{"text": "Logged."}, map[string]any{"data": meal}},
	}}}
	g.tasks[taskID] = task
	g.byMessage[messageID] = taskID
	return map[string]any{"task": task}
}

// answer confirms the waiting items an answer names, and completes the task once none are left.
func (g *nutritionAgent) answer(taskID string, data map[string]any) map[string]any {
	task, ok := g.tasks[taskID]
	if !ok {
		return map[string]any{"task": g.refuse("not_found", "no such task")}
	}
	left := []any{}
	for _, w := range g.waiting[taskID] {
		named := false
		for _, c := range anySlice(data["confirmations"]) {
			named = named || anyMap(c)["item_id"] == anyMap(w)["item_id"]
		}
		if !named {
			left = append(left, w)
		}
	}
	g.waiting[taskID] = left
	if len(left) == 0 || g.answerCompletes {
		task["status"] = map[string]any{"state": "TASK_STATE_COMPLETED"}
	}
	return map[string]any{"task": task}
}

func runA2A(t *testing.T, g *nutritionAgent, readToken string) A2AReport {
	t.Helper()
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	report, err := RunA2A(context.Background(), A2AConfig{URL: srv.URL, Token: "writer", ReadOnlyToken: readToken, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestA2AReferencePasses(t *testing.T) {
	g := newNutritionAgent()
	report := runA2A(t, g, "reader")
	for _, r := range report.Results {
		if r.Status != Pass {
			t.Errorf("%s %s: %s", r.Status, r.Name, r.Detail)
		}
	}
	if !report.Passed() {
		t.Error("not conformant")
	}
	if !strings.HasSuffix(report.Endpoint, "/a2a") {
		t.Errorf("endpoint %q, want the card's", report.Endpoint)
	}
	// The run logs one meal (replay, the refusals and the read-only write store nothing).
	if len(g.meals) != 1 {
		t.Errorf("%d meals stored, want 1", len(g.meals))
	}
}

// Each broken behaviour fails the check that covers it, and only checks that depend on it skip.
func TestA2AChecksCatchTheirBehaviour(t *testing.T) {
	cases := []struct {
		name   string
		breaks func(*nutritionAgent)
		fails  []string
		skips  []string
	}{
		{"a core skill off the card", func(g *nutritionAgent) { g.dropSkill = "targets" }, []string{"card"}, nil},
		{"no challenge", func(g *nutritionAgent) { g.noChallenge = true }, []string{"discovery"}, nil},
		// The task still starts, so the checks reading it run; those needing the meal skip.
		{"the logged meal isn't valid", func(g *nutritionAgent) { g.invalidMeal = true }, []string{"log"}, []string{"replay", "read"}},
		{"a retry logs twice", func(g *nutritionAgent) { g.noReplay = true }, []string{"replay"}, nil},
		{"an unknown food logged as sure", func(g *nutritionAgent) { g.confirmAll = true }, []string{"needs-confirmation"}, []string{"answer", "cancel"}},
		// A wrong answer ends the task, and an ended task can't be cancelled, so cancel fails too.
		{"a wrong answer ends the task", func(g *nutritionAgent) { g.answerCompletes = true }, []string{"answer", "cancel"}, nil},
		{"cancel ignored", func(g *nutritionAgent) { g.keepsTasks = true }, []string{"cancel"}, nil},
		{"no GetTask", func(g *nutritionAgent) { g.noGetTask = true }, []string{"get-task"}, nil},
		{"GetTask finds any ID", func(g *nutritionAgent) { g.findsAnyTask = true }, []string{"get-task"}, nil},
		{"get-meals lists nothing", func(g *nutritionAgent) { g.listsNothing = true }, []string{"read"}, nil},
		{"food-lookup without foods", func(g *nutritionAgent) { g.noFoods = true }, []string{"lookup"}, nil},
		{"summary without days", func(g *nutritionAgent) { g.noDays = true }, []string{"summary"}, nil},
		// read-only reads targets too, but only asks that the read isn't refused.
		{"targets not a targets document", func(g *nutritionAgent) { g.badTargets = true }, []string{"targets"}, nil},
		{"an invalid meal accepted", func(g *nutritionAgent) { g.acceptsInvalid = true }, []string{"errors"}, nil},
		{"a read-only token logs", func(g *nutritionAgent) { g.readerWrites = true }, []string{"read-only"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newNutritionAgent()
			c.breaks(g)
			got := map[string]Status{}
			details := map[string]string{}
			for _, r := range runA2A(t, g, "reader").Results {
				got[r.Name], details[r.Name] = r.Status, r.Detail
			}
			for _, name := range A2AChecks {
				want := Pass
				if slices.Contains(c.fails, name) {
					want = Fail
				} else if slices.Contains(c.skips, name) {
					want = Skip
				}
				if got[name] != want {
					t.Errorf("%s = %s, want %s (%s)", name, got[name], want, details[name])
				}
			}
		})
	}
}

// Without a read-only token, read-only is skipped and the agent can't pass.
func TestA2ANeedsAReadOnlyToken(t *testing.T) {
	report := runA2A(t, newNutritionAgent(), "")
	if report.Passed() {
		t.Error("passed without checking a read-only credential")
	}
	for _, r := range report.Results {
		want := Pass
		if r.Name == "read-only" {
			want = Skip
		}
		if r.Status != want {
			t.Errorf("%s = %s, want %s", r.Name, r.Status, want)
		}
	}
}
