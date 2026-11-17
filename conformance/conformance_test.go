package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// reference is an in-memory server keeping to the API core as far as the script goes. A broken
// field turns one behaviour off, so a test can check the right check catches it.
type reference struct {
	mu        sync.Mutex
	next      int
	byID      map[string]map[string]any
	keys      map[string]string
	external  map[string]string
	deleted   map[string]bool
	deletedAt map[string]string
	hooks     map[string]bool

	noChallenge, noReplay, noExternalReplay, revives, loseNotes, badExport bool
	noFeed, noTombstones, listsSecret, loseNutrients                       bool

	saved   map[string]map[string]any
	recipes map[string]map[string]any
	// Broken v0.2 behaviour: a copy that ignores its key, a saved meal logged without its name, a
	// recipe counted per 100 g rather than per serving.
	copyIgnoresKey, savedUntitled, recipeMiscounts bool

	foods map[string]map[string]any
	// Broken v0.3 behaviour: a person's own food that its barcode doesn't find.
	gtinSkipsOwnFoods bool
}

func newReference() *reference {
	return &reference{byID: map[string]map[string]any{}, keys: map[string]string{}, external: map[string]string{}, deleted: map[string]bool{},
		deletedAt: map[string]string{}, hooks: map[string]bool{}, saved: map[string]map[string]any{}, recipes: map[string]map[string]any{},
		foods: map[string]map[string]any{}}
}

