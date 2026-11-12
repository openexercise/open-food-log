package conformance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	openfoodlog "github.com/openexercise/open-food-log"
)

// A2AChecks are the A2A script's steps, in the order they run.
var A2AChecks = []string{
	"card", "discovery", "log", "replay", "needs-confirmation", "answer", "cancel", "get-task", "read",
	"lookup", "summary", "targets", "errors", "read-only",
}

// A2AProfileURI is the extension a compatible agent's card names, with its core skills in params.
const A2AProfileURI = "https://openfoodlog.org/a2a/v0"

// A2AConfig says which A2A agent to check and as whom.
type A2AConfig struct {
	// URL is where the agent card is served from: its /.well-known/agent-card.json, such as
	// https://mcp.lifterclub.com. The endpoint itself comes from the card.
	URL string
	// Token is a person's OAuth access token for the agent, with meals:read and meals:write.
	Token string
	// ReadOnlyToken is a meals:read token for the same person. Without one, read-only is skipped and
	// the agent can't pass.
	ReadOnlyToken string
	// HTTPClient sends the requests; http.DefaultClient when nil.
	HTTPClient *http.Client
}

// A2AReport is every A2A check's outcome, in order.
type A2AReport struct {
	URL      string   `json:"url"`
	Endpoint string   `json:"endpoint,omitempty"`
	Results  []Result `json:"results"`
}

// Passed reports whether every check passed.
func (r A2AReport) Passed() bool {
	for _, res := range r.Results {
		if res.Status != Pass {
			return false
		}
	}
	return len(r.Results) == len(A2AChecks)
}

