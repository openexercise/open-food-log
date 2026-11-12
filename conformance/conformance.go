// Package conformance checks that a server offers the Open Food Log API core and MCP profile.
//
// Run checks the REST API by running a fixed script against it as one person: log a meal, read it
// back equal, replay the Idempotency-Key and the external ID, change it, find it in the change
// feed, export the log and validate every meal in it, delete it, find its tombstone, check a
// deleted external ID stays deleted, and subscribe a webhook and remove it. RunMCP checks an MCP
// server against the tool profile (mcp.go), and CheckVectors checks an implementation of the
// amount calculation against the published cases (vectors.go). VerifySignature checks a webhook
// delivery's OFL-Signature.
//
// The script writes one meal to the account and deletes it at the end, also when a check fails
// part way. Use a test account: the token needs meals:read and meals:write, for a person who has
// given whatever consent the server asks before it stores health data.
//
// "Conformant" is self-reported: a server that passes may say so. The reference implementation
// is Lifterclub (https://lifterclub.com/developers), whose CI runs this suite. The script mirrors
// Open Workout's (github.com/openexercise/open-workout/conformance), so a server of both runs the
// same steps twice.
package conformance

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	openfoodlog "github.com/openexercise/open-food-log"
)

// Checks are the script's steps, in the order they run.
var Checks = []string{
	"discovery", "create", "read", "replay", "external-id", "update", "change-feed", "export", "delete", "tombstone",
	"stays-deleted", "webhooks",
}

// Status is a check's outcome.
type Status string

const (
	Pass Status = "pass"
	Fail Status = "fail"
	// Skip is a check that couldn't run because one it depends on failed.
	Skip Status = "skip"
)

// Result is one check's outcome.
type Result struct {
	Name   string `json:"name"`
	Status Status `json:"status"`
	// Detail says what was wrong, or why the check was skipped.
	Detail string `json:"detail,omitempty"`
}

// Report is every check's outcome, in order.
type Report struct {
	BaseURL string   `json:"base_url"`
	Results []Result `json:"results"`
}

// Passed reports whether every check passed.
func (r Report) Passed() bool {
	for _, res := range r.Results {
		if res.Status != Pass {
			return false
		}
	}
	return len(r.Results) == len(Checks)
}

// Config says where to run the script and as whom.
type Config struct {
	// BaseURL is the API's origin, such as https://api.lifterclub.com. Routes are under /v1.
	BaseURL string
	// Token is an OAuth access token with meals:read and meals:write.
	Token string
	// HTTPClient sends the requests; http.DefaultClient when nil.
	HTTPClient *http.Client
	// Now is the clock the meal's time comes from; time.Now when nil.
	Now func() time.Time
}

// Run runs every check in order and reports each one. The error is for a run that couldn't
// start (a bad base URL, no token); a failing server is a report with failures, not an error.
func Run(ctx context.Context, cfg Config) (Report, error) {
	base, err := url.Parse(strings.TrimRight(cfg.BaseURL, "/"))
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return Report{}, fmt.Errorf("base URL %q: want an http or https origin", cfg.BaseURL)
	}
	if cfg.Token == "" {
		return Report{}, errors.New("no token: the script logs a meal, so it needs meals:read and meals:write")
	}
	v, err := openfoodlog.NewValidator()
	if err != nil {
		return Report{}, err
	}
	r := &runner{cfg: cfg, base: base, client: cfg.HTTPClient, v: v, now: cfg.Now, run: random()}
	if r.client == nil {
		r.client = http.DefaultClient
	}
	if r.now == nil {
		r.now = time.Now
	}
	r.input = r.mealInput()
	report := Report{BaseURL: base.String()}
	steps := []func(context.Context) error{
		r.discovery, r.create, r.read, r.replay, r.externalID, r.update, r.changeFeed, r.export, r.delete, r.tombstone,
		r.staysDeleted, r.webhooks,
	}
	for i, step := range steps {
		res := Result{Name: Checks[i], Status: Pass}
		if why := r.blocked(Checks[i]); why != "" {
			res.Status, res.Detail = Skip, why
		} else if err := step(ctx); err != nil {
			res.Status, res.Detail = Fail, err.Error()
		}
		report.Results = append(report.Results, res)
	}
	r.cleanUp(ctx)
	return report, nil
}

