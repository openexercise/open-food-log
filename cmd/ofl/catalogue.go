package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// A source is one FoodData Central download. Earlier sources win when two describe a food by
// the same name: Foundation Foods are the newest laboratory data, survey foods (FNDDS) carry the
// portions people report, and SR Legacy (2018) fills the rest.
type source struct {
	key, file, field, label string
}

var sources = []source{
	{"foundation", "FoodData_Central_foundation_food_json_2026-10-29.json", "FoundationFoods", "USDA FoodData Central, Foundation Foods (2026-10-29)"},
	{"survey", "surveyDownload.json", "SurveyFoods", "USDA FoodData Central, FNDDS survey foods (2024-10-31)"},
	{"sr", "FoodData_Central_sr_legacy_food_json_2018-04.json", "SRLegacyFoods", "USDA FoodData Central, SR Legacy (2018-04)"},
}

// fallbacks are nutrient numbers read when a food lacks the vocabulary's own: Foundation Foods
// often report energy only by Atwater factors (958 specific, 957 general), fat as NLEA total fat,
// carbohydrate by summation, sugars and fibre by newer methods.
var fallbacks = map[string][]string{
	"energy_kcal":    {"958", "957"},
	"fat_g":          {"298"},
	"carbohydrate_g": {"205.2"},
	"sugars_g":       {"269.3"},
	"fiber_g":        {"293"},
}

type fdcFood struct {
	FdcID         int    `json:"fdcId"`
	Description   string `json:"description"`
	FoodNutrients []struct {
		Nutrient struct {
			Number   string `json:"number"`
			UnitName string `json:"unitName"`
		} `json:"nutrient"`
		Amount *float64 `json:"amount"`
	} `json:"foodNutrients"`
	FoodPortions []struct {
		Amount      *float64 `json:"amount"`
		MeasureUnit struct {
			Name string `json:"name"`
		} `json:"measureUnit"`
		Modifier           string  `json:"modifier"`
		PortionDescription string  `json:"portionDescription"`
		GramWeight         float64 `json:"gramWeight"`
	} `json:"foodPortions"`
}

type catalogueFood struct {
	Schema           string             `json:"$schema"`
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Kind             string             `json:"kind"`
	Aliases          []aliasDoc         `json:"aliases,omitempty"`
	NutrientsPer100g map[string]float64 `json:"nutrients_per_100g"`
	Portions         []portionDoc       `json:"portions,omitempty"`
	Identifiers      map[string]int     `json:"identifiers"`
	Provenance       provenanceDoc      `json:"provenance"`
}

type portionDoc struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Grams     float64 `json:"grams"`
	IsDefault bool    `json:"is_default,omitempty"`
}

type provenanceDoc struct {
	Sources   []sourceDoc `json:"sources"`
	License   string      `json:"license"`
	Generated []string    `json:"generated,omitempty"`
	Review    string      `json:"review"`
}

type sourceDoc struct {
	Name        string `json:"name"`
	ID          string `json:"id,omitempty"`
	URL         string `json:"url,omitempty"`
	License     string `json:"license,omitempty"`
	RetrievedAt string `json:"retrieved_at,omitempty"`
}

const retrieved = "2026-11-09"

var (
	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
	// brandWords are runs of capitals: SR Legacy names its branded and chain-restaurant records
	// that way (KRAFT, ON THE BORDER, McDONALD'S). Branded foods come by barcode instead.
	brandWords = regexp.MustCompile(`[A-Z]{3,}`)
	// notBrands are capitalised words SR Legacy uses for generic foods.
	notBrands = map[string]bool{"USDA": true, "BBQ": true, "UHT": true, "NFS": true, "RTE": true, "NFSMI": true}
)

func isBranded(name string) bool {
	for _, w := range brandWords.FindAllString(name, -1) {
		if !notBrands[w] {
			return true
		}
	}
	return false
}

func slug(s string) string {
	return strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(s), "-"), "-")
}

