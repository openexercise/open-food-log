package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFamilyOf(t *testing.T) {
	cases := map[string]string{
		"Meat loaf made with beef":                      "meat loaf",
		"Meat loaf dinner, NFS, frozen meal":            "meat loaf",
		"Meat loaf, NS as to type of meat":              "meat loaf",
		"Meat loaf made with chicken or turkey":         "meat loaf",
		"Egg omelet or scrambled egg, NS as to fat":     "egg omelet",
		"Egg, whole, raw":                               "egg",
		"Macaroni and cheese, from boxed mix":           "macaroni and",
		"Beef, chuck, eye roast, boneless, raw":         "beef",
		"Tomatoes as ingredient in omelet":              "tomatoes",
		"Chicken with dumplings":                        "chicken",
		"Orange juice, 100%, freshly squeezed":          "orange juice",
		"Peanut butter and jelly sandwich, regular, on": "peanut butter",
	}
	for name, want := range cases {
		if got := familyOf(name); got != want {
			t.Errorf("familyOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func foodsNamed(names ...string) []batchFood {
	var out []batchFood
	for i, n := range names {
		out = append(out, batchFood{ID: "ofc:" + strings.Repeat("x", i+1), Name: n})
	}
	return out
}

func TestBatchesKeepFamiliesWhole(t *testing.T) {
	foods := foodsNamed(
		"Apple, raw", "Apple, baked",
		"Meat loaf made with beef", "Meat loaf made with ham", "Meat loaf, NS as to type of meat",
		"Banana, raw",
	)
	batches := packBatches(groupFamilies(foods, 4), 4)
	if len(batches) != 2 {
		t.Fatalf("got %d batches, want 2: %+v", len(batches), batches)
	}
	for _, b := range batches {
		for _, f := range b {
			if f.Family == "meat loaf" && len(f.Foods) != 3 {
				t.Errorf("meat loaf split: %+v", f)
			}
		}
	}
}

func TestBigFamiliesSplitByTheirSecondPart(t *testing.T) {
	foods := foodsNamed("Beef, chuck, raw", "Beef, chuck, cooked", "Beef, brisket, raw", "Beef, brisket, cooked", "Beef, ground")
	families := groupFamilies(foods, 2)
	var keys []string
	for _, f := range families {
		keys = append(keys, f.Family)
		if len(f.Foods) > 2 {
			t.Errorf("%s has %d foods", f.Family, len(f.Foods))
		}
	}
	if !slices.Equal(keys, []string{"beef chuck", "beef brisket", "beef ground"}) {
		t.Errorf("families %q", keys)
	}
}

func TestToBatchFoodLeavesOutAModelsAliases(t *testing.T) {
	var r catalogueRecord
	json.Unmarshal([]byte(`{"id":"ofc:apple-raw","name":"Apple, raw","category":{"name":"Fruits"},
		"portions":[{"is_default":true}],"aliases":[{"name":"raw apple"},{"name":"apple"}],
		"provenance":{"generated":["/aliases/1"]}}`), &r)
	f := toBatchFood(r)
	if !f.HasDefaultPortion || f.Category != "Fruits" || !slices.Equal(f.Aliases, []string{"raw apple"}) {
		t.Errorf("got %+v", f)
	}
}

var loaves = batchFile{Batch: "batch-001", Families: []family{{Family: "meat loaf", Foods: []batchFood{
	{ID: "ofc:meat-loaf-made-with-beef", Name: "Meat loaf made with beef"},
	{ID: "ofc:meat-loaf-made-with-ham", Name: "Meat loaf made with ham"},
}}}}

func TestCheckAnswer(t *testing.T) {
	for _, c := range []struct {
		name    string
		answer  map[string][]string
		taken   map[string]string
		want    map[string][]string
		dropped int
		err     string
	}{
		{
			name:   "kept",
			answer: map[string][]string{"ofc:meat-loaf-made-with-beef": {"meatloaf", "beef meatloaf"}, "ofc:meat-loaf-made-with-ham": {"ham loaf"}},
			want:   map[string][]string{"ofc:meat-loaf-made-with-beef": {"meatloaf", "beef meatloaf"}, "ofc:meat-loaf-made-with-ham": {"ham loaf"}},
		},
		{
			name:   "leaves one out",
			answer: map[string][]string{"ofc:meat-loaf-made-with-beef": {"meatloaf"}},
			err:    "leaves out",
		},
		{
			name:   "answers another",
			answer: map[string][]string{"ofc:meat-loaf-made-with-beef": {}, "ofc:meat-loaf-made-with-ham": {}, "ofc:apple": {"apple"}},
			err:    "wasn't asked",
		},
		{
			name:   "one name for two of a family",
			answer: map[string][]string{"ofc:meat-loaf-made-with-beef": {"meatloaf"}, "ofc:meat-loaf-made-with-ham": {"Meatloaf"}},
			err:    "of one family",
		},
		{
			name:    "unacceptable and taken aliases are dropped",
			answer:  map[string][]string{"ofc:meat-loaf-made-with-beef": {"Meatloaf", "2 slices meatloaf", "meatloaf"}, "ofc:meat-loaf-made-with-ham": {"ham"}},
			taken:   map[string]string{"ham": "ofc:ham-nfs"},
			want:    map[string][]string{"ofc:meat-loaf-made-with-beef": {"meatloaf"}, "ofc:meat-loaf-made-with-ham": {}},
			dropped: 3,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, dropped, err := checkAnswer(loaves, c.answer, c.taken)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("err %v, want %q", err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(dropped) != c.dropped {
				t.Errorf("dropped %q, want %d", dropped, c.dropped)
			}
			for id, want := range c.want {
				if !slices.Equal(got[id], want) {
					t.Errorf("%s: %q, want %q", id, got[id], want)
				}
			}
		})
	}
}

func TestMergeRunAcrossBatches(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, v any) {
		data, _ := json.Marshal(v)
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A family split across two batches: both claim "meatloaf", and both keep it (the build picks one).
	write("batch-001.json", batchFile{Batch: "batch-001", Families: []family{{Family: "meat loaf", Foods: []batchFood{{ID: "ofc:a", Name: "Meat loaf made with beef"}}}}})
	write("answer-001.json", map[string][]string{"ofc:a": {"meatloaf", "beef meatloaf"}})
	write("batch-002.json", batchFile{Batch: "batch-002", Families: []family{{Family: "meat loaf", Foods: []batchFood{{ID: "ofc:b", Name: "Meat loaf made with ham"}}}}})
	write("answer-002.json", map[string][]string{"ofc:b": {"meatloaf"}})
	// No answer at all: refused, to be asked again.
	write("batch-003.json", batchFile{Batch: "batch-003", Families: []family{{Family: "apple", Foods: []batchFood{{ID: "ofc:c", Name: "Apple, raw"}}}}})

	res, err := mergeRun(dir, "m", "2026-11-16", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.refused["batch-003"]; !ok || len(res.refused) != 1 {
		t.Errorf("refused %v", res.refused)
	}
	if len(res.entries) != 2 || !slices.Equal(res.entries[0].Aliases, []string{"meatloaf", "beef meatloaf"}) || !slices.Equal(res.entries[1].Aliases, []string{"meatloaf"}) {
		t.Errorf("entries %+v", res.entries)
	}
	if len(res.dropped) != 0 {
		t.Errorf("dropped %q", res.dropped)
	}
	if res.entries[0].Model != "m" || res.entries[0].At != "2026-11-16" {
		t.Errorf("entry %+v", res.entries[0])
	}
}

func TestMatchesTakesAnyOfSeveralTexts(t *testing.T) {
	cases := []struct {
		text, match string
		want        bool
	}{
		{"potato, french fries, fast food", "", true},
		{"potato, french fries, fast food", "fast food|restaurant", true},
		{"pasta with cream sauce, restaurant", "fast food|restaurant", true},
		{"potato, french fries, from frozen", "fast food|restaurant", false},
	}
	for _, c := range cases {
		if got := matches(c.text, c.match); got != c.want {
			t.Errorf("matches(%q, %q) = %v, want %v", c.text, c.match, got, c.want)
		}
	}
}