// blocked says why a check can't run, when one it depends on failed.
func (r *runner) blocked(check string) string {
	switch {
	case check == "discovery" || check == "create" || check == "webhooks":
		return ""
	case r.id == "":
		return "needs a meal, which create didn't store"
	case (check == "stays-deleted" || check == "tombstone") && !r.deleted:
		return "needs the meal deleted, which delete didn't do"
	}
	return ""
}

type runner struct {
	cfg    Config
	base   *url.URL
	client *http.Client
	v      *openfoodlog.Validator
	now    func() time.Time
	run    string

	input   map[string]any
	key     string         // the create's Idempotency-Key
	id      string         // the stored meal's ID
	path    string         // where it lives, from Location
	stored  map[string]any // the meal as create returned it
	since   time.Time      // the change feed's start: just before the server stored the meal
	deleted bool
}

const (
	newTitle = "ofl conformance (changed)"
	newNotes = "Changed by ofl conformance."
)

// mealInput is the meal the script logs: two items that name no food, only the nutrients the
// person gave (so no catalogue is needed and nothing is matched), eaten an hour ago to the second,
// and an external ID unique to this run.
func (r *runner) mealInput() map[string]any {
	return map[string]any{
		"eaten_at":    r.now().Add(-time.Hour).UTC().Truncate(time.Second).Format(time.RFC3339),
		"meal_type":   "snack",
		"title":       "ofl conformance " + r.run,
		"notes":       "Written by ofl conformance and deleted at the end of its run.",
		"external_id": "ofl-conformance-" + r.run,
		"items": []any{
			map[string]any{"text": "ofl conformance bar", "nutrients": map[string]any{"energy_kcal": 250, "protein_g": 20}},
			map[string]any{"text": "ofl conformance drink", "nutrients": map[string]any{"energy_kcal": 90}},
		},
	}
}

// discovery: without a token, the API answers 401 and points at its protected resource
// metadata (RFC 9728), which is how a client finds the authorization server.
func (r *runner) discovery(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodGet, "/v1/meals", nil, nil, false)
	if err != nil {
		return err
	}
	if res.status != http.StatusUnauthorized {
		return fmt.Errorf("GET /v1/meals without a token: status %d, want 401", res.status)
	}
	challenge := res.header.Get("WWW-Authenticate")
	if !strings.HasPrefix(strings.ToLower(challenge), "bearer") || !strings.Contains(challenge, "resource_metadata=") {
		return fmt.Errorf("WWW-Authenticate %q: want a Bearer challenge with resource_metadata", challenge)
	}
	return nil
}

// create: POST /v1/meals stores the meal and answers 201 with Location and the stored meal, a
// valid OFL document holding what was sent.
func (r *runner) create(ctx context.Context) error {
	r.key = "ofl-conformance-" + random()
	res, err := r.send(ctx, http.MethodPost, "/v1/meals", r.input, header("Idempotency-Key", r.key), true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST /v1/meals", http.StatusCreated)
	}
	m, err := r.written(res)
	if err != nil {
		return err
	}
	id, _ := m["id"].(string)
	if id == "" {
		return errors.New("the stored meal has no id")
	}
	r.id, r.stored = id, m
	// The change feed reads from just before the server stored it, by the server's own clock when
	// the meal says, so clock skew between here and there can't hide it.
	r.since = r.now().Add(-5 * time.Minute)
	if at, ok := m["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339Nano, at); err == nil {
			r.since = t.Add(-time.Second)
		}
	}
	r.path = "/v1/meals/" + url.PathEscape(id)
	if loc := res.header.Get("Location"); loc == "" {
		return errors.New("201 without a Location header")
	} else if u, err := r.base.Parse(loc); err != nil || !strings.HasSuffix(u.Path, "/"+id) {
		return fmt.Errorf("Location %q doesn't name meal %s", loc, id)
	}
	return matchesInput(m, r.input)
}

// read: GET the meal back, valid and equal to what create returned.
func (r *runner) read(ctx context.Context) error {
	m, err := r.get(ctx)
	if err != nil {
		return err
	}
	if diff := differ(stable(r.stored), stable(m)); diff != "" {
		return fmt.Errorf("read back differs from what create returned: %s", diff)
	}
	return nil
}

