// The reference calculation of sections 6, 8 and 9, kept in step with the source of truth in
// mightydotrun/lifterclub (internal/ofl/calc_test.go), and held to the vectors and the examples.

package openfoodlog

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"
	"time"
)

type portion struct {
	ID    string
	Grams float64
}

type serving struct {
	Text  string
	Grams float64
	ML    float64
}

type label struct {
	Serving             *serving
	NutrientsPerServing map[string]float64 `json:"nutrients_per_serving"`
}

type food struct {
	ID                string
	NutrientsPer100g  map[string]float64 `json:"nutrients_per_100g"`
	NutrientsPer100ml map[string]float64 `json:"nutrients_per_100ml"`
	DensityGPerML     float64            `json:"density_g_per_ml"`
	Portions          []portion
	Label             *label
}

type amount struct {
	Value   float64
	Unit    string
	Portion string
}

type line struct {
	Text      string
	Food      *struct{ ID *string }
	Amount    *amount
	Share     float64
	Grams     *float64
	Nutrients map[string]float64
}

type recipe struct {
	ID                 string
	Lines              []line
	Servings           float64
	YieldG             float64            `json:"yield_g"`
	CookingYieldFactor float64            `json:"cooking_yield_factor"`
	NutrientsPer100g   map[string]float64 `json:"nutrients_per_100g"`
	NutrientsPerServ   map[string]float64 `json:"nutrients_per_serving"`
}

// measured is an amount worked out: its weight and volume where they can be known, and what it held.
type measured struct {
	grams, ml *float64
	nutrients map[string]float64
}

func ptr(v float64) *float64 { return &v }

func scale(m map[string]float64, by float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v * by
	}
	return out
}

// measure is the reference calculation of section 6 of the spec: an amount (times its share) on a
// food becomes grams, millilitres or servings, then nutrients from the first of the food's sets
// that the amount reaches.
func measure(f food, a amount, share float64) (measured, error) {
	if share == 0 {
		share = 1
	}
	v := a.Value * share
	var m measured
	var servings *float64
	switch {
	case a.Unit == "portion":
		i := slices.IndexFunc(f.Portions, func(p portion) bool { return p.ID == a.Portion })
		if i < 0 {
			return m, errors.New("portion_unknown")
		}
		m.grams = ptr(v * f.Portions[i].Grams)
	case gramsPer[a.Unit] > 0:
		m.grams = ptr(v * gramsPer[a.Unit])
	case mlPer[a.Unit] > 0:
		m.ml = ptr(v * mlPer[a.Unit])
	case a.Unit == "serving":
		if f.Label == nil || f.Label.Serving == nil {
			return m, errors.New("serving_unknown")
		}
		servings = ptr(v)
		if s := f.Label.Serving; s.Grams > 0 {
			m.grams = ptr(v * s.Grams)
		}
		if s := f.Label.Serving; s.ML > 0 {
			m.ml = ptr(v * s.ML)
		}
	default:
		return m, fmt.Errorf("unit %s", a.Unit)
	}
	if m.grams == nil && m.ml != nil && f.DensityGPerML > 0 {
		m.grams = ptr(*m.ml * f.DensityGPerML)
	}
	var perServing map[string]float64
	if f.Label != nil {
		perServing = f.Label.NutrientsPerServing
	}
	switch {
	case m.grams != nil && len(f.NutrientsPer100g) > 0:
		m.nutrients = scale(f.NutrientsPer100g, *m.grams/100)
	case m.ml != nil && len(f.NutrientsPer100ml) > 0:
		m.nutrients = scale(f.NutrientsPer100ml, *m.ml/100)
	case servings != nil && len(perServing) > 0:
		m.nutrients = scale(perServing, *servings)
	case len(f.NutrientsPer100ml) > 0 || len(perServing) == 0:
		return m, errors.New("density_unknown")
	default:
		return m, errors.New("serving_weight_unknown")
	}
	return m, nil
}

