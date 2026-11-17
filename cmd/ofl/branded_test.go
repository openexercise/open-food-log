package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeGTIN(t *testing.T) {
	cases := map[string]struct{ gtin, why string }{
		"036000291452":    {"00036000291452", ""},
		"1633636543505":   {"01633636543505", ""},
		"96385074":        {"00000096385074", ""},
		"00036000291452":  {"00036000291452", ""},
		"":                {"", "none"},
		"03600029145X":    {"", "not digits"},
		"0360002914":      {"", "check digit"},
		"36000291452":     {"00036000291452", ""},
		"123456789012345": {"", "over 14 digits"},
		"036000291453":    {"", "check digit"},
	}
	for in, want := range cases {
		gtin, why := normalizeGTIN(in)
		if gtin != want.gtin || why != want.why {
			t.Errorf("normalizeGTIN(%q) = %q, %q; want %q, %q", in, gtin, why, want.gtin, want.why)
		}
	}
}

func TestBasis(t *testing.T) {
	v := func(f float64) *float64 { return &f }
	label := func(kcal float64) map[string]struct {
		Value *float64 `json:"value"`
	} {
		return map[string]struct {
			Value *float64 `json:"value"`
		}{"calories": {Value: v(kcal)}}
	}
	cases := []struct {
		name string
		food brandedFood
		kcal float64
		want string
	}{
		{"grams", brandedFood{ServingSizeUnit: "GRM"}, 100, "100g"},
		{"millilitres", brandedFood{ServingSizeUnit: "mlt"}, 100, "100ml"},
		{"MG that is grams", brandedFood{ServingSizeUnit: "MG", ServingSize: v(38), PackageWeight: "5.3 oz/150 g", LabelNutrients: label(230)}, 605, "100g"},
		{"MG that is millilitres", brandedFood{ServingSizeUnit: "MG", ServingSize: v(250), PackageWeight: "8.4 fl oz", LabelNutrients: label(10)}, 4, "100ml"},
		{"IU with a weight beside fl oz", brandedFood{ServingSizeUnit: "IU", ServingSize: v(15), HouseholdServingFullText: "1 tbs", PackageWeight: "8.5 FL OZ/250 g", LabelNutrients: label(50)}, 333, "100g"},
		{"label disagrees", brandedFood{ServingSizeUnit: "MG", ServingSize: v(38), LabelNutrients: label(100)}, 605, ""},
		{"no label", brandedFood{ServingSizeUnit: "IU", ServingSize: v(38)}, 605, ""},
	}
	for _, c := range cases {
		if got, _ := basis(c.food, map[string]float64{"energy_kcal": c.kcal}); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestNewer(t *testing.T) {
	a := brandedFood{FdcID: 1, AvailableDate: "4/26/2020", ModifiedDate: "1/1/2025"}
	b := brandedFood{FdcID: 2, AvailableDate: "10/1/2021", ModifiedDate: "1/1/2019"}
	if !newer(b, a) || newer(a, b) {
		t.Error("the later available date should win")
	}
	c := brandedFood{FdcID: 3, AvailableDate: "4/26/2020", ModifiedDate: "1/1/2025"}
	if !newer(c, a) {
		t.Error("the higher FDC ID should break a tie")
	}
	d := brandedFood{FdcID: 0, AvailableDate: "4/26/2020", ModifiedDate: "2/1/2025"}
	if !newer(d, c) {
		t.Error("the later modified date should come before the FDC ID")
	}
}

func TestImplausible(t *testing.T) {
	serving := func(v float64) *float64 { return &v }
	cases := []struct {
		name string
		food brandedFood
		n    map[string]float64
		want string
	}{
		{"fine", brandedFood{ServingSizeUnit: "g"}, map[string]float64{"energy_kcal": 500, "protein_g": 10.7, "fat_g": 25, "carbohydrate_g": 57.1}, ""},
		{"no energy", brandedFood{ServingSizeUnit: "g"}, map[string]float64{"protein_g": 1}, whyNoEnergy},
		{"zero serving", brandedFood{ServingSize: serving(0)}, map[string]float64{"energy_kcal": 1}, whyNegative},
		{"over 100 g", brandedFood{ServingSizeUnit: "g"}, map[string]float64{"energy_kcal": 400, "protein_g": 50, "fat_g": 10, "carbohydrate_g": 50}, whyOverLimit},
		{"unit unread", brandedFood{ServingSizeUnit: "IU"}, map[string]float64{"energy_kcal": 100}, whyNoBasis},
		{"over 900 kcal", brandedFood{ServingSizeUnit: "g"}, map[string]float64{"energy_kcal": 901}, whyOverEnergy},
		{"Atwater far off", brandedFood{ServingSizeUnit: "g"}, map[string]float64{"energy_kcal": 100, "protein_g": 20, "fat_g": 10, "carbohydrate_g": 20}, whyEnergyAtwater},
		{"Atwater small gap", brandedFood{ServingSizeUnit: "g"}, map[string]float64{"energy_kcal": 20, "protein_g": 0, "fat_g": 0, "carbohydrate_g": 10}, ""},
		{"no macros, no check", brandedFood{ServingSizeUnit: "g"}, map[string]float64{"energy_kcal": 100}, ""},
	}
	for _, c := range cases {
		if got := implausible(c.food, c.n); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSentenceCase(t *testing.T) {
	for in, want := range map[string]string{
		"ORIGINAL GREEK YOGURT":   "Original greek yogurt",
		"Ferris Cinnamon Almonds": "Ferris Cinnamon Almonds",
		"100% WHOLE WHEAT BREAD":  "100% Whole wheat bread",
	} {
		if got := sentenceCase(in); got != want {
			t.Errorf("sentenceCase(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestBrandedReport runs the whole report over a handful of records, one for each rule.
func TestBrandedReport(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "schema", "v0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "catalogue"), 0o755); err != nil {
		t.Fatal(err)
	}
	vocab, err := os.ReadFile(filepath.Join("..", "..", "schema", "v0", "nutrients.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema", "v0", "nutrients.json"), vocab, 0o644); err != nil {
		t.Fatal(err)
	}
	nutrients := `"foodNutrients":[{"nutrient":{"number":"208","unitName":"kcal"},"amount":500},{"nutrient":{"number":"203","unitName":"g"},"amount":10.7},{"nutrient":{"number":"204","unitName":"g"},"amount":25},{"nutrient":{"number":"205","unitName":"g"},"amount":57.1}]`
	records := []string{
		`{"fdcId":1,"marketCountry":"Canada","gtinUpc":"036000291452",` + nutrients + `}`,
		`{"fdcId":2,"marketCountry":"United States","gtinUpc":"036000291453",` + nutrients + `}`,
		`{"fdcId":3,"marketCountry":"United States","gtinUpc":"036000291452","availableDate":"1/1/2020",` + nutrients + `}`,
		`{"fdcId":4,"marketCountry":"United States","gtinUpc":"036000291452","availableDate":"1/1/2024","ingredients":"OATS","description":"GRANOLA","servingSize":28,"servingSizeUnit":"g",` + nutrients + `}`,
		`{"fdcId":5,"marketCountry":"United States","gtinUpc":"96385074","discontinuedDate":"1/1/2025",` + nutrients + `}`,
		`{"fdcId":6,"marketCountry":"United States","gtinUpc":"00000000000000","foodNutrients":[]}`,
	}
	path := filepath.Join(t.TempDir(), "FoodData_Central_branded_food_json_2026-10-29.json")
	if err := os.WriteFile(path, []byte(`{"BrandedFoods": [`+strings.Join(records, ",\n")+`]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := brandedReport(path, root); err != nil {
		t.Fatal(err)
	}
	report, err := os.ReadFile(filepath.Join(root, "catalogue", "BRANDED-REPORT.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"| Read | 6 | |",
		"| 1. US market only (`United States` or `US`, kept as `US`) | 5 | 1 |",
		"| 2. A valid GTIN (up to 14 digits, zero-padded, check digit) | 4 | 1 |",
		"| 3. One per GTIN, newest kept | 3 | 1 |",
		"| 4. Not discontinued | 2 | 1 |",
		"| 5. Plausible numbers | 1 | 1 |",
		"| **Kept** | **1** | |",
		"| " + whyNoEnergy + " | 1 |",
	} {
		if !strings.Contains(string(report), want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}
