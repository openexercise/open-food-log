package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupplementGTIN(t *testing.T) {
	cases := map[string]struct{ gtin, why string }{
		"0 36000 29145 2":   {"00036000291452", ""},
		"0 3600-0291-45 2":  {"00036000291452", ""},
		" 036000291452 ":    {"00036000291452", ""},
		"1 633636 543505":   {"01633636543505", ""},
		"9638 5074":         {"00000096385074", ""},
		"00036000291452":    {"00036000291452", ""},
		"":                  {"", "none"},
		"   ":               {"", "none"},
		"0 36000 29145 3":   {"", "check digit"},
		"X000HBFWSN":        {"", "not digits"},
		"3100s":             {"", "not digits"},
		"0 36000 29145":     {"", "not 8, 12, 13 or 14 digits"}, // 11 digits: a UPC-A without its check digit
		"36000291452":       {"", "not 8, 12, 13 or 14 digits"}, // valid once zero-padded, as one in ten are by chance
		"800017":            {"", "not 8, 12, 13 or 14 digits"},
		"0 36000 29145 2 5": {"", "check digit"},
		"123456789012345":   {"", "not 8, 12, 13 or 14 digits"},
	}
	for in, want := range cases {
		gtin, why := supplementGTIN(in)
		if gtin != want.gtin || why != want.why {
			t.Errorf("supplementGTIN(%q) = %q, %q; want %q, %q", in, gtin, why, want.gtin, want.why)
		}
	}
}