// recipeTotals sums a recipe's lines' nutrients, leaving out any nutrient one line lacks, and
// works out its yield (section 8).
func recipeTotals(r recipe) (totals map[string]float64, yield float64) {
	totals = maps.Clone(r.Lines[0].Nutrients)
	raw := 0.0
	for _, l := range r.Lines {
		for k := range totals {
			if _, ok := l.Nutrients[k]; !ok {
				delete(totals, k)
			}
		}
		if l.Grams != nil {
			raw += *l.Grams
		}
	}
	for k := range totals {
		totals[k] = 0
		for _, l := range r.Lines {
			totals[k] += l.Nutrients[k]
		}
	}
	switch {
	case r.YieldG > 0:
		yield = r.YieldG
	case r.CookingYieldFactor > 0:
		yield = raw * r.CookingYieldFactor
	default:
		yield = raw
	}
	return totals, yield
}

// measureRecipe is a meal item's amount on a recipe: by weight, or by the serving (yield / servings).
func measureRecipe(r recipe, a amount, share float64) (measured, error) {
	if share == 0 {
		share = 1
	}
	totals, yield := recipeTotals(r)
	var g float64
	switch {
	case gramsPer[a.Unit] > 0:
		g = a.Value * share * gramsPer[a.Unit]
	case a.Unit == "serving":
		g = a.Value * share * yield / r.Servings
	case mlPer[a.Unit] > 0:
		return measured{}, errors.New("density_unknown")
	default:
		return measured{}, errors.New("portion_unknown")
	}
	return measured{grams: &g, nutrients: scale(totals, g/yield)}, nil
}

func near(a, b, tolerance float64) bool { return math.Abs(a-b) <= tolerance+1e-9 }

// readDoc reads one of this repository's JSON documents.
func readDoc(t *testing.T, name string, v any) {
	t.Helper()
	readJSON(t, name, v)
}

func readFoods(t *testing.T) map[string]food {
	t.Helper()
	var vectors struct{ Foods []food }
	readDoc(t, "schema/v0/amount-vectors.json", &vectors)
	foods := map[string]food{}
	for _, f := range vectors.Foods {
		foods[f.ID] = f
	}
	return foods
}

func checkMeasured(t *testing.T, name string, got measured, wantGrams, wantML *float64, isGramsKnown bool, want map[string]float64) {
	t.Helper()
	switch {
	case !isGramsKnown && got.grams != nil:
		t.Errorf("%s: got %v g, want grams unknown", name, *got.grams)
	case isGramsKnown && (got.grams == nil || wantGrams == nil || !near(*got.grams, *wantGrams, 1e-6)):
		t.Errorf("%s: got grams %v, want %v", name, got.grams, wantGrams)
	}
	if wantML != nil && (got.ml == nil || !near(*got.ml, *wantML, 1e-6)) {
		t.Errorf("%s: got ml %v, want %v", name, got.ml, *wantML)
	}
	for k, w := range want {
		if v, ok := got.nutrients[k]; !ok || !near(v, w, 1e-6) {
			t.Errorf("%s: %s = %v, want %v", name, k, v, w)
		}
	}
}

// expectation reads a vector's expect, telling "grams": null (can't be weighed) from no grams given.
type expectation struct {
	Grams     *float64
	ML        *float64 `json:"ml"`
	Nutrients map[string]float64
	Error     string
	HasGrams  bool `json:"-"`
}

func (e *expectation) UnmarshalJSON(b []byte) error {
	type plain expectation
	if err := json.Unmarshal(b, (*plain)(e)); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(b, &fields); err != nil {
		return err
	}
	_, e.HasGrams = fields["grams"]
	return nil
}

func TestAmountVectors(t *testing.T) {
	var vectors struct {
		Cases []struct {
			Name   string
			Food   string
			Amount amount
			Share  float64
			Expect expectation
		}
	}
	readDoc(t, "schema/v0/amount-vectors.json", &vectors)
	foods := readFoods(t)
	for _, c := range vectors.Cases {
		f, ok := foods[c.Food]
		if !ok {
			t.Errorf("%s: no food %s", c.Name, c.Food)
			continue
		}
		got, err := measure(f, c.Amount, c.Share)
		if c.Expect.Error != "" {
			if err == nil || err.Error() != c.Expect.Error {
				t.Errorf("%s: got %v, want error %s", c.Name, err, c.Expect.Error)
			}
			continue
		}
		if err != nil || !c.Expect.HasGrams {
			t.Errorf("%s: %v (grams expected: %v)", c.Name, err, c.Expect.HasGrams)
			continue
		}
		checkMeasured(t, c.Name, got, c.Expect.Grams, c.Expect.ML, c.Expect.Grams != nil, c.Expect.Nutrients)
	}
}