// replay: the same Idempotency-Key and body return the first result (200, Idempotent-Replayed)
// and store nothing new.
func (r *runner) replay(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodPost, "/v1/meals", r.input, header("Idempotency-Key", r.key), true)
	if err != nil {
		return err
	}
	return r.replayed(res, "a repeated Idempotency-Key")
}

// external-id: a new Idempotency-Key with the same external_id replays too, so an app syncing
// its own records never logs one twice.
func (r *runner) externalID(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodPost, "/v1/meals", r.input, header("Idempotency-Key", "ofl-conformance-"+random()), true)
	if err != nil {
		return err
	}
	return r.replayed(res, "a repeated external_id")
}

func (r *runner) replayed(res *response, what string) error {
	if res.status != http.StatusOK {
		return res.unexpected("POST /v1/meals with "+what, http.StatusOK)
	}
	if res.header.Get("Idempotent-Replayed") != "true" {
		return fmt.Errorf("%s: no Idempotent-Replayed: true header", what)
	}
	m, err := r.written(res)
	if err != nil {
		return err
	}
	if m["id"] != r.id {
		return fmt.Errorf("%s returned meal %v, want %s: it was stored twice", what, m["id"], r.id)
	}
	return nil
}

// update: PATCH changes the title and notes; the result and a read both show the change, and
// updated_at, when the server keeps it, doesn't go backwards.
func (r *runner) update(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodPatch, r.path, map[string]any{"title": newTitle, "notes": newNotes}, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK {
		return res.unexpected("PATCH "+r.path, http.StatusOK)
	}
	written, err := r.written(res)
	if err != nil {
		return err
	}
	m, err := r.get(ctx)
	if err != nil {
		return err
	}
	for _, doc := range []map[string]any{written, m} {
		if doc["title"] != newTitle || doc["notes"] != newNotes {
			return fmt.Errorf("after PATCH: title %v and notes %v, want %q and %q", doc["title"], doc["notes"], newTitle, newNotes)
		}
	}
	before, _ := r.stored["updated_at"].(string)
	after, _ := m["updated_at"].(string)
	if before != "" && after != "" {
		b, errB := time.Parse(time.RFC3339Nano, before)
		a, errA := time.Parse(time.RFC3339Nano, after)
		if errB != nil || errA != nil || a.Before(b) {
			return fmt.Errorf("updated_at went from %s to %s", before, after)
		}
	}
	return nil
}

// change-feed: GET /v1/meals?updated_since= lists the changed meal, valid, as a mirror syncing
// the log would read it.
func (r *runner) changeFeed(ctx context.Context) error {
	feed, err := r.feed(ctx)
	if err != nil {
		return err
	}
	for _, raw := range feed.Meals {
		if problems := r.v.ValidateAs("meal", raw); len(problems) > 0 {
			return fmt.Errorf("a meal in the change feed isn't valid OFL: %s", problems[0])
		}
		var m struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &m) == nil && m.ID == r.id {
			return nil
		}
	}
	return fmt.Errorf("the change feed since %s doesn't list meal %s", r.since.Format(time.RFC3339), r.id)
}

// export: GET /v1/export?format=json returns a valid OFL log holding this meal with its change.
func (r *runner) export(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodGet, "/v1/export?format=json", nil, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK {
		return res.unexpected("GET /v1/export?format=json", http.StatusOK)
	}
	if problems := r.v.ValidateAs("log", res.body); len(problems) > 0 {
		return fmt.Errorf("the export isn't a valid OFL log: %s", problems[0])
	}
	var log struct {
		Meals []map[string]any `json:"meals"`
	}
	if err := json.Unmarshal(res.body, &log); err != nil {
		return fmt.Errorf("export isn't a JSON object with meals: %v", err)
	}
	for _, m := range log.Meals {
		if m["id"] == r.id {
			if m["title"] != newTitle {
				return fmt.Errorf("the export holds this run's meal with title %v, want %q", m["title"], newTitle)
			}
			return nil
		}
	}
	return fmt.Errorf("the export's %d meals don't include %s", len(log.Meals), r.id)
}