func TestSkuShape(t *testing.T) {
	for in, want := range map[string]string{
		"0 48107 05843 2": "9 99999 99999 9",
		"X000HBFWSN":      "a999aaaaaa",
		" 39-2 ":          "99-9",
	} {
		if got := skuShape(in); got != want {
			t.Errorf("skuShape(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSupplementKey(t *testing.T) {
	cases := []struct{ group, name, key, why string }{
		{"Vitamin C", "Vitamin C", "vitamin_c_mg", ""},
		{"Vitamin C (unspecified)", "Vitamin C", "vitamin_c_mg", ""},
		{"Calcium", "Calcium", "calcium_mg", ""},
		{"Calcium", "elemental Calcium", "calcium_mg", ""},
		{"Calcium", "Calcium Carbonate", "", whyNamedOther},
		{"Sodium", "Sodium Alginate", "", whyNamedOther},
		{"Phosphorus", "Phosphorus", "phosphorus_mg", ""},
		{"Phosphorus", "Phosphorous", "phosphorus_mg", ""},
		{"Calories", "Calories", "energy_kcal", ""},
		{"Calories", "Calories from Fat", "", whyNamedOther},
		{"Header", "Calories", "energy_kcal", ""},
		{"Header", "Typical Fatty Acid Profile", "", whyNamedOther},
		{"Fat (unspecified)", "Total Fat", "fat_g", ""},
		{"Protein (unspecified)", "Protein", "protein_g", ""},
		{"Whey Protein", "Whey Protein Isolate", "", whyNoKey},
		{"Carbohydrate", "Total Carbohydrates", "carbohydrate_g", ""},
		{"Carbohydrate", "Other Carbohydrates", "", whyNamedOther},
		{"Carbohydrate (unspecified)", "Total Sugars", "sugars_g", ""},
		{"Sugar", "Sugar", "sugars_g", ""},
		{"Sugar", "Total Sugars", "sugars_g", ""},
		{"Sugar", "Added Sugars", "added_sugars_g", ""},
		{"Sugar Alcohol", "Sugar Alcohols", "polyols_g", ""},
		{"Fiber (unspecified)", "Dietary Fiber", "fiber_g", ""},
		{"Fiber (unspecified)", "Soluble Fiber", "", whyNamedOther},
		{"Psyllium fiber", "Psyllium", "", whyNoKey},
		{"Folate", "Folate", "folate_dfe_ug", ""},
		{"Folate", "Folic Acid", "folic_acid_ug", ""},
		{"Vitamin B9 (folic acid) ", "Folic Acid", "folic_acid_ug", ""},
		{"Vitamin B9 (5-Methyltetrahydrofolate)", "L-5-Methyltetrahydrofolate", "folate_dfe_ug", ""},
		{"Vitamin B1 (Thiamine)", "Thiamin", "thiamin_mg", ""},
		{"Vitamin B12 (methylcobalamin)", "Methylcobalamin", "vitamin_b12_ug", ""},
		{"Pantothenic Acid (Vitamin B5)", "Pantothenic Acid", "pantothenic_acid_mg", ""},
		{"Vitamin B5 (pantethine)", "Pantethine", "", whyNamedOther},
		{"Niacin", "Niacinamide", "niacin_mg", ""},
		{"Vitamin B3 (Nicotinamide riboside)", "Niagen", "", whyNamedOther},
		{"Vitamin A", "Vitamin A Palmitate", "vitamin_a_rae_ug", ""},
		{"Vitamin D (cholecalciferol)", "Vitamin D3", "vitamin_d_ug", ""},
		{"Vitamin E", "Vitamin E", "vitamin_e_mg", ""},
		{"Vitamin E", "D-Alpha-Tocopheryl Succinate", "vitamin_e_mg", ""},
		{"Vitamin E", "Gamma-Tocopherol", "", whyNamedOther},
		{"Vitamin E (mixed tocopherols)", "Vitamin E", "", whyNamedOther},
		{"Vitamin K", "Vitamin K", "vitamin_k_ug", ""},
		{"Vitamin K", "Vitamin K2", "", whyNamedOther},
		{"Vitamin K (menaquinone)", "Vitamin K", "", whyNamedOther},
		{"Vitamin K (Phylloquinone)", "Vitamin K1", "vitamin_k_ug", ""},
		{"Choline", "Choline", "choline_mg", ""},
		{"Choline", "Choline Bitartrate", "", whyNamedOther},
		{"Caffeine", "Caffeine Anhydrous", "caffeine_mg", ""},
		{"Caffeine", "Dicaffeine Malate", "", whyNamedOther},
		{"EPA (Eicosapentaenoic Acid)", "Eicosapentaenoic Acid", "epa_g", ""},
		{"DHA (Docosahexaenoic Acid)", "DHA", "dha_g", ""},
		{"Alpha-Linolenic Acid", "ALA", "ala_g", ""},
		{"Omega-3", "Total Omega-3 Fatty Acids", "", whyNoKey},
		{"Creatine", "Creatine Monohydrate", "", whyNoKey},
		{"Coenzyme Q-10", "CoQ10", "", whyNoKey},
		{"Proprietary Blend (Herb/Botanical)", "Energy Blend", "", whyNoKey},
		{"", "", "", whyNoKey},
	}
	for _, c := range cases {
		key, why := supplementKey(c.group, c.name)
		if key != c.key || why != c.why {
			t.Errorf("supplementKey(%q, %q) = %q, %q; want %q, %q", c.group, c.name, key, why, c.key, c.why)
		}
	}
}

// vocabularyUnits reads the repository's nutrient vocabulary.
func vocabularyUnits(t *testing.T) map[string]string {
	t.Helper()
	vocab, err := readVocabulary(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	units := map[string]string{}
	for _, v := range vocab {
		units[v.Key] = v.Unit
	}
	return units
}

// TestSupplementRulesNameVocabularyKeys keeps the mapping and the daily values to keys the
// vocabulary has, in units the measure converts.
func TestSupplementRulesNameVocabularyKeys(t *testing.T) {
	units := vocabularyUnits(t)
	for _, r := range supplementRules {
		unit, ok := units[r.key]
		if !ok {
			t.Errorf("rule for %q: not a vocabulary key", r.key)
			continue
		}
		if _, mass := micrograms[unit]; !mass && unit != "kcal" {
			t.Errorf("rule for %q: unit %q isn't one the measure converts", r.key, unit)
		}
	}
	for key := range dailyValues {
		if _, ok := units[key]; !ok {
			t.Errorf("daily value for %q: not a vocabulary key", key)
		}
	}
	for key := range iuFactors {
		if _, ok := dailyValues[key]; !ok || dailyValues[key].beforeIU == 0 {
			t.Errorf("%q converts from IU but has no daily value in IU", key)
		}
	}
}

func TestSupplementUnit(t *testing.T) {
	for in, want := range map[string]string{
		"Gram(s)": "g", "g": "g", "grams": "g", "mg": "mg", "Milligram(s)": "mg", "mcg": "mcg", "ug": "mcg",
		"IU": "IU", "UI": "IU", "Calorie(s)": "kcal", "{Calories}": "kcal", "Kcal": "kcal", "kJ": "kJ",
		"mcg RAE": "mcg RAE", "mcg DFE": "mcg DFE", "mg NE": "mg NE", "mg {alpha}-TE": "mg alpha-TE", "mg AT": "mg alpha-TE",
		"NP": "", "Not Present": "", "": "", "None": "", "CFU": "CFU", " mL ": "mL", "%": "%",
	} {
		if got := supplementUnit(in); got != want {
			t.Errorf("supplementUnit(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSupplementAmount(t *testing.T) {
	units := vocabularyUnits(t)
	cases := []struct {
		key    string
		amount float64
		unit   string
		value  float64
		how    string
	}{
		{"vitamin_c_mg", 500, "mg", 500, howDirect},
		{"vitamin_c_mg", 1, "g", 1000, howDirect},
		{"vitamin_b12_ug", 0.5, "mg", 500, howDirect},
		{"epa_g", 180, "mg", 0.18, howDirect},
		{"energy_kcal", 10, "kcal", 10, howDirect},
		{"energy_kcal", 41.84, "kJ", 10, howDirect},
		{"energy_kcal", 10, "g", 0, howNoneUnit},
		{"vitamin_a_rae_ug", 900, "mcg RAE", 900, howDirect},
		{"vitamin_a_rae_ug", 900, "mcg RE", 0, howNoneUnit},
		{"vitamin_a_rae_ug", 5000, "IU", 0, howIU},
		{"vitamin_d_ug", 400, "IU", 0, howIU},
		{"vitamin_e_mg", 30, "IU", 0, howIU},
		{"vitamin_e_mg", 15, "mg alpha-TE", 15, howDirect},
		{"niacin_mg", 16, "mg NE", 16, howDirect},
		{"vitamin_c_mg", 16, "mg NE", 0, howNoneUnit},
		{"folate_dfe_ug", 680, "mcg DFE", 680, howDirect},
		{"folate_dfe_ug", 1, "mg DFE", 1000, howDirect},
		{"folate_dfe_ug", 400, "mcg", 0, howNotDFE},
		{"folic_acid_ug", 400, "mcg", 400, howDirect},
		{"vitamin_c_mg", 100, "IU", 0, howNoneUnit},
		{"iron_mg", 15, "%", 0, howNoneUnit},
	}
	for _, c := range cases {
		value, how := supplementAmount(c.key, units[c.key], c.amount, c.unit)
		if how != c.how || !closeTo(value, c.value) {
			t.Errorf("supplementAmount(%s, %v %s) = %v, %q; want %v, %q", c.key, c.amount, c.unit, value, how, c.value, c.how)
		}
	}
}

func closeTo(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

func TestCheckDailyValue(t *testing.T) {
	units := vocabularyUnits(t)
	cases := []struct {
		name    string
		key     string
		amount  float64
		unit    string
		percent float64
		verdict string
		era     string
	}{
		{"current vitamin C", "vitamin_c_mg", 90, "mg", 100, dvAgrees, eraCurrent},
		{"vitamin C before 2016", "vitamin_c_mg", 500, "mg", 833, dvAgrees, eraBefore},
		{"vitamin C in mcg for mg", "vitamin_c_mg", 60, "mcg", 67, dvThousand, ""},
		{"vitamin C in mcg for mg, old DV", "vitamin_c_mg", 60, "mcg", 100, dvThousand, ""},
		{"iodine in mg for mcg", "iodine_ug", 150, "mg", 100, dvThousand, ""},
		{"zinc that is neither", "zinc_mg", 25, "mg", 100, dvDisagrees, ""},
		{"iron, the same either way", "iron_mg", 18, "mg", 100, dvAgrees, eraEither},
		{"a rounded small percent", "calcium_mg", 20, "mg", 2, dvAgrees, eraEither},
		{"fat a point or two out", "fat_g", 3, "g", 6, dvDisagrees, dvNarrowly},
		{"vitamin D in IU, old label", "vitamin_d_ug", 400, "IU", 100, dvAgrees, eraBefore},
		{"vitamin D in IU, new label", "vitamin_d_ug", 800, "IU", 100, dvAgrees, eraCurrent},
		{"vitamin D in mcg", "vitamin_d_ug", 25, "mcg", 125, dvAgrees, eraCurrent},
		{"vitamin A in IU, old label", "vitamin_a_rae_ug", 5000, "IU", 100, dvAgrees, eraBefore},
		{"vitamin A in mcg RAE", "vitamin_a_rae_ug", 450, "mcg RAE", 50, dvAgrees, eraCurrent},
		{"vitamin E in IU: 30 IU of the synthetic form is 13.5 mg, near today's 15", "vitamin_e_mg", 30, "IU", 100, dvAgrees, eraEither},
		{"vitamin E in IU, old label", "vitamin_e_mg", 60, "IU", 200, dvAgrees, eraEither},
		{"vitamin E in mg, new label", "vitamin_e_mg", 134, "mg", 893, dvAgrees, eraCurrent},
		{"natural vitamin E in IU, new label", "vitamin_e_mg", 22.4, "IU", 100, dvAgrees, eraCurrent},
		{"synthetic vitamin E in IU, new label", "vitamin_e_mg", 33.3, "IU", 100, dvAgrees, eraCurrent},
		{"folate in plain mcg", "folate_dfe_ug", 400, "mcg", 100, dvAgrees, eraEither},
		{"folate in DFE", "folate_dfe_ug", 680, "mcg DFE", 170, dvAgrees, eraEither},
		{"folic acid, old label", "folic_acid_ug", 400, "mcg", 100, dvAgrees, eraBefore},
		{"folic acid against DFE", "folic_acid_ug", 400, "mcg", 170, dvAgrees, eraCurrent},
		{"B12 old", "vitamin_b12_ug", 6, "mcg", 100, dvAgrees, eraBefore},
		{"B12 with a slipped percent", "vitamin_b12_ug", 50, "mcg", 8330, dvDisagrees, ""},
		{"choline has no old value", "choline_mg", 55, "mg", 10, dvAgrees, eraCurrent},
		{"no daily value for the key", "caffeine_mg", 100, "mg", 50, dvNotChecked, ""},
		{"a unit that doesn't convert", "iron_mg", 18, "%", 100, dvNotChecked, ""},
		{"no percent", "iron_mg", 18, "mg", 0, dvNotChecked, ""},
	}
	for _, c := range cases {
		verdict, era := checkDailyValue(c.key, units[c.key], c.amount, c.unit, c.percent)
		if verdict != c.verdict || era != c.era {
			t.Errorf("%s: got %q, %q; want %q, %q", c.name, verdict, era, c.verdict, c.era)
		}
	}
}

func TestIUForm(t *testing.T) {
	row := func(name, notes string, forms ...string) dsldRow {
		r := dsldRow{Name: name, Notes: notes}
		for _, f := range forms {
			r.Forms = append(r.Forms, dsldForm{Name: f})
		}
		return r
	}
	cases := []struct {
		key  string
		row  dsldRow
		want string
	}{
		{"vitamin_e_mg", row("Vitamin E", "", "D-Alpha-Tocopheryl Succinate"), "natural (d-alpha)"},
		{"vitamin_e_mg", row("Vitamin E", "", "DL-Alpha-Tocopheryl Acetate"), "synthetic (dl-alpha)"},
		{"vitamin_e_mg", row("Vitamin E", "Vitamin E (Form: as Natural Vitamin E)"), "natural (d-alpha)"},
		{"vitamin_e_mg", row("Vitamin E", ""), "not stated"},
		{"vitamin_a_rae_ug", row("Vitamin A", "", "Beta-Carotene"), "beta-carotene"},
		{"vitamin_a_rae_ug", row("Vitamin A", "", "Retinyl Palmitate", "Beta-Carotene"), "retinol and beta-carotene"},
		{"vitamin_a_rae_ug", row("Vitamin A Palmitate", ""), "retinol"},
		{"vitamin_a_rae_ug", row("Vitamin A", ""), "not stated"},
		{"vitamin_d_ug", row("Vitamin D", ""), "not needed (one factor for D2 and D3)"},
	}
	for _, c := range cases {
		if got := iuForm(c.key, c.row); got != c.want {
			t.Errorf("iuForm(%s, %q %v) = %q, want %q", c.key, c.row.Name, c.row.Forms, got, c.want)
		}
	}
}

// supplementLabels are made-up labels in the download's shape: a multivitamin on an old label, a
// blend, a label with a slipped unit, an off-market copy, and ones without a usable barcode.
var supplementLabels = map[string]string{
	"1": `{"id":1,"fullName":"Daily Multi","brandName":"Example","upcSku":"0 36000 29145 2","offMarket":0,"entryDate":"2014-03-01",
		"servingsPerContainer":"30","productType":{"langualCode":"A1315","langualCodeDescription":"Multi-Vitamin and Mineral (MVM)"},
		"physicalState":{"langualCode":"E0155","langualCodeDescription":"Tablet or Pill"},
		"servingSizes":[{"minQuantity":1,"maxQuantity":1,"unit":"Tablet(s)"}],
		"userGroups":[{"dailyValueTargetGroupName":"Adults and children 4 or more years of age"}],
		"contacts":[{"contactDetails":{"country":"USA"}}],
		"ingredientRows":[
			{"name":"Vitamin A","category":"vitamin","ingredientGroup":"Vitamin A","forms":[{"name":"Retinyl Palmitate"}],
			 "quantity":[{"operator":"=","quantity":5000,"unit":"IU","dailyValueTargetGroup":[{"name":"Adults and children 4 or more years of age","operator":"=","percent":100}]}],
			 "nestedRows":[{"name":"Beta-Carotene","category":"vitamin","ingredientGroup":"Vitamin A","quantity":[{"operator":"=","quantity":2500,"unit":"IU"}]}]},
			{"name":"Vitamin C","category":"vitamin","ingredientGroup":"Vitamin C",
			 "quantity":[{"operator":"=","quantity":60,"unit":"mg","dailyValueTargetGroup":[{"name":"Adults and children 4 or more years of age","operator":"=","percent":100}]}]},
			{"name":"Folic Acid","category":"vitamin","ingredientGroup":"Folate",
			 "quantity":[{"operator":"=","quantity":400,"unit":"mcg","dailyValueTargetGroup":[{"name":"Adults and children 4 or more years of age","operator":"=","percent":100}]}]}
		],
		"otheringredients":{"ingredients":[{"name":"Cellulose"}]}}`,
	"2": `{"id":2,"fullName":"Focus Blend","brandName":"Example","upcSku":"1 633636 543505","offMarket":0,"entryDate":"2023-05-01",
		"servingsPerContainer":null,"productType":{"langualCode":"A1325","langualCodeDescription":"Other Combinations"},
		"physicalState":{"langualCode":"E0159","langualCodeDescription":"Capsule"},
		"servingSizes":[{"minQuantity":1,"maxQuantity":2,"unit":"Capsule(s)"},{"minQuantity":3,"maxQuantity":3,"unit":"Capsule(s)"}],
		"labelRelationships":[{"type":"Flavor difference"}],
		"contacts":[{"contactDetails":{"country":"Canada"}}],
		"ingredientRows":[
			{"name":"Focus Blend","category":"blend","ingredientGroup":"Proprietary Blend (Combination)",
			 "quantity":[{"operator":"=","quantity":500,"unit":"mg"}],
			 "nestedRows":[
				{"name":"Caffeine Anhydrous","category":"non-nutrient/non-botanical","ingredientGroup":"Caffeine","quantity":[{"operator":"=","quantity":0,"unit":"NP"}]},
				{"name":"Creatine Monohydrate","category":"non-nutrient/non-botanical","ingredientGroup":"Creatine","quantity":[{"operator":"=","quantity":0,"unit":"NP"}]}]},
			{"name":"Choline Bitartrate","category":"non-nutrient/non-botanical","ingredientGroup":"Choline","quantity":[{"operator":"=","quantity":250,"unit":"mg"}]}
		]}`,
	"3": `{"id":3,"fullName":"Vitamin C 60","brandName":"Other Brand","upcSku":"9638 5074","offMarket":0,"entryDate":"2023-06-01",
		"productType":{"langualCode":"A1302","langualCodeDescription":"Vitamin"},
		"servingSizes":[{"minQuantity":1,"maxQuantity":1,"unit":"Tablet(s)"}],
		"ingredientRows":[
			{"name":"Vitamin C","category":"vitamin","ingredientGroup":"Vitamin C",
			 "quantity":[{"operator":"=","quantity":60,"unit":"mcg","dailyValueTargetGroup":[{"name":"Adults and children 4 or more years of age","operator":"=","percent":67}]}]},
			{"name":"Zinc","category":"mineral","ingredientGroup":"Zinc",
			 "quantity":[{"operator":"=","quantity":25,"unit":"mg","dailyValueTargetGroup":[{"name":"Adults and children 4 or more years of age","operator":"=","percent":100}]}]}
		]}`,
	"4":  `{"id":4,"fullName":"Daily Multi, 60 tablets","brandName":"Example","upcSku":"036000291452","offMarket":1,"entryDate":"2012-01-01","ingredientRows":null}`,
	"5":  `{"id":5,"fullName":"Sold Online","brandName":"Example","upcSku":"X000HBFWSN","offMarket":0,"entryDate":"2024-01-01","ingredientRows":[]}`,
	"6":  `{"id":6,"fullName":"No Barcode","brandName":"Example","upcSku":"","offMarket":0,"entryDate":"2024-01-01","ingredientRows":[]}`,
	"10": `{"id":10,"fullName":"Bad Check Digit","brandName":"Example","upcSku":"0 36000 29145 3","offMarket":0,"entryDate":"2024-01-01","ingredientRows":[]}`,
	"11": `{"id": "not a number"}`,
}

func TestReadRows(t *testing.T) {
	units := vocabularyUnits(t)
	var l dsldLabel
	if err := json.Unmarshal([]byte(supplementLabels["1"]), &l); err != nil {
		t.Fatal(err)
	}
	rows := readRows(l.IngredientRows, units)
	want := []struct {
		name, key, status, how string
		nested                 bool
	}{
		{"Vitamin A", "vitamin_a_rae_ug", statusMapped, howIU, false},
		{"Beta-Carotene", "vitamin_a_rae_ug", whyPartOfMapped, "", true},
		{"Vitamin C", "vitamin_c_mg", statusMapped, howDirect, false},
		{"Folic Acid", "folic_acid_ug", statusMapped, howDirect, false},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d", len(rows), len(want))
	}
	for i, w := range want {
		r := rows[i]
		if r.row.Name != w.name || r.key != w.key || r.status != w.status || r.how != w.how || r.nested != w.nested {
			t.Errorf("row %d: got %s %q %q %q nested=%v, want %+v", i, r.row.Name, r.key, r.status, r.how, r.nested, w)
		}
	}
	checks := readDailyValues(rows, units)
	if len(checks) != 3 {
		t.Fatalf("got %d checks, want 3: %+v", len(checks), checks)
	}
	for _, c := range checks {
		if c.verdict != dvAgrees || c.era != eraBefore {
			t.Errorf("%s: got %q, %q; want an old label's agreement", c.name, c.verdict, c.era)
		}
	}

	if err := json.Unmarshal([]byte(supplementLabels["2"]), &l); err != nil {
		t.Fatal(err)
	}
	rows = readRows(l.IngredientRows, units)
	states := []string{amountPresent, amountNP, amountNP, amountPresent}
	statuses := []string{whyNoKey, statusMapped, whyNoKey, whyNamedOther}
	for i, r := range rows {
		if r.state != states[i] || r.status != statuses[i] {
			t.Errorf("%s: got %q, %q; want %q, %q", r.row.Name, r.state, r.status, states[i], statuses[i])
		}
	}
}

// TestSupplementsMeasure runs the whole measure over a handful of made-up labels in a zip.
func TestSupplementsMeasure(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{filepath.Join("schema", "v0"), "catalogue"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	vocab, err := os.ReadFile(filepath.Join("..", "..", "schema", "v0", "nutrients.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "schema", "v0", "nutrients.json"), vocab, 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "DSLD-full-database-JSON.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	files := map[string]string{"DSLD-full-database-JSON/ReadMe.csv": "Welcome\nDate of download,2026-01-10T18:08:23-05:00\n\"Version 9.4.0 | January 2026\"\n"}
	for id, body := range supplementLabels {
		files["DSLD-full-database-JSON/"+id+".json"] = body
	}
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	gtins := filepath.Join(t.TempDir(), "gtins.txt")
	if err := os.WriteFile(gtins, []byte("00036000291452\n00000000000017\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := supplementsMeasure(path, root, supplementsOptions{}); err == nil {
		t.Error("no retrieval date should be refused")
	}
	res, err := supplementsMeasure(path, root, supplementsOptions{Retrieved: "2026-11-21", BrandedGTINs: gtins})
	if err != nil {
		t.Fatal(err)
	}
	if want := (supplementsResult{Read: 8, Failed: 1, Candidates: 3, CandidateGTINs: 3}); res != want {
		t.Errorf("got %+v, want %+v", res, want)
	}
	out, err := os.ReadFile(filepath.Join(root, "catalogue", "SUPPLEMENTS-REPORT.md"))
	if err != nil {
		t.Fatal(err)
	}
	report := string(out)
	for _, want := range []string{
		"| File | `DSLD-full-database-JSON.zip` |",
		"| Retrieved | 2026-11-21 |",
		"| DSLD's own date of download (its `ReadMe.csv`) | 2026-01-10T18:08:23-05:00 |",
		"| DSLD release (its `ReadMe.csv`) | Version 9.4.0 \\| January 2026 |",
		"CC0 1.0 Universal",
		"| Label files read | 8 |",
		"| Didn't parse | 1 |",
		"Didn't parse: `11.json`",
		"| On the market (`offMarket` 0) | 6 |",
		"| Off the market (`offMarket` 1) | 1 |",
		"| 2014 | 1 | 1 | 0 | 1 |",
		"| A non-empty `upcSku` | 6 | 85.7% |",
		"| A valid GTIN | 4 | 57.1% |",
		"| On the market with a valid GTIN (**the candidate set**) | 3 | 42.9% |",
		"| Distinct GTINs | 3 |",
		"| check digit | 1 |",
		"| not digits | 1 |",
		"| not digits | `a999aaaaaa` | 1 | `X000HBFWSN` |",
		"| 2 | 1 |", // one GTIN on two labels
		"| Exactly one label on the market | 1 | 100.0% |",
		"| Labels with different `fullName`s | 1 | 100.0% |",
		"1 labels (16.7% of those with an `upcSku`) give a code shaped like the ones Amazon prints",
		"| The candidate set's GTINs | 3 | 1 | 33.3% |",
		"| A1315 Multi-Vitamin and Mineral (MVM) | 1 | 1 |",
		"| More than one serving size | 1 | 1 |",
		"| A serving size given as a range (`minQuantity` below `maxQuantity`) | 1 | 1 |",
		"| `servingsPerContainer` present | 1 | 1 |",
		"| Rows | 7 | 3 | 7 | 3 |",
		"| `vitamin_c_mg` | mg | vitamin c |",
		"| Mapped to a key | 6 | 66.7% | 6 | 66.7% |",                        // every row: the caffeine row maps though it states no amount
		"| Mapped to a key | 5 | 71.4% | 5 | 71.4% |",                        // rows with an amount
		"| Every row with an amount maps | 2 | 28.6% | 2 | 66.7% |",          // labels 1 and 3
		"| No row with an amount maps | 1 | 14.3% | 1 | 33.3% |",             // label 2
		"| Creatine | 1 | 33.3% | 1 | 0 |",                                   // unmapped group
		"| Choline: Choline Bitartrate | 1 |",                                // kept out by its name
		"| `vitamin_a_rae_ug` | 1 | 1 | 0 | 1 | 0 | 0 |",                     // in IU
		"| `vitamin_a_rae_ug` | retinol | 1 |",                               // the form its IU row states
		"| `vitamin_c_mg` | 2 | 1 | 0 | 1 | 0 | 1 | 0 | 0 | 50.0% | 50.0% |", // one old label, one mcg for mg
		"| `zinc_mg` | 1 | 0 | 0 | 0 | 0 | 0 | 1 | 0 | 100.0% | 100.0% |",
		"| 3 | Vitamin C 60 (Other Brand) | Zinc | `zinc_mg` | 25 mg | 100% |",
		"| 3 | Vitamin C 60 (Other Brand) | Vitamin C | `vitamin_c_mg` | 60 mcg | 67% |",
		"| Only the daily values before 2016 | 1 | 14.3% | 1 | 33.3% |",
		"One draft record for each of the candidate set's 3 GTINs",
		"| A row the label gives no amount for (unit `NP`) | 1 | 14.3% | 1 | 33.3% |",
		"| A contact naming a country other than the United States | 1 |",
		"| Canada | 1 |",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q", want)
		}
	}

	if _, err := supplementsMeasure(path, root, supplementsOptions{Retrieved: "2026-11-21"}); err != nil {
		t.Fatal(err)
	}
	out, err = os.ReadFile(filepath.Join(root, "catalogue", "SUPPLEMENTS-REPORT.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "Not counted: the command ran without `--branded-gtins`") {
		t.Error("without the list, the report should say the overlap wasn't counted")
	}
}