func (s *reference) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer good" {
		if !s.noChallenge {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="http://example.test/.well-known/oauth-protected-resource"`)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/meals":
		s.create(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/meals":
		s.feed(w)
	case r.URL.Path == "/v1/webhooks" || strings.HasPrefix(r.URL.Path, "/v1/webhooks/"):
		s.webhooks(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/export":
		s.export(w)
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/meals/") && strings.HasSuffix(r.URL.Path, "/copy"):
		s.copyMeal(w, r, strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/meals/"), "/copy"))
	case strings.HasPrefix(r.URL.Path, "/v1/saved-meals"):
		s.savedMeals(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/recipes"):
		s.recipesRoute(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/foods"):
		s.foodsRoute(w, r)
	case strings.HasPrefix(r.URL.Path, "/v1/meals/"):
		id := strings.TrimPrefix(r.URL.Path, "/v1/meals/")
		m, ok := s.byID[id]
		if !ok || s.deleted[id] {
			problem(w, http.StatusNotFound, "not_found")
			return
		}
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]any{"meal": m, "needs_confirmation": []any{}})
		case http.MethodPatch:
			var patch map[string]any
			_ = json.NewDecoder(r.Body).Decode(&patch)
			for k, v := range patch {
				if k == "notes" && s.loseNotes {
					continue
				}
				m[k] = v
			}
			m["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
			writeJSON(w, http.StatusOK, map[string]any{"meal": m, "needs_confirmation": []any{}})
		case http.MethodDelete:
			s.deleted[id] = true
			s.deletedAt[id] = time.Now().UTC().Format(time.RFC3339Nano)
			writeJSON(w, http.StatusOK, map[string]any{"deleted_at": s.deletedAt[id]})
		}
	default:
		problem(w, http.StatusNotFound, "not_found")
	}
}

func (s *reference) create(w http.ResponseWriter, r *http.Request) {
	var in map[string]any
	_ = json.NewDecoder(r.Body).Decode(&in)
	key := r.Header.Get("Idempotency-Key")
	ext, _ := in["external_id"].(string)
	_, seenKey := s.keys[key]
	if id, ok := s.keys[key]; ok && !s.noReplay {
		s.replay(w, id)
		return
	}
	// Without key replay, a repeated key stores a duplicate whatever the external ID says.
	if id, ok := s.external[ext]; ok && ext != "" && !s.noExternalReplay && !(seenKey && s.noReplay) {
		if s.deleted[id] && !s.revives {
			problem(w, http.StatusNotFound, "not_found")
			return
		}
		delete(s.deleted, id)
		s.replay(w, id)
		return
	}
	s.next++
	id := fmt.Sprintf("meal_%d", s.next)
	var items []any
	for i, it := range in["items"].([]any) {
		it := it.(map[string]any)
		item := map[string]any{"id": fmt.Sprintf("%s_item_%d", id, i), "text": it["text"], "food": map[string]any{"id": nil, "name": it["text"], "match": "none"}}
		if n, ok := it["nutrients"]; ok && !s.loseNutrients {
			item["nutrients"] = n
		}
		if foodID, _ := it["food_id"].(string); foodID != "" {
			if rec, ok := s.recipes[foodID]; ok {
				item["food"] = map[string]any{"id": foodID, "name": rec["name"], "match": "custom"}
				item["amount"] = it["amount"]
				item["nutrients"] = rec["nutrients_per_serving"]
				if s.recipeMiscounts {
					item["nutrients"] = rec["nutrients_per_100g"]
				}
			}
		}
		items = append(items, item)
	}
	at, _ := time.Parse(time.RFC3339, in["eaten_at"].(string))
	m := map[string]any{
		"$schema": "https://openfoodlog.org/schema/v0/meal.json",
		"id":      id, "eaten_at": in["eaten_at"], "date": at.Format(time.DateOnly), "meal_type": in["meal_type"], "title": in["title"], "notes": in["notes"],
		"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
		"source":     map[string]any{"kind": "api", "client": "test", "external_id": ext},
		"items":      items, "estimates": []any{},
	}
	// A field the request left out is absent, not null.
	for _, k := range []string{"title", "notes", "meal_type"} {
		if m[k] == nil {
			delete(m, k)
		}
	}
	s.byID[id], s.keys[key] = m, id
	if _, ok := s.external[ext]; !ok {
		s.external[ext] = id
	}
	w.Header().Set("Location", "/v1/meals/"+id)
	writeJSON(w, http.StatusCreated, map[string]any{"meal": m, "needs_confirmation": []any{}})
}

func (s *reference) replay(w http.ResponseWriter, id string) {
	w.Header().Set("Idempotent-Replayed", "true")
	w.Header().Set("Location", "/v1/meals/"+id)
	writeJSON(w, http.StatusOK, map[string]any{"meal": s.byID[id], "needs_confirmation": []any{}})
}

func (s *reference) export(w http.ResponseWriter) {
	out := []any{}
	for id, m := range s.byID {
		if !s.deleted[id] {
			out = append(out, m)
		}
	}
	if s.badExport {
		out = append(out, map[string]any{"id": "broken"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"$schema": "https://openfoodlog.org/schema/v0/log.json", "exported_at": time.Now().UTC(), "meals": out, "consents": []any{}})
}

// feed is the change feed: every stored meal, and the deleted ones as tombstones.
func (s *reference) feed(w http.ResponseWriter) {
	meals, deleted := []any{}, []any{}
	for id, m := range s.byID {
		switch {
		case s.noFeed:
		case !s.deleted[id]:
			meals = append(meals, m)
		case !s.noTombstones:
			deleted = append(deleted, map[string]any{"id": id, "deleted_at": s.deletedAt[id], "updated_at": s.deletedAt[id]})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"meals": meals, "deleted": deleted})
}

func (s *reference) webhooks(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == http.MethodPost:
		s.next++
		id := fmt.Sprintf("wh_%d", s.next)
		s.hooks[id] = true
		writeJSON(w, http.StatusCreated, map[string]any{"id": id, "secret": "whsec"})
	case r.Method == http.MethodGet:
		list := []any{}
		for id := range s.hooks {
			h := map[string]any{"id": id}
			if s.listsSecret {
				h["secret"] = "whsec"
			}
			list = append(list, h)
		}
		writeJSON(w, http.StatusOK, map[string]any{"webhooks": list})
	case r.Method == http.MethodDelete:
		delete(s.hooks, strings.TrimPrefix(r.URL.Path, "/v1/webhooks/"))
		w.WriteHeader(http.StatusNoContent)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func problem(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status, "code": code})
}

func run(t *testing.T, s *reference) Report {
	t.Helper()
	srv := httptest.NewServer(s)
	t.Cleanup(srv.Close)
	report, err := Run(context.Background(), Config{BaseURL: srv.URL, Token: "good", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func statuses(r Report) map[string]Status {
	out := map[string]Status{}
	for _, res := range r.Results {
		out[res.Name] = res.Status
	}
	return out
}

func TestReferenceServerPasses(t *testing.T) {
	s := newReference()
	report := run(t, s)
	if !report.Passed() {
		for _, res := range report.Results {
			t.Logf("%s %s %s", res.Status, res.Name, res.Detail)
		}
		t.Fatal("the reference server should pass")
	}
	// The run cleans up after itself: its meals are deleted, and its webhook, saved meal and recipe removed.
	for id := range s.byID {
		if !s.deleted[id] {
			t.Errorf("%s left stored", id)
		}
	}
	if len(s.hooks) > 0 {
		t.Errorf("webhooks left subscribed: %v", s.hooks)
	}
	if len(s.saved) > 0 || len(s.recipes) > 0 || len(s.foods) > 0 {
		t.Errorf("left behind: %d saved meals, %d recipes, %d foods", len(s.saved), len(s.recipes), len(s.foods))
	}
}

// Each broken behaviour fails the check that covers it, and only checks that depend on it skip.
func TestEachCheckCatchesItsBehaviour(t *testing.T) {
	cases := []struct {
		name   string
		breaks func(*reference)
		fails  []string
	}{
		{"no challenge", func(s *reference) { s.noChallenge = true }, []string{"discovery"}},
		{"no replay", func(s *reference) { s.noReplay = true }, []string{"replay"}},
		// Without external IDs, a deleted meal's ID can't stay deleted either.
		{"no external replay", func(s *reference) { s.noExternalReplay = true }, []string{"external-id", "stays-deleted"}},
		// Logging again and saved meals keep the items' nutrients, so they catch it too.
		{"nutrients not stored", func(s *reference) { s.loseNutrients = true }, []string{"create", "copy", "saved-meal"}},
		{"notes not changed", func(s *reference) { s.loseNotes = true }, []string{"update"}},
		{"invalid export", func(s *reference) { s.badExport = true }, []string{"export"}},
		{"deleted comes back", func(s *reference) { s.revives = true }, []string{"stays-deleted"}},
		{"no change feed", func(s *reference) { s.noFeed = true }, []string{"change-feed", "tombstone"}},
		{"no tombstones", func(s *reference) { s.noTombstones = true }, []string{"tombstone"}},
		{"secret listed", func(s *reference) { s.listsSecret = true }, []string{"webhooks"}},
		{"barcode skips own foods", func(s *reference) { s.gtinSkipsOwnFoods = true }, []string{"own-food"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newReference()
			c.breaks(s)
			got := statuses(run(t, s))
			for _, name := range Checks {
				want := Pass
				for _, f := range c.fails {
					if name == f {
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

// A server that can't store the meal fails create, and every check that needs it skips, rather
// than piling up failures that all say the same thing.
func TestCreateFailureSkipsTheRest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Bearer resource_metadata="x"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		problem(w, http.StatusForbidden, "consent_required")
	}))
	defer srv.Close()
	report, err := Run(context.Background(), Config{BaseURL: srv.URL, Token: "t", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	got := statuses(report)
	if got["discovery"] != Pass || got["create"] != Fail {
		t.Fatalf("got %v", got)
	}
	if !strings.Contains(report.Results[1].Detail, "consent_required") {
		t.Errorf("create detail %q should name the problem code", report.Results[1].Detail)
	}
	for _, name := range Checks[2:] {
		want := Skip
		if name == "webhooks" || name == "recipe" || name == "own-food" { // need no meal, so they run, and fail here like everything else
			want = Fail
		}
		if got[name] != want {
			t.Errorf("%s = %s, want %s", name, got[name], want)
		}
	}
	if report.Passed() {
		t.Error("a report with failures passed")
	}
}

func TestConfigIsChecked(t *testing.T) {
	for _, cfg := range []Config{{BaseURL: "ftp://x", Token: "t"}, {BaseURL: "https://x"}, {BaseURL: "not a url", Token: "t"}} {
		if _, err := Run(context.Background(), cfg); err == nil {
			t.Errorf("%+v: no error", cfg)
		}
	}
}

// foodsRoute keeps the person's own foods: POST makes one, GET reads one by its ID or a gtin: it
// carries, DELETE removes one (not core, but the run tidies up with it where offered).
func (s *reference) foodsRoute(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/v1/foods"), "/")
	switch {
	case r.Method == http.MethodPost && id == "":
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		s.next++
		f := map[string]any{"id": fmt.Sprintf("usr:food-%d", s.next), "name": in["name"], "kind": "custom",
			"brand": in["brand"], "nutrients_per_100g": in["nutrients_per_100g"]}
		if code, _ := in["gtin"].(string); code != "" {
			f["gtin"] = []any{strings.Repeat("0", 14-len(code)) + code}
		}
		s.foods[f["id"].(string)] = f
		writeJSON(w, http.StatusCreated, f)
	case r.Method == http.MethodGet && strings.HasPrefix(id, "gtin:"):
		code := strings.TrimPrefix(id, "gtin:")
		code = strings.Repeat("0", max(0, 14-len(code))) + code
		for _, f := range s.foods {
			if g, _ := f["gtin"].([]any); len(g) > 0 && g[0] == code && !s.gtinSkipsOwnFoods {
				writeJSON(w, http.StatusOK, f)
				return
			}
		}
		problem(w, http.StatusNotFound, "not_found")
	case r.Method == http.MethodDelete && s.foods[id] != nil:
		delete(s.foods, id)
		w.WriteHeader(http.StatusNoContent)
	default:
		problem(w, http.StatusNotFound, "not_found")
	}
}

func TestOwnFoodGTINIsValid(t *testing.T) {
	for range 50 {
		code := ownFoodGTIN()
		if len(code) != 13 || code[0] != '2' {
			t.Fatalf("%s: want 13 digits starting 2", code)
		}
		if got := code[12] - '0'; got != checkDigit(code[:12]) {
			t.Fatalf("%s: check digit %d, want %d", code, got, checkDigit(code[:12]))
		}
	}
	// A known code: 4006381333931 (GS1's example).
	if d := checkDigit("400638133393"); d != 1 {
		t.Errorf("check digit of 400638133393 = %d, want 1", d)
	}
}