// delete: DELETE removes the meal; reading it is then a not_found problem.
func (r *runner) delete(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodDelete, r.path, nil, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK && res.status != http.StatusNoContent {
		return res.unexpected("DELETE "+r.path, http.StatusOK)
	}
	r.deleted = true
	res, err = r.send(ctx, http.MethodGet, r.path, nil, nil, true)
	if err != nil {
		return err
	}
	return res.problem("GET a deleted meal", http.StatusNotFound, "not_found")
}

// tombstone: after the delete, the change feed with include_deleted lists the meal as a tombstone
// in deleted, and no longer as a meal, so a mirror removes its copy.
func (r *runner) tombstone(ctx context.Context) error {
	feed, err := r.feed(ctx)
	if err != nil {
		return err
	}
	for _, raw := range feed.Meals {
		var m struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &m) == nil && m.ID == r.id {
			return fmt.Errorf("the change feed still lists deleted meal %s as a meal", r.id)
		}
	}
	for _, t := range feed.Deleted {
		if t.ID == r.id {
			if t.DeletedAt == "" || t.UpdatedAt == "" {
				return fmt.Errorf("the tombstone for %s lacks deleted_at or updated_at", r.id)
			}
			return nil
		}
	}
	return fmt.Errorf("the change feed with include_deleted has no tombstone for meal %s", r.id)
}

// stays-deleted: logging the same external_id again, as a sync would after the person deleted
// it, doesn't bring it back.
func (r *runner) staysDeleted(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodPost, "/v1/meals", r.input, header("Idempotency-Key", "ofl-conformance-"+random()), true)
	if err != nil {
		return err
	}
	return res.problem("POST a deleted meal's external_id", http.StatusNotFound, "not_found")
}

// cleanUp deletes the meal if a failed check left it stored.
func (r *runner) cleanUp(ctx context.Context) {
	if r.id != "" && !r.deleted {
		_, _ = r.send(context.WithoutCancel(ctx), http.MethodDelete, r.path, nil, nil, true)
	}
}

// get reads the meal: 200 with a valid OFL meal.
func (r *runner) get(ctx context.Context) (map[string]any, error) {
	res, err := r.send(ctx, http.MethodGet, r.path, nil, nil, true)
	if err != nil {
		return nil, err
	}
	if res.status != http.StatusOK {
		return nil, res.unexpected("GET "+r.path, http.StatusOK)
	}
	return r.written(res)
}

