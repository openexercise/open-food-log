package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddedSugarsGrams(t *testing.T) {
	// USDA's factor: one teaspoon equivalent is 4.2 g of added sugars, to one decimal place.
	for teaspoons, want := range map[float64]float64{0: 0, 0.1: 0.4, 0.5: 2.1, 1: 4.2, 2.4: 10.1, 7.5: 31.5, 23.8: 100} {
		if got := addedSugarsGrams(teaspoons); got != want {
			t.Errorf("%v teaspoons is %v g, want %v", teaspoons, got, want)
		}
	}
}

func TestAddedSugarsFor(t *testing.T) {
	cases := []struct {
		name      string
		teaspoons float64
		sugars    float64
		hasSugars bool
		grams     float64
		capped    bool
		ok        bool
	}{
		{"under the total", 7.5, 32.9, true, 31.5, false, true},
		{"equal to the total", 1, 4.2, true, 4.2, false, true},
		{"a stated zero", 0, 4.81, true, 0, false, true},
		{"zero in a food with no sugars", 0, 0, true, 0, false, true},
		{"just above: the total", 2.4, 9.94, true, 9.94, true, true},
		{"2.1 g above: still the total", 1, 2.1, true, 2.1, true, true},
		{"more than 2.1 g above: none", 1, 2.09, true, 0, false, false},
		{"far above: none", 10, 5, true, 0, false, false},
		{"no total sugars: none", 1, 0, false, 0, false, false},
		{"no total sugars, zero: none", 0, 0, false, 0, false, false},
	}
	for _, c := range cases {
		grams, capped, ok := addedSugarsFor(c.teaspoons, c.sugars, c.hasSugars)
		if grams != c.grams || capped != c.capped || ok != c.ok {
			t.Errorf("%s: got %v, capped %v, ok %v", c.name, grams, capped, ok)
		}
	}
}

func TestAddSugars(t *testing.T) {
	survey, foundation, sr := sourceLabel("survey"), sourceLabel("foundation"), sourceLabel("sr")
	food := func(id int, primary string, nutrients map[string]float64) *catalogueFood {
		return &catalogueFood{
			NutrientsPer100g: nutrients, Identifiers: map[string]int{"fdc_id": id},
			Provenance: provenanceDoc{Sources: []sourceDoc{{Name: primary, ID: "x"}, {Name: modelSourcePrefix + "a model"}}},
		}
	}
	cookie := food(1, survey, map[string]float64{"sugars_g": 32.9})
	milk := food(2, survey, map[string]float64{"sugars_g": 4.81})
	cola := food(3, survey, map[string]float64{"sugars_g": 9.94})
	odd := food(4, survey, map[string]float64{"sugars_g": 1})
	noTotal := food(5, survey, map[string]float64{"energy_kcal": 10})
	noRow := food(6, survey, map[string]float64{"sugars_g": 3})
	noCode := food(7, survey, map[string]float64{"sugars_g": 3})
	// A survey food whose name a Foundation Foods record took: the numbers are Foundation's.
	taken := food(8, foundation, map[string]float64{"sugars_g": 12})
	taken.Provenance.Sources = append(taken.Provenance.Sources, sourceDoc{Name: survey, ID: "9"})
	legacy := food(10, sr, map[string]float64{"sugars_g": 40})
	foods := []*catalogueFood{cookie, milk, cola, odd, noTotal, noRow, noCode, taken, legacy}
	codes := map[int]string{1: "c1", 2: "c2", 3: "c3", 4: "c4", 5: "c5", 6: "c6", 9: "c9", 10: "c10"}
	fped := map[string]float64{"c1": 7.5, "c2": 0, "c3": 2.4, "c4": 2, "c5": 1, "c9": 3, "c10": 9, "unused": 1}

	check := func() {
		t.Helper()
		got := addSugars(foods, codes, fped)
		want := sugarStats{rows: 8, unusedRows: 3, survey: 7, filled: 3, zero: 1, capped: 1, noRow: 2, noSugars: 1, above: 1}
		if got != want {
			t.Errorf("stats %+v, want %+v", got, want)
		}
		for _, c := range []struct {
			food *catalogueFood
			want float64
			name string
			code string
		}{{cookie, 31.5, fpedSourceName, "c1"}, {milk, 0, fpedSourceName, "c2"}, {cola, 9.94, fpedSourceName + fpedCappedNote, "c3"}} {
			if got, ok := c.food.NutrientsPer100g["added_sugars_g"]; !ok || got != c.want {
				t.Errorf("food %s has %v (%v), want %v", c.code, got, ok, c.want)
			}
			var named []sourceDoc
			for _, s := range c.food.Provenance.Sources {
				if isFPED(s) {
					named = append(named, s)
				}
			}
			if len(named) != 1 || named[0] != (sourceDoc{Name: c.name, ID: c.code, URL: fpedURL, License: fpedLicense, RetrievedAt: fpedRetrieved}) {
				t.Errorf("food %s names %+v", c.code, named)
			}
		}
		for _, f := range []*catalogueFood{odd, noTotal, noRow, noCode, taken, legacy} {
			if _, ok := f.NutrientsPer100g["added_sugars_g"]; ok {
				t.Errorf("food %d has added sugars", f.Identifiers["fdc_id"])
			}
			for _, s := range f.Provenance.Sources {
				if isFPED(s) {
					t.Errorf("food %d names the database", f.Identifiers["fdc_id"])
				}
			}
		}
	}
	check()
	// A second run takes the first's values away before giving them again.
	check()

	// A stated zero is written, not left out.
	line, err := json.Marshal(milk)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(line), `"added_sugars_g":0,`) {
		t.Errorf("zero isn't stated: %s", line)
	}

	// A row that goes makes its food lose the value and the source.
	delete(fped, "c1")
	addSugars(foods, codes, fped)
	if _, ok := cookie.NutrientsPer100g["added_sugars_g"]; ok || len(cookie.Provenance.Sources) != 2 {
		t.Errorf("the cookie kept %+v", cookie)
	}
}