// buildCatalogue reads the FoodData Central downloads in dir and writes catalogue/foods.jsonl
// and catalogue/REPORT.md under root.
func buildCatalogue(dir, root string) error {
	vocab, err := readVocabulary(root)
	if err != nil {
		return err
	}
	byID := map[string]*catalogueFood{}
	type counts struct{ read, kept, branded, empty, noEnergy, merged, noPortions int }
	report := map[string]*counts{}
	for _, src := range sources {
		foods, err := readSource(filepath.Join(dir, src.file), src.field)
		if err != nil {
			return err
		}
		c := &counts{read: len(foods)}
		report[src.key] = c
		for _, f := range foods {
			if src.key == "sr" && isBranded(f.Description) {
				c.branded++
				continue
			}
			nutrients := mapNutrients(f, vocab)
			if len(nutrients) == 0 {
				c.empty++
				continue
			}
			// Energy is what people log food for; a record without it is left out rather than
			// given a number its source doesn't state.
			if _, ok := nutrients["energy_kcal"]; !ok {
				c.noEnergy++
				continue
			}
			id := "ofc:" + slug(f.Description)
			sd := sourceDoc{Name: src.label, ID: strconv.Itoa(f.FdcID), URL: fmt.Sprintf("https://fdc.nal.usda.gov/food-details/%d/nutrients", f.FdcID), License: "CC0-1.0", RetrievedAt: retrieved}
			if existing, ok := byID[id]; ok {
				// The same name from a later source: noted, not merged into the numbers.
				existing.Provenance.Sources = append(existing.Provenance.Sources, sd)
				c.merged++
				continue
			}
			food := &catalogueFood{
				Schema: "https://openfoodlog.org/schema/v0/food.json", ID: id, Name: f.Description, Kind: "generic",
				NutrientsPer100g: nutrients, Portions: portions(f),
				Identifiers: map[string]int{"fdc_id": f.FdcID},
				Provenance:  provenanceDoc{Sources: []sourceDoc{sd}, License: "CC0-1.0", Review: "unreviewed"},
			}
			if len(food.Portions) == 0 {
				c.noPortions++
			}
			byID[id] = food
			c.kept++
		}
	}

	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	model, err := readModelAliases(root)
	if err != nil {
		return err
	}
	var ruleCount, modelCount, withAliases int
	for _, id := range ids {
		r, g := attachAliases(byID[id], model)
		ruleCount += r
		modelCount += g
		if r+g > 0 {
			withAliases++
		}
	}
	out := filepath.Join(root, "catalogue")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(out, "foods.jsonl"))
	if err != nil {
		return err
	}
	w := bufio.NewWriter(file)
	for _, id := range ids {
		line, err := json.Marshal(byID[id])
		if err != nil {
			return err
		}
		w.Write(line)
		w.WriteByte('\n')
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# Catalogue report\n\nWhat `ofl catalogue` did with the FoodData Central downloads, retrieved " + retrieved + ".\n\n")
	b.WriteString("| Source | Read | Kept | Same name as an earlier source | Branded, left out | No nutrients, left out | No energy, left out | Kept without portions |\n| --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, src := range sources {
		c := report[src.key]
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d | %d | %d |\n", src.label, c.read, c.kept, c.merged, c.branded, c.empty, c.noEnergy, c.noPortions)
	}
	fmt.Fprintf(&b, "\n%d foods in `foods.jsonl`, %d with aliases: %d from the names' own shape, %d suggested by a model (listed in each record's `provenance.generated`).\n", len(ids), withAliases, ruleCount, modelCount)
	if err := os.WriteFile(filepath.Join(out, "REPORT.md"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("catalogue: %d foods\n", len(ids))
	return nil
}

func readSource(path, field string) ([]fdcFood, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var foods []fdcFood
	if err := json.Unmarshal(doc[field], &foods); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return foods, nil
}

type vocabEntry struct {
	Key  string  `json:"key"`
	Unit string  `json:"unit"`
	FDC  *string `json:"fdc_nutrient_number"`
}

func readVocabulary(root string) ([]vocabEntry, error) {
	raw, err := os.ReadFile(filepath.Join(root, "schema", "v0", "nutrients.json"))
	if err != nil {
		return nil, err
	}
	var v struct {
		Nutrients []vocabEntry `json:"nutrients"`
	}
	return v.Nutrients, json.Unmarshal(raw, &v)
}

// mapNutrients reads each vocabulary nutrient by its FoodData Central number, then its fallbacks,
// keeping only amounts reported in the vocabulary's unit. Nothing missing is filled in.
func mapNutrients(f fdcFood, vocab []vocabEntry) map[string]float64 {
	have := map[string]float64{}
	units := map[string]string{}
	for _, n := range f.FoodNutrients {
		if n.Amount == nil || math.IsNaN(*n.Amount) || *n.Amount < 0 {
			continue
		}
		have[n.Nutrient.Number] = *n.Amount
		units[n.Nutrient.Number] = strings.ToLower(n.Nutrient.UnitName)
	}
	out := map[string]float64{}
	for _, v := range vocab {
		if v.FDC == nil {
			continue
		}
		for _, number := range append([]string{*v.FDC}, fallbacks[v.Key]...) {
			if amount, ok := have[number]; ok && units[number] == v.Unit {
				out[v.Key] = amount
				break
			}
		}
	}
	return out
}

// portions turns FoodData Central's measures into named portions. A survey food's "Quantity not
// specified" is the amount to assume when a person names the food without one, so it's the default.
func portions(f fdcFood) []portionDoc {
	var out []portionDoc
	seen := map[string]int{}
	for _, p := range f.FoodPortions {
		if p.GramWeight <= 0 {
			continue
		}
		label, isDefault := "", false
		switch {
		case p.PortionDescription != "" && p.PortionDescription != "Quantity not specified":
			if strings.HasPrefix(p.PortionDescription, "Guideline amount") {
				continue
			}
			label = p.PortionDescription
		case p.PortionDescription == "Quantity not specified":
			label, isDefault = "Quantity not specified", true
		default:
			amount := 1.0
			if p.Amount != nil && *p.Amount > 0 {
				amount = *p.Amount
			}
			unit := p.MeasureUnit.Name
			switch unit {
			case "undetermined", "":
				unit = ""
			case "RACC":
				unit = "serving (RACC)"
			}
			label = strings.TrimSpace(strings.Join(strings.Fields(strconv.FormatFloat(amount, 'f', -1, 64)+" "+unit), " "))
			if p.Modifier != "" {
				if unit == "" {
					label += " " + p.Modifier
				} else {
					label += ", " + p.Modifier
				}
			}
		}
		id := slug(label)
		if id == "" {
			continue
		}
		if n := seen[id]; n > 0 {
			seen[id]++
			id = fmt.Sprintf("%s-%d", id, n+1)
		} else {
			seen[id] = 1
		}
		out = append(out, portionDoc{ID: id, Label: label, Grams: p.GramWeight, IsDefault: isDefault})
	}
	return out
}