// written reads a result: a JSON object whose meal is a valid OFL document.
func (r *runner) written(res *response) (map[string]any, error) {
	if err := res.isJSON(); err != nil {
		return nil, err
	}
	var body struct {
		Meal json.RawMessage `json:"meal"`
	}
	if err := json.Unmarshal(res.body, &body); err != nil || len(body.Meal) == 0 {
		return nil, fmt.Errorf("the result has no meal: %s", clip(res.body))
	}
	if p := r.v.ValidateAs("meal", body.Meal); len(p) > 0 {
		return nil, fmt.Errorf("the result's meal isn't valid OFL: %s", p[0])
	}
	var m map[string]any
	if err := json.Unmarshal(body.Meal, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// matchesInput checks the stored meal holds what was sent: time, type, title, notes, external
// ID, and each item's words and nutrients in order.
func matchesInput(m, in map[string]any) error {
	at, err := time.Parse(time.RFC3339Nano, fmt.Sprint(m["eaten_at"]))
	want, _ := time.Parse(time.RFC3339, in["eaten_at"].(string))
	if err != nil || !at.Equal(want) {
		return fmt.Errorf("eaten_at %v, want %s", m["eaten_at"], in["eaten_at"])
	}
	for _, f := range []string{"meal_type", "title", "notes"} {
		if m[f] != in[f] {
			return fmt.Errorf("%s %v, want %q", f, m[f], in[f])
		}
	}
	src, _ := m["source"].(map[string]any)
	if src["external_id"] != in["external_id"] {
		return fmt.Errorf("source.external_id %v, want %q", src["external_id"], in["external_id"])
	}
	items, _ := m["items"].([]any)
	sent := in["items"].([]any)
	if len(items) != len(sent) {
		return fmt.Errorf("%d items stored, want %d", len(items), len(sent))
	}
	for i := range sent {
		got, want := items[i].(map[string]any), sent[i].(map[string]any)
		if got["text"] != want["text"] {
			return fmt.Errorf("items[%d].text %v, want %q", i, got["text"], want["text"])
		}
		gotN, _ := got["nutrients"].(map[string]any)
		wantN := want["nutrients"].(map[string]any)
		if len(gotN) != len(wantN) {
			return fmt.Errorf("items[%d].nutrients %v, want %v", i, gotN, wantN)
		}
		for k, v := range wantN {
			if fmt.Sprint(gotN[k]) != fmt.Sprint(v) {
				return fmt.Errorf("items[%d].nutrients.%s %v, want %v", i, k, gotN[k], v)
			}
		}
	}
	return nil
}

// stable drops what may differ between two reads of an unchanged meal: nulls, which a server may
// write or leave out, and updated_at.
func stable(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, x := range t {
			if x == nil || k == "updated_at" {
				continue
			}
			out[k] = stable(x)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, x := range t {
			out[i] = stable(x)
		}
		return out
	}
	return v
}

// differ names the first place a and b differ, or "".
func differ(a, b any) string {
	if reflect.DeepEqual(a, b) {
		return ""
	}
	return walk("", a, b)
}

func walk(at string, a, b any) string {
	am, aok := a.(map[string]any)
	bm, bok := b.(map[string]any)
	if aok && bok {
		for k := range am {
			if d := walk(at+"/"+k, am[k], bm[k]); d != "" {
				return d
			}
		}
		for k := range bm {
			if _, ok := am[k]; !ok {
				return fmt.Sprintf("%s/%s only in the read", at, k)
			}
		}
		return ""
	}
	as, aok := a.([]any)
	bs, bok := b.([]any)
	if aok && bok {
		if len(as) != len(bs) {
			return fmt.Sprintf("%s has %d items, then %d", at, len(as), len(bs))
		}
		for i := range as {
			if d := walk(fmt.Sprintf("%s/%d", at, i), as[i], bs[i]); d != "" {
				return d
			}
		}
		return ""
	}
	if !reflect.DeepEqual(a, b) {
		return fmt.Sprintf("%s was %v, then %v", at, a, b)
	}
	return ""
}

type changeFeed struct {
	Meals   []json.RawMessage `json:"meals"`
	Deleted []struct {
		ID        string `json:"id"`
		DeletedAt string `json:"deleted_at"`
		UpdatedAt string `json:"updated_at"`
	} `json:"deleted"`
}

// feed reads the change feed since the meal was stored, with tombstones.
func (r *runner) feed(ctx context.Context) (changeFeed, error) {
	path := "/v1/meals?" + url.Values{"updated_since": {r.since.UTC().Format(time.RFC3339Nano)}, "include_deleted": {"true"}, "limit": {"100"}}.Encode()
	res, err := r.send(ctx, http.MethodGet, path, nil, nil, true)
	if err != nil {
		return changeFeed{}, err
	}
	if res.status != http.StatusOK {
		return changeFeed{}, res.unexpected("GET the change feed", http.StatusOK)
	}
	if err := res.isJSON(); err != nil {
		return changeFeed{}, err
	}
	var feed changeFeed
	if err := json.Unmarshal(res.body, &feed); err != nil {
		return changeFeed{}, fmt.Errorf("the change feed: %w", err)
	}
	return feed, nil
}

// webhooks: POST /v1/webhooks subscribes to the meal events and answers 201 with the signing
// secret, the list shows the subscription without it, and DELETE removes it. Nothing is
// delivered: the URL is on example.com, and the subscription is gone before anything changes.
func (r *runner) webhooks(ctx context.Context) error {
	in := map[string]any{"url": "https://example.com/ofl-conformance/" + r.run, "events": []string{"meal.created", "meal.updated", "meal.deleted"}}
	res, err := r.send(ctx, http.MethodPost, "/v1/webhooks", in, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST /v1/webhooks", http.StatusCreated)
	}
	var created struct {
		ID     string `json:"id"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(res.body, &created); err != nil || created.ID == "" {
		return fmt.Errorf("POST /v1/webhooks: no id in %s", clip(res.body))
	}
	path := "/v1/webhooks/" + url.PathEscape(created.ID)
	defer r.send(context.WithoutCancel(ctx), http.MethodDelete, path, nil, nil, true) //nolint:errcheck // cleanup
	if created.Secret == "" {
		return errors.New("POST /v1/webhooks: no secret to verify deliveries with")
	}
	listed, err := r.webhookListed(ctx, created.ID)
	if err != nil {
		return err
	}
	if !listed {
		return fmt.Errorf("GET /v1/webhooks doesn't list subscription %s", created.ID)
	}
	res, err = r.send(ctx, http.MethodDelete, path, nil, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusNoContent && res.status != http.StatusOK {
		return res.unexpected("DELETE "+path, http.StatusNoContent)
	}
	if listed, err = r.webhookListed(ctx, created.ID); err != nil {
		return err
	}
	if listed {
		return fmt.Errorf("subscription %s is still listed after DELETE", created.ID)
	}
	return nil
}

func (r *runner) webhookListed(ctx context.Context, id string) (bool, error) {
	res, err := r.send(ctx, http.MethodGet, "/v1/webhooks", nil, nil, true)
	if err != nil {
		return false, err
	}
	if res.status != http.StatusOK {
		return false, res.unexpected("GET /v1/webhooks", http.StatusOK)
	}
	var list struct {
		Webhooks []map[string]any `json:"webhooks"`
	}
	if err := json.Unmarshal(res.body, &list); err != nil {
		return false, fmt.Errorf("GET /v1/webhooks: %w", err)
	}
	for _, h := range list.Webhooks {
		if h["id"] == id {
			if _, ok := h["secret"]; ok {
				return true, errors.New("GET /v1/webhooks shows the secret, which only the creation response may")
			}
			return true, nil
		}
	}
	return false, nil
}

type response struct {
	status int
	header http.Header
	body   []byte
}

// unexpected describes a status other than want, with the problem's code when there is one.
func (res *response) unexpected(what string, want int) error {
	var p struct {
		Code   string `json:"code"`
		Detail string `json:"detail"`
		Error  string `json:"error"`
	}
	_ = json.Unmarshal(res.body, &p)
	msg := fmt.Sprintf("%s: status %d, want %d", what, res.status, want)
	if code := p.Code + p.Error; code != "" {
		msg += " (" + code
		if p.Detail != "" {
			msg += ": " + p.Detail
		}
		msg += ")"
	}
	if res.status == http.StatusTooManyRequests {
		msg += "; rate limited, Retry-After " + res.header.Get("Retry-After")
	}
	return errors.New(msg)
}

func (res *response) isJSON() error {
	ctype, _, _ := mime.ParseMediaType(res.header.Get("Content-Type"))
	if ctype != "application/json" {
		return fmt.Errorf("Content-Type %q, want application/json", res.header.Get("Content-Type"))
	}
	return nil
}

// problem checks an RFC 9457 problem with the status and code.
func (res *response) problem(what string, status int, code string) error {
	if res.status != status {
		return res.unexpected(what, status)
	}
	ctype, _, _ := mime.ParseMediaType(res.header.Get("Content-Type"))
	if ctype != "application/problem+json" {
		return fmt.Errorf("%s: Content-Type %q, want application/problem+json", what, res.header.Get("Content-Type"))
	}
	var p struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(res.body, &p); err != nil || p.Code != code {
		return fmt.Errorf("%s: problem code %q, want %q", what, p.Code, code)
	}
	return nil
}

// send makes one request. A transport error is the check's failure.
func (r *runner) send(ctx context.Context, method, path string, body any, h http.Header, auth bool) (*response, error) {
	target, err := r.base.Parse(path)
	if err != nil {
		return nil, err
	}
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target.String(), rd)
	if err != nil {
		return nil, err
	}
	for k, vs := range h {
		req.Header[k] = vs
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ofl-conformance/0")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+r.cfg.Token)
	}
	res, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("%s %s: reading the reply: %w", method, path, err)
	}
	return &response{status: res.StatusCode, header: res.Header, body: raw}, nil
}

func header(k, v string) http.Header { return http.Header{k: {v}} }

func random() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func clip(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "…"
	}
	return string(b)
}
