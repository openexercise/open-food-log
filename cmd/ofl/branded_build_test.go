package main

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	openfoodlog "github.com/openexercise/open-food-log"
)

// TestBuildBranded builds the branded catalogue from a handful of records, one for each rule and
// each shape of record, and checks every line against the food schema.
func TestBuildBranded(t *testing.T) {
	nutrients := func(kcal, protein, fat, carb float64) string {
		b, _ := json.Marshal([]map[string]any{
			{"nutrient": map[string]string{"number": "208", "unitName": "kcal"}, "amount": kcal},
			{"nutrient": map[string]string{"number": "203", "unitName": "g"}, "amount": protein},
			{"nutrient": map[string]string{"number": "204", "unitName": "g"}, "amount": fat},
			{"nutrient": map[string]string{"number": "205", "unitName": "g"}, "amount": carb},
			{"nutrient": map[string]string{"number": "307", "unitName": "mg"}, "amount": 120},
			{"nutrient": map[string]string{"number": "318", "unitName": "IU"}, "amount": 50},
		})
		return `"foodNutrients":` + string(b)
	}
	granola := nutrients(500, 10.7, 25, 57.1)
	records := []string{
		// Kept: the newer of two versions of a granola, on a 100 g basis, with ingredients.
		`{"fdcId":3,"marketCountry":"United States","gtinUpc":"036000291452","availableDate":"1/1/2020","description":"OLD GRANOLA",` + granola + `}`,
		`{"fdcId":4,"marketCountry":"United States","gtinUpc":"036000291452","availableDate":"1/1/2024","modifiedDate":"12/15/2023","publicationDate":"2/1/2024","dataSource":"LI",
		  "description":"HONEY ALMOND GRANOLA","brandOwner":"Acme Foods Inc.","brandName":"ACME","brandedFoodCategory":"Chips, Pretzels & Snacks",
		  "ingredients":"WHOLE GRAIN OATS,\nHONEY\u0007, ALMONDS.","servingSize":28,"servingSizeUnit":"GRM","householdServingFullText":"1/4 cup",
		  "labelNutrients":{"calories":{"value":140},"protein":{"value":3},"carbohydrates":{"value":16},"vitaminD":{"value":0}},` + granola + `}`,
		// Kept: a drink on a 100 ml basis, an 11-digit UPC-A, no household serving, no brand name.
		`{"fdcId":7,"marketCountry":"US","gtinUpc":"12345678905","description":"Sparkling Water, Lime","brandOwner":"Fizz Co","dataSource":"GDSN",
		  "servingSize":355,"servingSizeUnit":"MLT",` + nutrients(0, 0, 0, 0) + `}`,
		// Dropped: Canada, a bad check digit, discontinued, no energy.
		`{"fdcId":1,"marketCountry":"Canada","gtinUpc":"00000096385074",` + granola + `}`,
		`{"fdcId":2,"marketCountry":"United States","gtinUpc":"036000291453",` + granola + `}`,
		`{"fdcId":5,"marketCountry":"United States","gtinUpc":"96385074","discontinuedDate":"1/1/2025",` + granola + `}`,
		`{"fdcId":6,"marketCountry":"United States","gtinUpc":"00000000000000","foodNutrients":[]}`,
	}
	path := filepath.Join(t.TempDir(), "FoodData_Central_branded_food_json_2026-10-29.json")
	if err := os.WriteFile(path, []byte(`{"BrandedFoods": [`+strings.Join(records, ",\n")+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "branded-foods.jsonl.gz")
	res, err := buildBranded(path, out)
	if err != nil {
		t.Fatal(err)
	}
	if res.Records != 2 {
		t.Errorf("%d records, want 2", res.Records)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != res.SHA256 || int64(len(data)) != res.Bytes {
		t.Errorf("result says %s, %d bytes; the file is %s, %d", res.SHA256, res.Bytes, got, len(data))
	}
	check, err := os.ReadFile(out + ".sha256")
	if err != nil || string(check) != res.SHA256+"  branded-foods.jsonl.gz\n" {
		t.Errorf("checksum file %q, %v", check, err)
	}

	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	v, err := openfoodlog.NewValidator()
	if err != nil {
		t.Fatal(err)
	}
	foods := map[string]map[string]any{}
	var order []string
	scanner := bufio.NewScanner(gz)
	for scanner.Scan() {
		if problems := v.ValidateAs("food", scanner.Bytes()); problems != nil {
			t.Fatalf("%s: %v", scanner.Text(), problems)
		}
		var food map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &food); err != nil {
			t.Fatal(err)
		}
		id := food["id"].(string)
		foods[id] = food
		order = append(order, id)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(order, " ") != "gtin:00012345678905 gtin:00036000291452" {
		t.Errorf("records out of order: %v", order)
	}

	encode := func(v any) string { b, _ := marshalPlain(v); return string(b) }
	if !strings.Contains(string(mustGunzip(t, out)), `"Chips, Pretzels & Snacks"`) {
		t.Error("the file escapes & in text")
	}
	granolaFood := foods["gtin:00036000291452"]
	for field, want := range map[string]string{
		"name":               `"Honey almond granola"`,
		"kind":               `"branded"`,
		"brand":              `"ACME"`,
		"gtin":               `["00036000291452"]`,
		"nutrients_per_100g": `{"carbohydrate_g":57.1,"energy_kcal":500,"fat_g":25,"protein_g":10.7,"sodium_mg":120}`,
		"label":              `{"nutrients_per_serving":{"carbohydrate_g":16,"energy_kcal":140,"protein_g":3},"region":"us","serving":{"grams":28,"text":"1/4 cup"}}`,
		"category":           `{"basis":"inferred","name":"Chips, Pretzels & Snacks","scheme":"fdc"}`,
		"ingredients_text":   `{"basis":"declared","locale":"en","text":"WHOLE GRAIN OATS, HONEY, ALMONDS."}`,
		"identifiers":        `{"fdc_id":4}`,
		"provenance":         `{"license":"CC0-1.0","review":"unreviewed","sources":[{"id":"4","license":"CC0-1.0","name":"USDA FoodData Central, Global Branded Food Products Database (2026-10-29): data provider Acme Foods Inc., through Label Insight, available 2024-01-01, modified 2023-12-15, published 2024-02-01, described \"HONEY ALMOND GRANOLA\"","url":"https://fdc.nal.usda.gov/food-details/4/nutrients"}]}`,
	} {
		if got := encode(granolaFood[field]); got != want {
			t.Errorf("granola %s = %s, want %s", field, got, want)
		}
	}
	if _, ok := granolaFood["nutrients_basis"]; ok {
		t.Error("a 100 g record names its basis; 100g is the default")
	}

	water := foods["gtin:00012345678905"]
	for field, want := range map[string]string{
		"name":                `"Sparkling Water, Lime"`,
		"brand":               `"Fizz Co"`,
		"nutrients_basis":     `"100ml"`,
		"nutrients_per_100g":  `{}`,
		"nutrients_per_100ml": `{"carbohydrate_g":0,"energy_kcal":0,"fat_g":0,"protein_g":0,"sodium_mg":120}`,
		"label":               `{"region":"us","serving":{"ml":355,"text":"355 ml"}}`,
	} {
		if got := encode(water[field]); got != want {
			t.Errorf("water %s = %s, want %s", field, got, want)
		}
	}
	if _, ok := water["ingredients_text"]; ok {
		t.Error("a record without ingredients has ingredients_text")
	}

	// The same download builds the same file, so the checksum can be pinned.
	again, err := buildBranded(path, filepath.Join(t.TempDir(), "branded-foods.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if again.SHA256 != res.SHA256 {
		t.Error("two builds of the same download differ")
	}
}

func mustGunzip(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