// checkLines holds a recipe's or a meal's items to the foods they name: grams and nutrients follow
// from the amount (within tolerance), and no item claims a nutrient its food lacks.
func checkLines(t *testing.T, where string, lines []line, foods map[string]food, recipes map[string]recipe, tolerance float64) {
	t.Helper()
	for _, it := range lines {
		if it.Food == nil || it.Food.ID == nil || it.Amount == nil {
			continue
		}
		name := fmt.Sprintf("%s %q", where, it.Text)
		var got measured
		var err error
		if r, ok := recipes[*it.Food.ID]; ok {
			got, err = measureRecipe(r, *it.Amount, it.Share)
		} else if f, ok := foods[*it.Food.ID]; ok {
			got, err = measure(f, *it.Amount, it.Share)
		} else {
			t.Errorf("%s: no food or recipe %s", name, *it.Food.ID)
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		switch {
		case got.grams == nil && it.Grams != nil:
			t.Errorf("%s: grams %v, but the amount can't be weighed", name, *it.Grams)
		case got.grams != nil && (it.Grams == nil || !near(*it.Grams, *got.grams, tolerance)):
			t.Errorf("%s: grams %v, want %v", name, it.Grams, *got.grams)
		}
		for k, v := range it.Nutrients {
			want, ok := got.nutrients[k]
			if !ok {
				t.Errorf("%s: %s isn't known for %s", name, k, *it.Food.ID)
			} else if !near(v, want, tolerance) {
				t.Errorf("%s: %s = %v, want %v", name, k, v, want)
			}
		}
		if len(it.Nutrients) != len(got.nutrients) {
			t.Errorf("%s: %d nutrients, the food has %d", name, len(it.Nutrients), len(got.nutrients))
		}
	}
}

func readRecipes(t *testing.T) []recipe {
	t.Helper()
	var vectors struct{ Recipes []recipe }
	readDoc(t, "schema/v0/recipe-vectors.json", &vectors)
	return vectors.Recipes
}

func TestRecipeVectors(t *testing.T) {
	var vectors struct {
		Cases []struct {
			Name   string
			Recipe string
			Amount *amount
			Expect struct {
				YieldG              float64            `json:"yield_g"`
				NutrientsPer100g    map[string]float64 `json:"nutrients_per_100g"`
				NutrientsPerServing map[string]float64 `json:"nutrients_per_serving"`
				Absent              []string
				Grams               float64
				Nutrients           map[string]float64
			}
		}
	}
	readDoc(t, "schema/v0/recipe-vectors.json", &vectors)
	foods := readFoods(t)
	recipes := map[string]recipe{}
	for _, r := range readRecipes(t) {
		recipes[r.ID] = r
		checkLines(t, r.ID, r.Lines, foods, nil, 1e-6)
	}
	for _, c := range vectors.Cases {
		r, ok := recipes[c.Recipe]
		if !ok {
			t.Errorf("%s: no recipe %s", c.Name, c.Recipe)
			continue
		}
		if c.Amount != nil {
			got, err := measureRecipe(r, *c.Amount, 1)
			if err != nil {
				t.Errorf("%s: %v", c.Name, err)
				continue
			}
			checkMeasured(t, c.Name, got, &c.Expect.Grams, nil, true, c.Expect.Nutrients)
			continue
		}
		totals, yield := recipeTotals(r)
		if !near(yield, c.Expect.YieldG, 1e-6) {
			t.Errorf("%s: yield %v, want %v", c.Name, yield, c.Expect.YieldG)
		}
		for k, want := range c.Expect.NutrientsPer100g {
			if got := totals[k] * 100 / yield; !near(got, want, 1e-6) {
				t.Errorf("%s: %s per 100 g = %v, want %v", c.Name, k, got, want)
			}
		}
		for k, want := range c.Expect.NutrientsPerServing {
			if got := totals[k] / r.Servings; !near(got, want, 1e-6) {
				t.Errorf("%s: %s per serving = %v, want %v", c.Name, k, got, want)
			}
		}
		for _, k := range c.Expect.Absent {
			if _, ok := totals[k]; ok {
				t.Errorf("%s: %s should be absent", c.Name, k)
			}
		}
	}
}

type target struct {
	Nutrient         string
	Per              string
	MealType         string   `json:"meal_type"`
	Min              *float64 `json:"min"`
	Max              *float64 `json:"max"`
	Target           *float64 `json:"target"`
	TolerancePercent *float64 `json:"tolerance_percent"`
	Weekdays         []int
	EffectiveFrom    string `json:"effective_from"`
	EffectiveTo      string `json:"effective_to"`
}

// applying is the reference choice of section 9: of the entries for this nutrient, per and meal
// type that cover the day, the more specific (weekdays, then meal_type), then the latest
// effective_from, then the later in the list. -1 when none applies.
func applying(targets []target, day, nutrient, per, mealType string) int {
	d, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return -1
	}
	weekday := int(d.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	best := -1
	rank := func(t target) string {
		return fmt.Sprintf("%t %t %s", len(t.Weekdays) > 0, t.MealType != "", t.EffectiveFrom)
	}
	for i, t := range targets {
		if t.Nutrient != nutrient || t.Per != per || t.EffectiveFrom > day || (t.EffectiveTo != "" && t.EffectiveTo < day) {
			continue
		}
		if (per == "meal" && t.MealType != "" && t.MealType != mealType) || (len(t.Weekdays) > 0 && !slices.Contains(t.Weekdays, weekday)) {
			continue
		}
		if best < 0 || rank(t) >= rank(targets[best]) {
			best = i
		}
	}
	return best
}

// status is the reference judgement of section 9. A total that isn't complete is a lower bound:
// it can show a minimum met or a maximum passed, never the reverse.
func status(t target, total float64, isComplete bool) string {
	low, high := t.Min, t.Max
	if low == nil && high == nil {
		tolerance := 10.0
		if t.TolerancePercent != nil {
			tolerance = *t.TolerancePercent
		}
		low, high = ptr(*t.Target*(1-tolerance/100)), ptr(*t.Target*(1+tolerance/100))
	}
	switch {
	case high != nil && total > *high:
		return "above"
	case low != nil && total < *low && isComplete:
		return "under"
	case low != nil && total < *low, high != nil && !isComplete:
		return "unknown"
	}
	return "met"
}

type targetCase struct {
	Name       string
	Date       string
	Nutrient   string
	Per        string
	MealType   string `json:"meal_type"`
	Total      float64
	IsComplete bool `json:"is_complete"`
	Expect     struct {
		Target *int
		Status *string
	}
}

func TestTargetVectors(t *testing.T) {
	var vectors struct {
		Targets struct{ Targets []target }
		Cases   []targetCase
	}
	readDoc(t, "schema/v0/target-vectors.json", &vectors)
	targets := vectors.Targets.Targets
	for _, c := range vectors.Cases {
		i := applying(targets, c.Date, c.Nutrient, c.Per, c.MealType)
		if c.Expect.Target == nil {
			if i >= 0 {
				t.Errorf("%s: target %d applies, want none", c.Name, i)
			}
			if c.Expect.Status != nil {
				t.Errorf("%s: a status with no target", c.Name)
			}
			continue
		}
		if i != *c.Expect.Target {
			t.Errorf("%s: target %d applies, want %d", c.Name, i, *c.Expect.Target)
			continue
		}
		if got := status(targets[i], c.Total, c.IsComplete); c.Expect.Status == nil || got != *c.Expect.Status {
			t.Errorf("%s: %s, want %s", c.Name, got, deref(c.Expect.Status))
		}
	}
	for _, want := range []string{"met", "under", "above", "unknown"} {
		if !slices.ContainsFunc(vectors.Cases, func(c targetCase) bool {
			return c.Expect.Status != nil && *c.Expect.Status == want
		}) {
			t.Errorf("no case is %s", want)
		}
	}
}

// TestExamplesAddUp checks every example item and recipe line against the food or recipe it names:
// its grams and nutrients follow from its amount, it names no nutrient the food lacks, every
// estimate points at a value, and a recipe's stated nutrients follow from its lines.
func TestExamplesAddUp(t *testing.T) {
	foods := readFoods(t)
	recipes := map[string]recipe{}
	var example recipe
	readDoc(t, "examples/overnight-oats.json", &example)
	recipes[example.ID] = example
	checkLines(t, example.ID, example.Lines, foods, nil, 0.01)
	totals, yield := recipeTotals(example)
	for k, v := range example.NutrientsPer100g {
		if !near(v, totals[k]*100/yield, 0.01) {
			t.Errorf("%s: %s per 100 g = %v, want %v", example.ID, k, v, totals[k]*100/yield)
		}
	}
	for k, v := range example.NutrientsPerServ {
		if !near(v, totals[k]/example.Servings, 0.01) {
			t.Errorf("%s: %s per serving = %v, want %v", example.ID, k, v, totals[k]/example.Servings)
		}
	}
	if len(example.NutrientsPer100g) != len(totals) || len(example.NutrientsPerServ) != len(totals) {
		t.Errorf("%s: states %d and %d nutrients, its lines all have %d", example.ID, len(example.NutrientsPer100g), len(example.NutrientsPerServ), len(totals))
	}

	type meal struct {
		Items     []line
		Estimates []string
	}
	var log struct {
		Meals   []json.RawMessage
		Recipes []json.RawMessage
	}
	readDoc(t, "examples/log.json", &log)
	var breakfast, dinner json.RawMessage
	readDoc(t, "examples/breakfast.json", &breakfast)
	readDoc(t, "examples/dinner.json", &dinner)

	for _, raw := range append(log.Meals, breakfast, dinner) {
		var m meal
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		var doc any
		_ = json.Unmarshal(raw, &doc)
		for _, p := range m.Estimates {
			if !pointsAtValue(doc, p) {
				t.Errorf("estimate %s points at nothing", p)
			}
		}
		checkLines(t, "meal", m.Items, foods, recipes, 0.01)
	}
	for _, raw := range log.Recipes {
		if string(raw) == "" || !strings.Contains(string(raw), example.ID) {
			t.Errorf("log.json's recipes should hold %s, which its dinner names", example.ID)
		}
	}
}

// TestLabelsAgree holds a food that declares both a per-serving label with a weight and per-100 g
// values to the same numbers either way.
func TestLabelsAgree(t *testing.T) {
	foods := readFoods(t)
	for _, name := range []string{"examples/granola-bar.json", "examples/vitamin-d3-softgel.json"} {
		var f food
		readDoc(t, name, &f)
		foods[name] = f
	}
	for id, f := range foods {
		if f.Label == nil || f.Label.Serving == nil || f.Label.Serving.Grams == 0 || len(f.NutrientsPer100g) == 0 {
			continue
		}
		for k, v := range f.Label.NutrientsPerServing {
			if want := v * 100 / f.Label.Serving.Grams; !near(f.NutrientsPer100g[k], want, 0.01) {
				t.Errorf("%s: %s per 100 g = %v, its label gives %v", id, k, f.NutrientsPer100g[k], want)
			}
		}
	}
}

// pointsAtValue reports whether the JSON pointer p (RFC 6901, without escapes) resolves in doc.
func pointsAtValue(doc any, p string) bool {
	for _, part := range strings.Split(p, "/")[1:] {
		switch v := doc.(type) {
		case map[string]any:
			next, ok := v[part]
			if !ok {
				return false
			}
			doc = next
		case []any:
			var i int
			if _, err := fmt.Sscan(part, &i); err != nil || i < 0 || i >= len(v) {
				return false
			}
			doc = v[i]
		default:
			return false
		}
	}
	return true
}

func deref(s *string) string {
	if s == nil {
		return "none"
	}
	return *s
}
