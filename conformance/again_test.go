package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// The reference server's v0.2 routes: logging a meal again, saved meals and recipes, as far as the
// script goes.

// copyMeal logs a meal again through create, so its key replays as a meal's does.
func (s *reference) copyMeal(w http.ResponseWriter, r *http.Request, id string) {
	m, ok := s.byID[id]
	if !ok || s.deleted[id] {
		problem(w, http.StatusNotFound, "not_found")
		return
	}
	key := "copy:" + r.Header.Get("Idempotency-Key")
	if s.copyIgnoresKey {
		key = fmt.Sprintf("copy:%d", s.next)
	}
	s.logItems(w, key, m["title"], m["items"].([]any))
}

// logItems logs a meal of these items, as create would, keyed so a retry replays.
func (s *reference) logItems(w http.ResponseWriter, key string, title any, items []any) {
	if id, ok := s.keys[key]; ok {
		s.replay(w, id)
		return
	}
	in := []any{}
	for _, it := range items {
		it := it.(map[string]any)
		in = append(in, map[string]any{"text": it["text"], "nutrients": it["nutrients"]})
	}
	body := map[string]any{"eaten_at": "2026-11-13T08:00:00Z", "meal_type": "snack", "title": title, "notes": "logged again", "items": in}
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, "/v1/meals", strings.NewReader(string(raw)))
	req.Header.Set("Idempotency-Key", key)
	s.create(w, req)
}

func (s *reference) savedMeals(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/v1/saved-meals"), "/")
	switch {
	case r.Method == http.MethodPost && rest == "":
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		from, ok := s.byID[fmt.Sprint(in["from_meal_id"])]
		if !ok {
			problem(w, http.StatusBadRequest, "invalid_input")
			return
		}
		s.next++
		id := fmt.Sprintf("saved:%d", s.next)
		items := []any{}
		for _, it := range from["items"].([]any) {
			it := it.(map[string]any)
			items = append(items, map[string]any{"text": it["text"], "food": it["food"], "nutrients": it["nutrients"]})
		}
		sm := map[string]any{"$schema": "https://openfoodlog.org/schema/v0/saved-meal.json", "ofl_version": "0.2", "id": id, "name": in["name"], "items": items}
		s.saved[id] = sm
		w.Header().Set("Location", "/v1/saved-meals/"+id)
		writeJSON(w, http.StatusCreated, sm)
	case r.Method == http.MethodGet && rest == "":
		list := []any{}
		for _, sm := range s.saved {
			list = append(list, sm)
		}
		writeJSON(w, http.StatusOK, map[string]any{"saved_meals": list})
	default:
		id, action, _ := strings.Cut(rest, "/")
		sm, ok := s.saved[id]
		if !ok {
			problem(w, http.StatusNotFound, "not_found")
			return
		}
		switch {
		case r.Method == http.MethodPost && action == "log":
			title := sm["name"]
			if s.savedUntitled {
				title = "Snack"
			}
			s.logItems(w, "saved:"+r.Header.Get("Idempotency-Key"), title, sm["items"].([]any))
		case r.Method == http.MethodGet:
			writeJSON(w, http.StatusOK, sm)
		case r.Method == http.MethodDelete:
			delete(s.saved, id)
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// recipesRoute keeps recipes whose lines name no food: their nutrients over the yield, and over
// the servings.
func (s *reference) recipesRoute(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/v1/recipes"), "/")
	if id != "" {
		if _, ok := s.recipes[id]; !ok {
			problem(w, http.StatusNotFound, "not_found")
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.recipes[id])
	case http.MethodDelete:
		delete(s.recipes, id)
		w.WriteHeader(http.StatusNoContent)
	case http.MethodPost, http.MethodPut:
		var in map[string]any
		_ = json.NewDecoder(r.Body).Decode(&in)
		status := http.StatusOK
		if id == "" {
			s.next++
			id, status = fmt.Sprintf("recipe:%d", s.next), http.StatusCreated
			w.Header().Set("Location", "/v1/recipes/"+id)
		}
		servings, yield := in["servings"].(float64), in["yield_g"].(float64)
		sum := map[string]float64{}
		lines := []any{}
		for _, l := range in["lines"].([]any) {
			l := l.(map[string]any)
			n := map[string]float64{}
			raw, _ := json.Marshal(l["nutrients"])
			_ = json.Unmarshal(raw, &n)
			for k, v := range n {
				sum[k] += v
			}
			lines = append(lines, map[string]any{"text": l["text"], "food": map[string]any{"id": nil, "name": l["text"], "match": "none"}, "nutrients": n})
		}
		per100, perServing := map[string]float64{}, map[string]float64{}
		for k, v := range sum {
			per100[k], perServing[k] = v*100/yield, v/servings
		}
		rec := map[string]any{
			"$schema": "https://openfoodlog.org/schema/v0/recipe.json", "id": id, "name": in["name"], "lines": lines,
			"servings": servings, "yield_g": yield, "nutrients_per_100g": per100, "nutrients_per_serving": perServing,
		}
		s.recipes[id] = rec
		writeJSON(w, status, rec)
	}
}

func TestV02ChecksCatchTheirBehaviour(t *testing.T) {
	cases := []struct {
		name   string
		breaks func(*reference)
		fails  string
	}{
		{"a copy that ignores its key", func(s *reference) { s.copyIgnoresKey = true }, "copy"},
		{"a saved meal logged without its name", func(s *reference) { s.savedUntitled = true }, "saved-meal"},
		{"a recipe counted per 100 g", func(s *reference) { s.recipeMiscounts = true }, "recipe"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newReference()
			c.breaks(s)
			got := statuses(run(t, s))
			for _, name := range Checks {
				want := Pass
				if name == c.fails {
					want = Fail
				}
				if got[name] != want {
					t.Errorf("%s = %s, want %s", name, got[name], want)
				}
			}
			if len(s.saved) > 0 || len(s.recipes) > 0 {
				t.Errorf("left behind: %d saved meals, %d recipes", len(s.saved), len(s.recipes))
			}
		})
	}
}