func TestFoodCode(t *testing.T) {
	for in, want := range map[string]string{
		`{"fdcId": 1, "foodCode": "11000000"}`:   "11000000",
		`{"fdcId": 1, "foodCode": 11000000}`:     "11000000",
		`{"fdcId": 1, "foodCode": " 11000000 "}`: "11000000",
		`{"fdcId": 1, "foodCode": null}`:         "",
		`{"fdcId": 1}`:                           "",
	} {
		var f fdcFood
		if err := json.Unmarshal([]byte(in), &f); err != nil || string(f.FoodCode) != want {
			t.Errorf("%s: read %q, %v", in, f.FoodCode, err)
		}
	}
	for _, in := range []string{`{"foodCode": 1.5}`, `{"foodCode": true}`, `{"foodCode": [1]}`} {
		var f fdcFood
		if err := json.Unmarshal([]byte(in), &f); err == nil {
			t.Errorf("%s was read as %q", in, f.FoodCode)
		}
	}
}

// TestAddedSugarsSources checks the committed inputs: every FPED row is a food code with an
// amount to one decimal place, and every survey food's code has a row.
func TestAddedSugarsSources(t *testing.T) {
	root := filepath.Join("..", "..")
	fped, err := readFPED(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(fped) != 5432 {
		t.Errorf("%d FPED rows, want the 5,432 of FPED 2021-2023", len(fped))
	}
	for code, teaspoons := range fped {
		if len(code) != 8 || teaspoons != round(teaspoons, 1) {
			t.Errorf("food code %q has %v teaspoons", code, teaspoons)
		}
	}
	codes, err := readFoodCodes(root)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for id, code := range codes {
		if _, ok := fped[code]; !ok {
			t.Errorf("FDC ID %d: food code %s has no FPED row", id, code)
		}
		if seen[code] {
			t.Errorf("food code %s names two survey foods", code)
		}
		seen[code] = true
	}
}

func TestReadCSV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.csv")
	if err := os.WriteFile(path, []byte("# a note, with a comma\na,b\n1,2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows, err := readCSV(path, "a", "b")
	if err != nil || len(rows) != 1 || rows[0][0] != "1" || rows[0][1] != "2" {
		t.Errorf("read %v, %v", rows, err)
	}
	if _, err := readCSV(path, "a", "c"); err == nil {
		t.Error("other columns were accepted")
	}
}
