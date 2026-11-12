package openfoodlog

import (
	"encoding/json"
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

	suffix := map[string]string{"kcal": "_kcal", "kJ": "_kj", "g": "_g", "mg": "_mg", "µg": "_ug"}
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

// TestMeasureAgrees holds the package's Measure to the amount vectors, as the reference does.
func TestMeasureAgrees(t *testing.T) {
	vectors, err := LoadVectors()
	if err != nil {
		t.Fatal(err)
	}
	foods := map[string]Food{}
	for _, f := range vectors.Foods {
		foods[f.ID] = f
	}
	for _, c := range vectors.Cases {
		got, err := Measure(foods[c.Food], c.Amount, c.Share)
		if c.Expect.Error != "" {
			if err == nil || err.Error() != c.Expect.Error {
				t.Errorf("%s: got %v, want error %s", c.Name, err, c.Expect.Error)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if (got.Grams == nil) != (c.Expect.Grams == nil) || (got.Grams != nil && math.Abs(*got.Grams-*c.Expect.Grams) > 1e-6) {
			t.Errorf("%s: grams %v, want %v", c.Name, got.Grams, c.Expect.Grams)
		}
		if c.Expect.ML != nil && (got.ML == nil || math.Abs(*got.ML-*c.Expect.ML) > 1e-6) {
			t.Errorf("%s: ml %v, want %v", c.Name, got.ML, *c.Expect.ML)
		}
		for k, want := range c.Expect.Nutrients {
			if v, ok := got.Nutrients[k]; !ok || math.Abs(v-want) > 1e-6 {
				t.Errorf("%s: %s = %v, want %v", c.Name, k, v, want)
			}
		}
	}
}
