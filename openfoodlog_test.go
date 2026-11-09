package openfoodlog

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func readJSON(t *testing.T, name string, v any) []byte {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return raw
}

func validator(t *testing.T) *Validator {
	t.Helper()
	v, err := NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestExamplesValidate(t *testing.T) {
	v := validator(t)
	paths, _ := filepath.Glob("examples/*.json")
	if len(paths) == 0 {
		t.Fatal("no examples")
	}
	for _, p := range paths {
		var doc any
		raw := readJSON(t, p, &doc)
		if problems := v.Validate(raw); problems != nil {
			t.Errorf("%s: %v", p, problems)
		}
	}
	var vectors struct{ Foods []json.RawMessage }
	readJSON(t, "schema/v0/amount-vectors.json", &vectors)
	for i, f := range vectors.Foods {
		if problems := v.ValidateAs("food", f); problems != nil {
			t.Errorf("vector food %d: %v", i, problems)
		}
	}
}

func TestSchemasRefuse(t *testing.T) {
	item := func(fields string) string {
		return `{"date": "2026-11-09", "source": {"kind": "app"}, "items": [{"text": "x"` + fields + `}]}`
	}
	cases := map[string]string{
		"no time or day":            `{"source": {"kind": "app"}, "items": []}`,
		"a portion unit unnamed":    item(`, "amount": {"value": 1, "unit": "portion"}`),
		"a portion on grams":        item(`, "amount": {"value": 1, "unit": "g", "portion": "large"}`),
		"a cup as a unit":           item(`, "amount": {"value": 1, "unit": "cup"}`),
		"a nutrient without a unit": item(`, "nutrients": {"protein": 20}`),
		"a negative nutrient":       item(`, "nutrients": {"protein_g": -1}`),
		"no amount of nothing":      item(`, "amount": {"value": 0, "unit": "g"}`),
		"an unknown match":          item(`, "food": {"id": null, "name": "x", "match": "guess"}`),
	}
	v := validator(t)
	for name, doc := range cases {
		if v.ValidateAs("meal", []byte(doc)) == nil {
			t.Errorf("%s: validated", name)
		}
	}
	if v.Validate([]byte(`{"items": []}`)) == nil {
		t.Error("a document without $schema validated")
	}
}

func TestNutrientVocabularyMatchesTheSchema(t *testing.T) {
	var vocab struct {
		Nutrients []struct{ Key, Unit string }
	}
	readJSON(t, "schema/v0/nutrients.json", &vocab)
	var food struct {
		Defs struct {
			Nutrients struct{ Properties map[string]any }
		} `json:"$defs"`
	}
	readJSON(t, "schema/v0/food.json", &food)

	suffix := map[string]string{"kcal": "_kcal", "g": "_g", "mg": "_mg", "µg": "_ug"}
	var keys []string
	for _, n := range vocab.Nutrients {
		if slices.Contains(keys, n.Key) {
			t.Errorf("%s is listed twice", n.Key)
		}
		keys = append(keys, n.Key)
		if s, ok := suffix[n.Unit]; !ok || !strings.HasSuffix(n.Key, s) {
			t.Errorf("%s: its key doesn't end in its unit, %s", n.Key, n.Unit)
		}
		if _, ok := food.Defs.Nutrients.Properties[n.Key]; !ok {
			t.Errorf("%s is in nutrients.json but not food.json", n.Key)
		}
	}
	for k := range food.Defs.Nutrients.Properties {
		if !slices.Contains(keys, k) {
			t.Errorf("%s is in food.json but not nutrients.json", k)
		}
	}
}

func near(a, b, tolerance float64) bool { return math.Abs(a-b) <= tolerance+1e-9 }

func TestAmountVectors(t *testing.T) {
	vectors, err := LoadVectors()
	if err != nil {
		t.Fatal(err)
	}
	foods := map[string]Food{}
	for _, f := range vectors.Foods {
		foods[f.ID] = f
	}
	for _, c := range vectors.Cases {
		f, ok := foods[c.Food]
		if !ok {
			t.Errorf("%s: no food %s", c.Name, c.Food)
			continue
		}
		g, err := Grams(f, c.Amount)
		if c.Expect.Error != "" {
			if err == nil || err.Error() != c.Expect.Error {
				t.Errorf("%s: got %v, want error %s", c.Name, err, c.Expect.Error)
			}
			continue
		}
		if err != nil || c.Expect.Grams == nil || !near(g, *c.Expect.Grams, 1e-6) {
			t.Errorf("%s: got %v g (%v), want %v", c.Name, g, err, c.Expect.Grams)
			continue
		}
		got := Nutrients(f, g)
		for k, want := range c.Expect.Nutrients {
			if !near(got[k], want, 1e-6) {
				t.Errorf("%s: %s = %v, want %v", c.Name, k, got[k], want)
			}
		}
	}
}

// TestExamplesAddUp checks every example item against the food it names: its grams and nutrients
// follow from its amount, it names no nutrient the food lacks, and every estimate points at a value.
func TestExamplesAddUp(t *testing.T) {
	vectors, err := LoadVectors()
	if err != nil {
		t.Fatal(err)
	}
	foods := map[string]Food{}
	for _, f := range vectors.Foods {
		foods[f.ID] = f
	}
	type meal struct {
		Items []struct {
			Text      string
			Food      *struct{ ID *string }
			Amount    *Amount
			Grams     *float64
			Nutrients map[string]float64
		}
		Estimates []string
	}
	var log struct{ Meals []json.RawMessage }
	readJSON(t, "examples/log.json", &log)
	var breakfast json.RawMessage
	readJSON(t, "examples/breakfast.json", &breakfast)

	for _, raw := range append(log.Meals, breakfast) {
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
		for _, it := range m.Items {
			if it.Food == nil || it.Food.ID == nil || it.Amount == nil {
				continue
			}
			f, ok := foods[*it.Food.ID]
			if !ok {
				t.Errorf("%q: no food %s in the vectors", it.Text, *it.Food.ID)
				continue
			}
			g, err := Grams(f, *it.Amount)
			if err != nil || it.Grams == nil || !near(*it.Grams, g, 0.01) {
				t.Errorf("%q: grams %v, want %v (%v)", it.Text, it.Grams, g, err)
			}
			want := Nutrients(f, g)
			for k, v := range it.Nutrients {
				if w, ok := want[k]; !ok {
					t.Errorf("%q: %s isn't known for %s", it.Text, k, f.ID)
				} else if !near(v, w, 0.01) {
					t.Errorf("%q: %s = %v, want %v", it.Text, k, v, w)
				}
			}
			if len(it.Nutrients) != len(want) {
				t.Errorf("%q: %d nutrients, the food has %d", it.Text, len(it.Nutrients), len(want))
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
