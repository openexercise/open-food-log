package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
)

// Logging again, saved meals and recipes (v0.2; spec section 8). Each check makes what it needs,
// with items that name no food so no catalogue is needed, and deletes it again; cleanUp removes
// whatever a failing check left.

// copyMeal: POST /v1/meals/{id}/copy logs the meal again as a new meal holding the same items,
// counted again, and a repeated Idempotency-Key returns that copy rather than a second one.
func (r *runner) copyMeal(ctx context.Context) error {
	key := "ofl-conformance-" + random()
	path := r.path + "/copy"
	res, err := r.send(ctx, http.MethodPost, path, map[string]any{}, header("Idempotency-Key", key), true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST "+path, http.StatusCreated)
	}
	m, err := r.written(res)
	if err != nil {
		return err
	}
	id, _ := m["id"].(string)
	if id == "" || id == r.id {
		return fmt.Errorf("the copy has id %q: want a new meal", id)
	}
	r.extraMeals = append(r.extraMeals, id)
	if err := sameItems(m, r.input); err != nil {
		return fmt.Errorf("the copy: %w", err)
	}
	res, err = r.send(ctx, http.MethodPost, path, map[string]any{}, header("Idempotency-Key", key), true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK || res.header.Get("Idempotent-Replayed") != "true" {
		return res.unexpected("POST "+path+" with a repeated Idempotency-Key", http.StatusOK)
	}
	again, err := r.written(res)
	if err != nil {
		return err
	}
	if again["id"] != id {
		return fmt.Errorf("a repeated Idempotency-Key copied the meal again (%v, want %s)", again["id"], id)
	}
	return nil
}

// savedMeal: POST /v1/saved-meals with from_meal_id keeps the meal as a valid OFL saved meal;
// logging it makes a meal titled with its name and holding its items; it's listed; and deleting
// it leaves a not_found.
func (r *runner) savedMeal(ctx context.Context) error {
	name := "ofl conformance saved " + r.run
	res, err := r.send(ctx, http.MethodPost, "/v1/saved-meals", map[string]any{"name": name, "from_meal_id": r.id}, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST /v1/saved-meals", http.StatusCreated)
	}
	saved, err := r.document(res, "saved-meal")
	if err != nil {
		return err
	}
	id, _ := saved["id"].(string)
	if id == "" {
		return errors.New("the saved meal has no id")
	}
	r.savedID = id
	if saved["name"] != name {
		return fmt.Errorf("the saved meal is named %v, want %q", saved["name"], name)
	}
	if err := sameItems(saved, r.input); err != nil {
		return fmt.Errorf("the saved meal: %w", err)
	}
	path := "/v1/saved-meals/" + url.PathEscape(id)

	res, err = r.send(ctx, http.MethodPost, path+"/log", map[string]any{}, header("Idempotency-Key", "ofl-conformance-"+random()), true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST "+path+"/log", http.StatusCreated)
	}
	m, err := r.written(res)
	if err != nil {
		return err
	}
	if logged, _ := m["id"].(string); logged != "" {
		r.extraMeals = append(r.extraMeals, logged)
	}
	if m["title"] != name {
		return fmt.Errorf("the meal logged from a saved meal has title %v, want its name %q", m["title"], name)
	}
	if err := sameItems(m, r.input); err != nil {
		return fmt.Errorf("the meal logged from a saved meal: %w", err)
	}

	res, err = r.send(ctx, http.MethodGet, "/v1/saved-meals", nil, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK {
		return res.unexpected("GET /v1/saved-meals", http.StatusOK)
	}
	var list struct {
		SavedMeals []json.RawMessage `json:"saved_meals"`
	}
	if err := json.Unmarshal(res.body, &list); err != nil {
		return fmt.Errorf("GET /v1/saved-meals: no saved_meals list: %v", err)
	}
	if !listed(list.SavedMeals, id, r.v, "saved-meal") {
		return fmt.Errorf("GET /v1/saved-meals doesn't list %s as a valid saved meal", id)
	}
	return r.remove(ctx, path, &r.savedID)
}

// Recipe numbers: one line that names no food, so no catalogue is needed. Its 300 kcal and 30 g
// of protein over two servings make a serving 150 kcal and 15 g; over three, 100 and 10.
const (
	recipeKcal    = 300.0
	recipeProtein = 30.0
)

func (r *runner) recipeInput(servings float64) map[string]any {
	return map[string]any{
		"name":     "ofl conformance recipe " + r.run,
		"servings": servings,
		"yield_g":  300,
		"lines": []any{map[string]any{
			"text": "ofl conformance stock", "nutrients": map[string]any{"energy_kcal": recipeKcal, "protein_g": recipeProtein},
		}},
	}
}

// recipe: POST /v1/recipes makes a valid OFL recipe whose nutrients follow from its lines; a meal
// item names it by its id and counts one serving; PUT replaces it keeping its id; DELETE leaves a
// not_found.
func (r *runner) recipe(ctx context.Context) error {
	res, err := r.send(ctx, http.MethodPost, "/v1/recipes", r.recipeInput(2), nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST /v1/recipes", http.StatusCreated)
	}
	rec, err := r.document(res, "recipe")
	if err != nil {
		return err
	}
	id, _ := rec["id"].(string)
	if id == "" {
		return errors.New("the recipe has no id")
	}
	r.recipeID = id
	if err := perServing(rec, recipeKcal/2, recipeProtein/2); err != nil {
		return err
	}

	meal := map[string]any{
		"eaten_at":  r.input["eaten_at"],
		"meal_type": "snack",
		"items": []any{map[string]any{
			"text": "ofl conformance recipe", "food_id": id, "amount": map[string]any{"value": 1, "unit": "serving"},
		}},
	}
	res, err = r.send(ctx, http.MethodPost, "/v1/meals", meal, header("Idempotency-Key", "ofl-conformance-"+random()), true)
	if err != nil {
		return err
	}
	if res.status != http.StatusCreated {
		return res.unexpected("POST /v1/meals naming the recipe", http.StatusCreated)
	}
	m, err := r.written(res)
	if err != nil {
		return err
	}
	if logged, _ := m["id"].(string); logged != "" {
		r.extraMeals = append(r.extraMeals, logged)
	}
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		return fmt.Errorf("the meal naming the recipe has %d items, want 1", len(items))
	}
	item, _ := items[0].(map[string]any)
	food, _ := item["food"].(map[string]any)
	if food["id"] != id {
		return fmt.Errorf("the item's food is %v, want the recipe %s", food["id"], id)
	}
	if got := number(item["nutrients"], "energy_kcal"); !near(got, recipeKcal/2) {
		return fmt.Errorf("one serving of the recipe counted %v kcal, want %v", got, recipeKcal/2)
	}

	path := "/v1/recipes/" + url.PathEscape(id)
	res, err = r.send(ctx, http.MethodPut, path, r.recipeInput(3), nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK {
		return res.unexpected("PUT "+path, http.StatusOK)
	}
	rec, err = r.document(res, "recipe")
	if err != nil {
		return err
	}
	if rec["id"] != id {
		return fmt.Errorf("PUT changed the recipe's id from %s to %v", id, rec["id"])
	}
	if err := perServing(rec, recipeKcal/3, recipeProtein/3); err != nil {
		return fmt.Errorf("after PUT: %w", err)
	}
	return r.remove(ctx, path, &r.recipeID)
}

// remove deletes a recipe or saved meal and checks reading it is then not_found.
func (r *runner) remove(ctx context.Context, path string, id *string) error {
	res, err := r.send(ctx, http.MethodDelete, path, nil, nil, true)
	if err != nil {
		return err
	}
	if res.status != http.StatusOK && res.status != http.StatusNoContent {
		return res.unexpected("DELETE "+path, http.StatusNoContent)
	}
	*id = ""
	res, err = r.send(ctx, http.MethodGet, path, nil, nil, true)
	if err != nil {
		return err
	}
	return res.problem("GET "+path+" after DELETE", http.StatusNotFound, "not_found")
}

// cleanUpAgain removes what the v0.2 checks made, when one failed part way.
func (r *runner) cleanUpAgain(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)
	for _, id := range r.extraMeals {
		_, _ = r.send(ctx, http.MethodDelete, "/v1/meals/"+url.PathEscape(id), nil, nil, true)
	}
	if r.savedID != "" {
		_, _ = r.send(ctx, http.MethodDelete, "/v1/saved-meals/"+url.PathEscape(r.savedID), nil, nil, true)
	}
	if r.recipeID != "" {
		_, _ = r.send(ctx, http.MethodDelete, "/v1/recipes/"+url.PathEscape(r.recipeID), nil, nil, true)
	}
}

// document reads a result that is itself an OFL document of a kind (a recipe, a saved meal).
func (r *runner) document(res *response, kind string) (map[string]any, error) {
	if err := res.isJSON(); err != nil {
		return nil, err
	}
	if p := r.v.ValidateAs(kind, res.body); len(p) > 0 {
		return nil, fmt.Errorf("the result isn't a valid OFL %s: %s", kind, p[0])
	}
	var doc map[string]any
	return doc, json.Unmarshal(res.body, &doc)
}

// sameItems checks a meal or saved meal holds the input's items: their words and nutrients, in order.
func sameItems(doc, in map[string]any) error {
	got, _ := doc["items"].([]any)
	want, _ := in["items"].([]any)
	if len(got) != len(want) {
		return fmt.Errorf("%d items, want %d", len(got), len(want))
	}
	for i := range want {
		g, _ := got[i].(map[string]any)
		w, _ := want[i].(map[string]any)
		if g["text"] != w["text"] {
			return fmt.Errorf("item %d is %v, want %v", i, g["text"], w["text"])
		}
		wn, _ := w["nutrients"].(map[string]any)
		for k := range wn {
			if gv, wv := number(g["nutrients"], k), number(wn, k); !near(gv, wv) {
				return fmt.Errorf("item %d's %s is %v, want %v", i, k, gv, wv)
			}
		}
	}
	return nil
}

// perServing checks a recipe's nutrients_per_serving.
func perServing(rec map[string]any, kcal, protein float64) error {
	if got := number(rec["nutrients_per_serving"], "energy_kcal"); !near(got, kcal) {
		return fmt.Errorf("the recipe's energy a serving is %v, want %v", got, kcal)
	}
	if got := number(rec["nutrients_per_serving"], "protein_g"); !near(got, protein) {
		return fmt.Errorf("the recipe's protein a serving is %v, want %v", got, protein)
	}
	return nil
}

// listed says a list holds a valid document of the kind with this id.
func listed(docs []json.RawMessage, id string, v interface {
	ValidateAs(string, []byte) []string
}, kind string) bool {
	for _, raw := range docs {
		var d struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(raw, &d) == nil && d.ID == id {
			return len(v.ValidateAs(kind, raw)) == 0
		}
	}
	return false
}

// number reads a nutrient from a decoded document, or from the script's own input, whose numbers
// may be Go ints; a missing one is NaN, which is near nothing.
func number(m any, key string) float64 {
	obj, _ := m.(map[string]any)
	switch v := obj[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return math.NaN()
}