// A2ACoreSkills are the skills a compatible agent offers (a2a/README.md): those the published
// card's profile extension lists as core.
func A2ACoreSkills() ([]string, error) {
	var card struct {
		Capabilities struct {
			Extensions []struct {
				URI    string `json:"uri"`
				Params struct {
					CoreSkills []string `json:"core_skills"`
				} `json:"params"`
			} `json:"extensions"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(openfoodlog.A2AProfile, &card); err != nil {
		return nil, fmt.Errorf("read the A2A profile: %w", err)
	}
	for _, e := range card.Capabilities.Extensions {
		if e.URI == A2AProfileURI {
			return e.Params.CoreSkills, nil
		}
	}
	return nil, errors.New("the A2A profile names no core skills")
}

// RunA2A checks an A2A agent against the OFL A2A profile's core as one person, over A2A 1.0
// JSON-RPC: the card names the profile and every core skill and offers OAuth with the meals scopes;
// the endpoint answers 401 with discovery; log-meal logs an Open Food Log meal holding a food no
// catalogue knows, so the task asks for input, and replays the same message; a wrong answer keeps
// it waiting, and CancelTask ends it; GetTask reads it back; get-meals lists the meal; food-lookup,
// nutrition-summary and targets answer with their documents; a message that isn't a meal is refused
// with a profile code; and a read-only token can read but not log. A2A has no delete, so the meal
// stays in the person's log: use a test account.
func RunA2A(ctx context.Context, cfg A2AConfig) (A2AReport, error) {
	base, err := url.Parse(cfg.URL)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return A2AReport{}, fmt.Errorf("agent URL %q: want an http or https URL", cfg.URL)
	}
	if cfg.Token == "" {
		return A2AReport{}, errors.New("no token: the script logs a meal, so it needs a token with meals:write")
	}
	v, err := openfoodlog.NewValidator()
	if err != nil {
		return A2AReport{}, err
	}
	core, err := A2ACoreSkills()
	if err != nil {
		return A2AReport{}, err
	}
	a := &a2aRunner{cfg: cfg, v: v, run: random(), core: core, today: time.Now().UTC().Format(time.DateOnly)}
	if a.client = cfg.HTTPClient; a.client == nil {
		a.client = http.DefaultClient
	}
	a.cardURL = strings.TrimRight(base.String(), "/") + "/.well-known/agent-card.json"
	report := A2AReport{URL: base.String()}
	steps := []func(context.Context) error{
		a.card, a.discovery, a.log, a.replay, a.needsConfirmation, a.answer, a.cancel, a.getTask, a.read,
		a.lookup, a.summary, a.targets, a.errors, a.readOnly,
	}
	for i, step := range steps {
		res := Result{Name: A2AChecks[i], Status: Pass}
		if why := a.blocked(A2AChecks[i]); why != "" {
			res.Status, res.Detail = Skip, why
		} else if err := step(ctx); err != nil {
			res.Status, res.Detail = Fail, err.Error()
		}
		report.Results = append(report.Results, res)
	}
	report.Endpoint = a.endpoint
	return report, nil
}

type a2aRunner struct {
	cfg      A2AConfig
	client   *http.Client
	v        *openfoodlog.Validator
	run      string
	core     []string
	today    string
	cardURL  string
	endpoint string

	message map[string]any // the log-meal message
	task    map[string]any // its task
	mealID  string
	waiting map[string]any // the status message's data part while input-required
}

func (a *a2aRunner) blocked(check string) string {
	switch check {
	case "card":
		return ""
	case "read-only":
		if a.cfg.ReadOnlyToken == "" {
			return "no read-only token: give one (--read-token or OFL_READ_TOKEN) for the same person"
		}
	}
	if a.endpoint == "" {
		return "needs the endpoint, which card didn't find"
	}
	switch check {
	case "replay", "read":
		if a.mealID == "" {
			return "needs a meal, which log didn't store"
		}
	case "needs-confirmation", "get-task":
		if a.task == nil {
			return "needs a task, which log didn't start"
		}
	case "answer", "cancel":
		if a.waiting == nil {
			return "needs a task waiting for input, which needs-confirmation didn't find"
		}
	}
	return ""
}

// card: the agent card names an A2A 1.0 JSON-RPC interface, the profile extension with every core
// skill, each core skill itself, and OAuth 2 with the meals scopes.
func (a *a2aRunner) card(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cardURL, nil)
	if err != nil {
		return err
	}
	res, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", a.cardURL, err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", a.cardURL, res.StatusCode)
	}
	var card struct {
		SupportedInterfaces []struct {
			URL             string `json:"url"`
			ProtocolBinding string `json:"protocolBinding"`
			ProtocolVersion string `json:"protocolVersion"`
		} `json:"supportedInterfaces"`
		Capabilities struct {
			Extensions []struct {
				URI    string `json:"uri"`
				Params struct {
					CoreSkills []string `json:"core_skills"`
				} `json:"params"`
			} `json:"extensions"`
		} `json:"capabilities"`
		Skills []struct {
			ID string `json:"id"`
		} `json:"skills"`
		SecuritySchemes map[string]json.RawMessage `json:"securitySchemes"`
	}
	if err := json.Unmarshal(raw, &card); err != nil {
		return fmt.Errorf("the card isn't JSON: %w", err)
	}
	for _, i := range card.SupportedInterfaces {
		if i.ProtocolBinding == "JSONRPC" && strings.HasPrefix(i.ProtocolVersion, "1.") {
			a.endpoint = i.URL
			break
		}
	}
	if a.endpoint == "" {
		return errors.New("supportedInterfaces has no JSONRPC interface for A2A 1.x")
	}
	var listed []string
	for _, e := range card.Capabilities.Extensions {
		if e.URI == A2AProfileURI {
			listed = e.Params.CoreSkills
		}
	}
	for _, id := range a.core {
		if !slices.Contains(listed, id) {
			return fmt.Errorf("the %s extension's core_skills %v lack %s", A2AProfileURI, listed, id)
		}
		if !slices.ContainsFunc(card.Skills, func(s struct {
			ID string `json:"id"`
		}) bool {
			return s.ID == id
		}) {
			return fmt.Errorf("the card has no skill %s", id)
		}
	}
	for _, scheme := range card.SecuritySchemes {
		s := string(scheme)
		if strings.Contains(s, "meals:read") && strings.Contains(s, "meals:write") && strings.Contains(strings.ToLower(s), "oauth2") {
			return nil
		}
	}
	return errors.New("securitySchemes has no OAuth 2 scheme offering meals:read and meals:write")
}

// discovery: without a credential, the endpoint answers 401 with a Bearer challenge naming its
// protected resource metadata (RFC 9728).
func (a *a2aRunner) discovery(ctx context.Context) error {
	res, err := a.post(ctx, "", "GetTask", map[string]any{"id": "ofl-conformance-" + a.run})
	if err != nil {
		return err
	}
	if res.status != http.StatusUnauthorized {
		return fmt.Errorf("a call without a credential: status %d, want 401", res.status)
	}
	challenge := res.header.Get("WWW-Authenticate")
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer") || !strings.Contains(challenge, "resource_metadata=") {
		return fmt.Errorf("WWW-Authenticate %q: want a Bearer challenge with resource_metadata", challenge)
	}
	return nil
}

// log: an Open Food Log meal with a food no catalogue knows and one the person gave the numbers
// for starts a task whose artifact holds the logged meal, valid against meal.json.
func (a *a2aRunner) log(ctx context.Context) error {
	a.message = map[string]any{
		"messageId": "ofl-conformance-" + a.run, "role": "ROLE_USER",
		"metadata": map[string]any{"skill": "log-meal"},
		"parts": []any{map[string]any{"data": map[string]any{
			"$schema": "https://openfoodlog.org/schema/v0/meal.json", "date": a.today, "title": "OFL conformance " + a.run,
			"source": map[string]any{"kind": "api", "client": "ofl-conformance"},
			"items": []any{
				map[string]any{"text": "zzq conformance stew " + a.run},
				map[string]any{"text": "a snack", "food": map[string]any{"id": nil, "name": "a snack", "match": "none"}, "nutrients": map[string]any{"energy_kcal": 100}},
			},
		}}},
	}
	task, err := a.send(ctx, a.cfg.Token, a.message)
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("log-meal answered with a message, want a task")
	}
	a.task = task
	meal, err := a.artifactMeal(task)
	if err != nil {
		return err
	}
	a.mealID, _ = meal["id"].(string)
	if a.mealID == "" {
		return errors.New("the logged meal has no id")
	}
	return nil
}

// artifactMeal finds the task's artifact holding a meal and validates it.
func (a *a2aRunner) artifactMeal(task map[string]any) (map[string]any, error) {
	artifacts, _ := task["artifacts"].([]any)
	for _, art := range artifacts {
		parts, _ := art.(map[string]any)["parts"].([]any)
		for _, p := range parts {
			data, _ := p.(map[string]any)["data"].(map[string]any)
			if data == nil || data["items"] == nil {
				continue
			}
			raw, _ := json.Marshal(data)
			if problems := a.v.ValidateAs("meal", raw); len(problems) > 0 {
				return nil, fmt.Errorf("the logged meal isn't a valid OFL meal: %s", strings.Join(problems, "; "))
			}
			return data, nil
		}
	}
	return nil, fmt.Errorf("no artifact holds a meal (task state %s)", taskState(task))
}

// replay: the same message again (a retry after a lost reply) returns the same meal.
func (a *a2aRunner) replay(ctx context.Context) error {
	task, err := a.send(ctx, a.cfg.Token, a.message)
	if err != nil {
		return err
	}
	if task == nil {
		return errors.New("the repeated message answered with a message, want a task")
	}
	meal, err := a.artifactMeal(task)
	if err != nil {
		return err
	}
	if meal["id"] != a.mealID {
		return fmt.Errorf("the repeated messageId logged meal %v, want %s again", meal["id"], a.mealID)
	}
	return nil
}

// needsConfirmation: the food no catalogue knows leaves the task input-required, its status
// message listing the item in needs_confirmation.
func (a *a2aRunner) needsConfirmation(context.Context) error {
	if state := taskState(a.task); state != "TASK_STATE_INPUT_REQUIRED" {
		return fmt.Errorf("task state %s, want TASK_STATE_INPUT_REQUIRED for a food no catalogue knows", state)
	}
	for _, p := range statusParts(a.task) {
		data, _ := p.(map[string]any)["data"].(map[string]any)
		list, _ := data["needs_confirmation"].([]any)
		for _, c := range list {
			if c, _ := c.(map[string]any); strings.Contains(fmt.Sprint(c["text"]), a.run) && c["item_id"] != nil {
				a.waiting = data
				return nil
			}
		}
	}
	return errors.New("the status message has no data part whose needs_confirmation lists the unknown food")
}

// answer: an answer naming an item that isn't waiting keeps the task input-required.
func (a *a2aRunner) answer(ctx context.Context) error {
	task, err := a.send(ctx, a.cfg.Token, map[string]any{
		"messageId": "ofl-conformance-answer-" + a.run, "role": "ROLE_USER", "taskId": a.task["id"], "contextId": a.task["contextId"],
		"parts": []any{map[string]any{"data": map[string]any{"confirmations": []any{map[string]any{"item_id": "00000000-0000-0000-0000-000000000000", "food_id": "x"}}}}},
	})
	if err != nil {
		return err
	}
	if task == nil || taskState(task) != "TASK_STATE_INPUT_REQUIRED" {
		return fmt.Errorf("after an answer naming no waiting item: task state %s, want still TASK_STATE_INPUT_REQUIRED", taskState(task))
	}
	return nil
}

// cancel: CancelTask ends the waiting task; the meal stays logged (read checks it).
func (a *a2aRunner) cancel(ctx context.Context) error {
	r, err := a.rpc(ctx, a.cfg.Token, "CancelTask", map[string]any{"id": a.task["id"]})
	if err != nil {
		return err
	}
	if state := taskState(r); state != "TASK_STATE_CANCELED" {
		return fmt.Errorf("CancelTask: state %s, want TASK_STATE_CANCELED", state)
	}
	return nil
}

// getTask: GetTask reads the task back; one that doesn't exist is an error.
func (a *a2aRunner) getTask(ctx context.Context) error {
	r, err := a.rpc(ctx, a.cfg.Token, "GetTask", map[string]any{"id": a.task["id"]})
	if err != nil {
		return err
	}
	if r["id"] != a.task["id"] {
		return fmt.Errorf("GetTask returned task %v, want %v", r["id"], a.task["id"])
	}
	if _, err := a.rpc(ctx, a.cfg.Token, "GetTask", map[string]any{"id": "ofl-conformance-missing-" + a.run}); err == nil {
		return errors.New("GetTask for a task that doesn't exist succeeded")
	}
	return nil
}

// read: get-meals over the day lists the meal, as a valid OFL meal.
func (a *a2aRunner) read(ctx context.Context) error {
	day, _ := time.Parse(time.DateOnly, a.today)
	msg, err := a.ask(ctx, a.cfg.Token, "get-meals", map[string]any{
		"from": day.AddDate(0, 0, -1).Format(time.DateOnly), "to": day.AddDate(0, 0, 1).Format(time.DateOnly),
	})
	if err != nil {
		return err
	}
	for _, data := range dataParts(msg) {
		if data["id"] == a.mealID {
			raw, _ := json.Marshal(data)
			if problems := a.v.ValidateAs("meal", raw); len(problems) > 0 {
				return fmt.Errorf("get-meals' meal isn't a valid OFL meal: %s", strings.Join(problems, "; "))
			}
			return nil
		}
	}
	return fmt.Errorf("get-meals doesn't list meal %s", a.mealID)
}

// lookup: food-lookup answers a query with a foods list.
func (a *a2aRunner) lookup(ctx context.Context) error {
	msg, err := a.ask(ctx, a.cfg.Token, "food-lookup", map[string]any{"query": "banana", "limit": 5})
	if err != nil {
		return err
	}
	for _, data := range dataParts(msg) {
		if _, ok := data["foods"].([]any); ok {
			return nil
		}
	}
	return errors.New("food-lookup's answer has no data part with a foods list")
}

// summary: nutrition-summary answers with the days of the range.
func (a *a2aRunner) summary(ctx context.Context) error {
	msg, err := a.ask(ctx, a.cfg.Token, "nutrition-summary", map[string]any{"from": a.today, "to": a.today})
	if err != nil {
		return err
	}
	for _, data := range dataParts(msg) {
		if days, ok := data["days"].([]any); ok && len(days) == 1 {
			return nil
		}
	}
	return errors.New("nutrition-summary's answer has no data part with one day in days")
}

// targets: reading targets answers with a valid OFL targets document.
func (a *a2aRunner) targets(ctx context.Context) error {
	msg, err := a.ask(ctx, a.cfg.Token, "targets", map[string]any{})
	if err != nil {
		return err
	}
	for _, data := range dataParts(msg) {
		if _, ok := data["targets"]; !ok {
			continue
		}
		raw, _ := json.Marshal(data)
		if problems := a.v.ValidateAs("targets", raw); len(problems) > 0 {
			return fmt.Errorf("the targets aren't a valid OFL targets document: %s", strings.Join(problems, "; "))
		}
		return nil
	}
	return errors.New("targets' answer has no data part with a targets document")
}

// errors: a log-meal message whose meal isn't valid is refused, saving nothing, with a data part
// {"error": {"code"}} whose code is the profile's.
func (a *a2aRunner) errors(ctx context.Context) error {
	r, err := a.rpc(ctx, a.cfg.Token, "SendMessage", map[string]any{"message": map[string]any{
		"messageId": "ofl-conformance-bad-" + a.run, "role": "ROLE_USER", "metadata": map[string]any{"skill": "log-meal"},
		"parts": []any{map[string]any{"data": map[string]any{"items": []any{map[string]any{"name": "not an item"}}}}},
	}})
	if err != nil {
		return err
	}
	code := refusalCode(r)
	if code != "invalid_input" {
		return fmt.Errorf("a meal that isn't valid: error code %q, want invalid_input (task state %s)", code, taskState(r["task"]))
	}
	if task, ok := r["task"].(map[string]any); ok {
		if state := taskState(task); state != "TASK_STATE_REJECTED" && state != "TASK_STATE_FAILED" {
			return fmt.Errorf("a meal that isn't valid: task state %s, want TASK_STATE_REJECTED", state)
		}
	}
	return nil
}

// readOnly: a meals:read token reads but can't log.
func (a *a2aRunner) readOnly(ctx context.Context) error {
	if _, err := a.ask(ctx, a.cfg.ReadOnlyToken, "targets", map[string]any{}); err != nil {
		return fmt.Errorf("reading with the read-only token: %w", err)
	}
	r, err := a.rpc(ctx, a.cfg.ReadOnlyToken, "SendMessage", map[string]any{"message": map[string]any{
		"messageId": "ofl-conformance-ro-" + a.run, "role": "ROLE_USER", "metadata": map[string]any{"skill": "log-meal"},
		"parts": []any{map[string]any{"data": map[string]any{
			"date": a.today, "source": map[string]any{"kind": "api"}, "items": []any{map[string]any{"text": "zzq read-only " + a.run}},
		}}},
	}})
	if err != nil {
		// Refused by the transport (403, or a JSON-RPC error) is a refusal too.
		return nil
	}
	if code := refusalCode(r); code == "" {
		return fmt.Errorf("the read-only token logged a meal (task state %s)", taskState(r["task"]))
	}
	return nil
}

// send sends a message and returns the task it answered with, or nil for a message.
func (a *a2aRunner) send(ctx context.Context, token string, message map[string]any) (map[string]any, error) {
	r, err := a.rpc(ctx, token, "SendMessage", map[string]any{"message": message})
	if err != nil {
		return nil, err
	}
	task, _ := r["task"].(map[string]any)
	if task == nil && r["message"] == nil {
		return nil, fmt.Errorf("SendMessage's result holds neither a task nor a message: %s", clipJSON(r))
	}
	return task, nil
}

// ask sends a message naming a skill with one data part, and returns the message it answered with.
// A refusal is an error.
func (a *a2aRunner) ask(ctx context.Context, token, skill string, data map[string]any) (map[string]any, error) {
	r, err := a.rpc(ctx, token, "SendMessage", map[string]any{"message": map[string]any{
		"messageId": "ofl-conformance-" + skill + "-" + random(), "role": "ROLE_USER", "metadata": map[string]any{"skill": skill},
		"parts": []any{map[string]any{"data": data}},
	}})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", skill, err)
	}
	if code := refusalCode(r); code != "" {
		return nil, fmt.Errorf("%s was refused: %s", skill, code)
	}
	if msg, ok := r["message"].(map[string]any); ok {
		return msg, nil
	}
	if task, ok := r["task"].(map[string]any); ok {
		// An agent may answer with a completed task: its artifacts carry the answer.
		out := map[string]any{"parts": []any{}}
		for _, art := range anySlice(task["artifacts"]) {
			out["parts"] = append(out["parts"].([]any), anySlice(art.(map[string]any)["parts"])...)
		}
		return out, nil
	}
	return nil, fmt.Errorf("%s answered with neither a message nor a task", skill)
}

type a2aResponse struct {
	status int
	header http.Header
	body   []byte
}

// post sends one JSON-RPC request with A2A-Version 1.0.
func (a *a2aRunner) post(ctx context.Context, token, method string, params any) (*a2aResponse, error) {
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("A2A-Version", "1.0")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", method, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 16<<20))
	if err != nil {
		return nil, fmt.Errorf("%s: reading the reply: %w", method, err)
	}
	return &a2aResponse{status: res.StatusCode, header: res.Header, body: body}, nil
}

// rpc sends one JSON-RPC request and returns its result, or an error for a JSON-RPC error or a
// status other than 200.
func (a *a2aRunner) rpc(ctx context.Context, token, method string, params any) (map[string]any, error) {
	res, err := a.post(ctx, token, method, params)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Result map[string]any `json:"result"`
		Error  map[string]any `json:"error"`
	}
	if err := json.Unmarshal(res.body, &reply); err != nil {
		return nil, fmt.Errorf("%s: status %d, not JSON-RPC: %s", method, res.status, clip(res.body))
	}
	if reply.Error != nil {
		return nil, fmt.Errorf("%s: JSON-RPC error %v: %v", method, reply.Error["code"], reply.Error["message"])
	}
	if res.status != http.StatusOK {
		return nil, fmt.Errorf("%s: status %d", method, res.status)
	}
	return reply.Result, nil
}

func taskState(task any) string {
	t, _ := task.(map[string]any)
	status, _ := t["status"].(map[string]any)
	s, _ := status["state"].(string)
	return s
}

func statusParts(task map[string]any) []any {
	status, _ := task["status"].(map[string]any)
	msg, _ := status["message"].(map[string]any)
	return anySlice(msg["parts"])
}

func dataParts(msg map[string]any) []map[string]any {
	var out []map[string]any
	for _, p := range anySlice(msg["parts"]) {
		if data, ok := p.(map[string]any)["data"].(map[string]any); ok {
			out = append(out, data)
		}
	}
	return out
}

// refusalCode is the error code of a refused message (an answer, or a task's status message, with a
// data part {"error": {"code"}}), or "".
func refusalCode(result map[string]any) string {
	var parts []any
	if msg, ok := result["message"].(map[string]any); ok {
		parts = anySlice(msg["parts"])
	}
	if task, ok := result["task"].(map[string]any); ok {
		parts = statusParts(task)
	}
	for _, p := range parts {
		data, _ := p.(map[string]any)["data"].(map[string]any)
		if e, ok := data["error"].(map[string]any); ok {
			code, _ := e["code"].(string)
			return code
		}
	}
	return ""
}

func anySlice(v any) []any {
	s, _ := v.([]any)
	return s
}

func clipJSON(v any) string {
	raw, _ := json.Marshal(v)
	return clip(raw)
}
