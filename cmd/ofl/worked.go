package main

import (
	"fmt"
	"math"
	"strings"
)

// deriveNutrients adds the values OFL v0 section 4 defines from others, and only those, to a
// food's nutrients, returning the keys it added. FoodData Central's own value always wins.
//   - energy_kj is energy_kcal × 4.184 where FDC gives no kilojoules (268).
//   - salt_g is sodium × 2.5, as EU and UK labels declare it; FDC's own salt (375) is never in
//     these downloads.
//   - carbohydrate_available_g is carbohydrate_g − fiber_g (carbohydrate_g = available + fibre).
//     Both of FDC's totals, by difference (205) and by summation (205.2), include fibre.
func deriveNutrients(n map[string]float64) []string {
	var worked []string
	if kcal, ok := n["energy_kcal"]; ok {
		if _, has := n["energy_kj"]; !has {
			n["energy_kj"] = round(kcal*4.184, 1)
			worked = append(worked, "energy_kj")
		}
	}
	if na, ok := n["sodium_mg"]; ok {
		if _, has := n["salt_g"]; !has {
			n["salt_g"] = round(na*2.5/1000, 5)
			worked = append(worked, "salt_g")
		}
	}
	carb, hasCarb := n["carbohydrate_g"]
	fiber, hasFiber := n["fiber_g"]
	if _, has := n["carbohydrate_available_g"]; hasCarb && hasFiber && !has {
		// Fibre above the total is two analyses disagreeing; no available value is given then.
		if avail := round(carb-fiber, 3); avail >= 0 {
			n["carbohydrate_available_g"] = avail
			worked = append(worked, "carbohydrate_available_g")
		}
	}
	return worked
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

// category is the food's FoodData Central category: SR Legacy's and Foundation Foods' food
// category, or a survey food's WWEIA category with its number. It's marked inferred: FDC files
// foods under it, but no one measured or declared it for the food.
func category(f fdcFood) *categoryDoc {
	if d := strings.TrimSpace(f.FoodCategory.Description); d != "" {
		return &categoryDoc{Name: d, Scheme: "fdc", Basis: "inferred"}
	}
	d := strings.TrimSpace(f.WweiaFoodCategory.Description)
	if d == "" {
		return nil
	}
	c := &categoryDoc{Name: d, Scheme: "fdc", Basis: "inferred"}
	for _, a := range f.FoodAttributes {
		if a.Name == "WWEIA Category number" {
			c.Code = strings.TrimSpace(a.Value)
		}
	}
	return c
}

// leftOut says why a vocabulary key is missing from some or all foods. A key not listed here and
// missing from a food is one FoodData Central doesn't report for that food.
var leftOut = map[string]string{
	"ala_g":                    "FNDDS survey foods give only 18:3 undifferentiated (619), which counts isomers other than ALA, so survey foods have none.",
	"added_sugars_g":           "None of the three downloads reports added sugars (539); FoodData Central gives them for branded foods only. Survey foods take an estimate from USDA's Food Patterns Equivalents Database instead (see Added sugars, below); SR Legacy and Foundation Foods records have none.",
	"polyols_g":                "None of the three downloads reports sugar alcohols (299).",
	"chloride_mg":              "None of the three downloads reports chlorine (302).",
	"chromium_ug":              "None of the three downloads reports chromium (310).",
	"folate_dfe_ug":            "Foundation Foods give only total folate (417), which equals DFE only when a food has no folic acid, so they have none.",
	"folic_acid_ug":            "Foundation Foods don't report folic acid (431).",
	"biotin_ug":                "Only Foundation Foods measure biotin (416).",
	"iodine_ug":                "Only Foundation Foods measure iodine (314).",
	"molybdenum_ug":            "Only Foundation Foods measure molybdenum (316).",
	"starch_g":                 "Only Foundation Foods and some SR Legacy foods measure starch (209).",
	"carbohydrate_available_g": "Worked out only where both carbohydrate and fibre are given, and not where fibre exceeds the total.",
	"vitamin_d_ug":             "Read in µg (328) only; FoodData Central's IU value (324) comes with it wherever it's given, so no IU is converted.",
	"vitamin_a_rae_ug":         "Read as RAE (320) only; SR Legacy's vitamin A in IU (318) doesn't say the form, so isn't converted.",
}

// nutrientsHeading starts the part of the report that nutrientReport writes, down to its end.
const nutrientsHeading = "\n## Nutrients\n"

// workedOutNote ends the source that names a record's worked-out keys.
const workedOutNote = " worked out by OFL v0 section 4"

// nutrientReport is the report's per-nutrient table, what's missing and why, added sugars and
// attributes, counted from the records themselves: by the source whose numbers a food uses (its
// first), with the worked-out keys read from the source that names them. The build and `ofl
// catalogue refresh` both write it, so the report never needs the downloads to stay true.
func nutrientReport(vocab []vocabEntry, foods []*catalogueFood, sugars sugarStats) string {
	derived, withCategory := map[string]int{}, map[string]int{}
	coverage := map[string]map[string]int{}
	for _, v := range vocab {
		coverage[v.Key] = map[string]int{}
	}
	keyOf := map[string]string{}
	for _, src := range sources {
		keyOf[src.label] = src.key
	}
	for _, food := range foods {
		if len(food.Provenance.Sources) == 0 {
			continue
		}
		label := food.Provenance.Sources[0].Name
		key := keyOf[label]
		for k := range food.NutrientsPer100g {
			if coverage[k] != nil {
				coverage[k][key]++
			}
		}
		if food.Category != nil {
			withCategory[key]++
		}
		for _, s := range food.Provenance.Sources {
			keys, ok := strings.CutSuffix(s.Name, workedOutNote)
			if !ok {
				continue
			}
			for _, k := range strings.Split(strings.TrimPrefix(keys, label+": "), ", ") {
				derived[k]++
			}
		}
	}
	b, total := &strings.Builder{}, len(foods)
	b.WriteString("\n## Nutrients\n\nHow many foods give each key of the vocabulary, by the source whose numbers the food uses. \"Worked out\" counts values OFL v0 section 4 defines from others (`energy_kj` from kcal × 4.184 where FoodData Central gives no kJ, `salt_g` as sodium × 2.5, `carbohydrate_available_g` as carbohydrate − fibre); each such record names them in `provenance.sources`. A food without a key doesn't have it: never zero.\n\n")
	b.WriteString("| Key | FDC number | Foods | Worked out |")
	for _, src := range sources {
		b.WriteString(" " + src.label + " |")
	}
	b.WriteString("\n| --- | --- | --- | --- |" + strings.Repeat(" --- |", len(sources)) + "\n")
	var missing []string
	for _, v := range vocab {
		number := "none"
		if v.FDC != nil {
			number = *v.FDC
		}
		sum := 0
		for _, src := range sources {
			sum += coverage[v.Key][src.key]
		}
		fmt.Fprintf(b, "| `%s` | %s | %d | %d |", v.Key, number, sum, derived[v.Key])
		for _, src := range sources {
			fmt.Fprintf(b, " %d |", coverage[v.Key][src.key])
		}
		b.WriteString("\n")
		if sum < total {
			missing = append(missing, v.Key)
		}
	}
	b.WriteString("\n### Left out, and why\n\nEvery key is read by its FoodData Central number in the vocabulary's unit. Where a food lacks one, FoodData Central doesn't report it for that food. Beyond that:\n\n")
	for _, k := range missing {
		if why, ok := leftOut[k]; ok {
			b.WriteString("- `" + k + "`: " + why + "\n")
		}
	}
	b.WriteString(sugarReport(sugars))
	b.WriteString("\n## Attributes\n\n")
	n := 0
	for _, src := range sources {
		n += withCategory[src.key]
	}
	fmt.Fprintf(b, "%d foods have a `category`: FoodData Central's food category (SR Legacy, Foundation Foods) or WWEIA category with its number (survey foods), scheme `fdc`, basis `inferred`.\n\n", n)
	b.WriteString("No food has `allergens`, `diet_flags`, `ingredients_text`, `glycemic_index` or `glycemic_load`. FoodData Central's generic foods state none of them, and the catalogue has no other CC0 source that does; working them out from names would be guessing.\n")
	return b.String()
}
