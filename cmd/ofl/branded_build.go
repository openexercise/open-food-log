package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	openfoodlog "github.com/openexercise/open-food-log"
)

// The branded catalogue: `ofl catalogue branded` reads USDA's Global Branded Food Products
// Database by the rules the branded report measures (branded.go) and writes one OFL food per kept
// product to branded-foods.jsonl.gz, ordered by ID. The file is a release asset, never committed.

// brandedFoodDoc is one record of branded-foods.jsonl.gz, an OFL food.
type brandedFoodDoc struct {
	Schema            string             `json:"$schema"`
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	Kind              string             `json:"kind"`
	Brand             string             `json:"brand,omitempty"`
	GTIN              []string           `json:"gtin"`
	NutrientsBasis    string             `json:"nutrients_basis,omitempty"`
	NutrientsPer100g  map[string]float64 `json:"nutrients_per_100g"`
	NutrientsPer100ml map[string]float64 `json:"nutrients_per_100ml,omitempty"`
	Label             *labelDoc          `json:"label,omitempty"`
	Category          *categoryDoc       `json:"category,omitempty"`
	IngredientsText   *ingredientsDoc    `json:"ingredients_text,omitempty"`
	Identifiers       map[string]int     `json:"identifiers"`
	Provenance        provenanceDoc      `json:"provenance"`
}

type labelDoc struct {
	Region              string             `json:"region"`
	Serving             *servingDoc        `json:"serving,omitempty"`
	NutrientsPerServing map[string]float64 `json:"nutrients_per_serving,omitempty"`
}

type servingDoc struct {
	Text  string  `json:"text"`
	Grams float64 `json:"grams,omitempty"`
	ML    float64 `json:"ml,omitempty"`
}

type ingredientsDoc struct {
	Text   string `json:"text"`
	Locale string `json:"locale"`
	Basis  string `json:"basis"`
}

// labelKeys maps FoodData Central's labelNutrients onto the vocabulary, each in the unit a US
// label declares it in. vitaminD is left out: older records give it in IU, newer ones in µg, and
// the record doesn't say which.
var labelKeys = map[string]string{
	"calories":      "energy_kcal",
	"protein":       "protein_g",
	"fat":           "fat_g",
	"saturatedFat":  "saturated_fat_g",
	"transFat":      "trans_fat_g",
	"cholesterol":   "cholesterol_mg",
	"sodium":        "sodium_mg",
	"carbohydrates": "carbohydrate_g",
	"fiber":         "fiber_g",
	"sugars":        "sugars_g",
	"addedSugar":    "added_sugars_g",
	"calcium":       "calcium_mg",
	"iron":          "iron_mg",
	"potassium":     "potassium_mg",
}

// dataSources names FoodData Central's dataSource codes.
var dataSources = map[string]string{
	"LI":          "Label Insight",
	"GDSN":        "GS1's GDSN",
	"Euromonitor": "Euromonitor",
}

// isoDate gives an M/D/YYYY date as YYYY-MM-DD, or "".
func isoDate(s string) string {
	if t, ok := fdcDate(s); ok {
		return t.Format("2006-01-02")
	}
	return ""
}

// cleanText keeps a label's text on one line, without control characters.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case unicode.IsControl(r) || r == unicode.ReplacementChar:
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// number formats a serving size for its text: 28, 2.5.
func number(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// brandedRecord is the OFL food for a kept product: its nutrients (n, read by mapNutrients) on the
// basis basis gives, the label's serving and per-serving values, and provenance naming the release,
// the FDC record, the data provider and the record's dates.
func brandedRecord(b brandedFood, gtin string, n map[string]float64, release string) brandedFoodDoc {
	name := cleanText(b.Description)
	brand := cleanText(b.BrandName)
	if brand == "" {
		brand = cleanText(b.BrandOwner)
	}
	if name == "" {
		name = brand
	}
	if name == "" {
		name = "Product " + gtin
	}
	cased := sentenceCase(name)
	rec := brandedFoodDoc{
		Schema: "https://openfoodlog.org/schema/v0/food.json", ID: "gtin:" + gtin, Name: cased, Kind: "branded",
		Brand: brand, GTIN: []string{gtin},
		Identifiers: map[string]int{"fdc_id": b.FdcID},
	}
	bs, _ := basis(b, n)
	if bs == "100ml" {
		rec.NutrientsBasis = "100ml"
		rec.NutrientsPer100ml = n
		rec.NutrientsPer100g = map[string]float64{} // can't be weighed: no density
	} else {
		rec.NutrientsPer100g = n
	}

	label := &labelDoc{Region: "us"}
	if b.ServingSize != nil && *b.ServingSize > 0 {
		s := &servingDoc{Text: cleanText(b.HouseholdServingFullText)}
		unit := "g"
		if bs == "100ml" {
			s.ML, unit = *b.ServingSize, "ml"
		} else {
			s.Grams = *b.ServingSize
		}
		if s.Text == "" {
			s.Text = number(*b.ServingSize) + " " + unit
		}
		label.Serving = s
	}
	per := map[string]float64{}
	for k, v := range b.LabelNutrients {
		key, ok := labelKeys[k]
		if ok && v.Value != nil && *v.Value >= 0 && !math.IsNaN(*v.Value) && !math.IsInf(*v.Value, 0) {
			per[key] = *v.Value
		}
	}
	if len(per) > 0 {
		label.NutrientsPerServing = per
	}
	if label.Serving != nil || label.NutrientsPerServing != nil {
		rec.Label = label
	}

	if c := cleanText(b.BrandedFoodCategory); c != "" {
		rec.Category = &categoryDoc{Name: c, Scheme: "fdc", Basis: "inferred"}
	}
	if ingr := cleanText(b.Ingredients); ingr != "" {
		rec.IngredientsText = &ingredientsDoc{Text: ingr, Locale: "en", Basis: "declared"}
	}

	provider := cleanText(b.BrandOwner)
	if provider == "" {
		provider = brand
	}
	via := dataSources[strings.TrimSpace(b.DataSource)]
	if via == "" {
		via = strings.TrimSpace(b.DataSource)
	}
	detail := []string{}
	if provider != "" {
		detail = append(detail, "data provider "+provider)
	}
	if via != "" {
		detail = append(detail, "through "+via)
	}
	for _, d := range []struct{ what, date string }{
		{"available", b.AvailableDate}, {"modified", b.ModifiedDate}, {"published", b.PublicationDate},
	} {
		if iso := isoDate(d.date); iso != "" {
			detail = append(detail, d.what+" "+iso)
		}
	}
	if cased != name {
		detail = append(detail, "described "+strconv.Quote(name))
	}
	label0 := "USDA FoodData Central, Global Branded Food Products Database (" + release + ")"
	if len(detail) > 0 {
		label0 += ": " + strings.Join(detail, ", ")
	}
	rec.Provenance = provenanceDoc{
		Sources: []sourceDoc{{
			Name: label0, ID: strconv.Itoa(b.FdcID), License: "CC0-1.0",
			URL: fmt.Sprintf("https://fdc.nal.usda.gov/food-details/%d/nutrients", b.FdcID),
		}},
		License: "CC0-1.0", Review: "unreviewed",
	}
	return rec
}

// marshalPlain encodes v as one line of JSON without escaping &, < and > (labels name "Chips,
// Pretzels & Snacks"; nothing here is put in HTML).
func marshalPlain(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// vocabularyFS reads the nutrient vocabulary from the embedded schemas, so the command runs
// outside a checkout.
func vocabularyFS() ([]vocabEntry, error) {
	raw, err := fs.ReadFile(openfoodlog.Schemas, "schema/v0/nutrients.json")
	if err != nil {
		return nil, err
	}
	var v struct {
		Nutrients []vocabEntry `json:"nutrients"`
	}
	return v.Nutrients, json.Unmarshal(raw, &v)
}

// keptVersion is what pass 1 keeps of a product's record to choose the newest.
type keptVersion struct {
	fdcID                       int
	availableDate, modifiedDate string
}

// brandedResult is what a build wrote.
type brandedResult struct {
	Records int
	Bytes   int64
	SHA256  string
}

// buildBranded reads the Branded Foods JSON at path and writes the branded catalogue to out
// (gzipped JSONL, by ID) and its checksum to out + ".sha256". Every record is validated against
// the food schema; one that fails stops the build.
func buildBranded(path, out string) (brandedResult, error) {
	var res brandedResult
	m := releaseInName.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return res, errors.New("want the unzipped download, named FoodData_Central_branded_food_json_YYYY-MM-DD.json")
	}
	release := m[1]
	vocab, err := vocabularyFS()
	if err != nil {
		return res, err
	}
	v, err := openfoodlog.NewValidator()
	if err != nil {
		return res, err
	}

	// Pass 1: the US records with a GTIN, and the newest record of each GTIN.
	best := map[string]keptVersion{}
	err = eachBranded(path, func(raw json.RawMessage) error {
		var b brandedFood
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		if !isUS(b.MarketCountry) {
			return nil
		}
		gtin, why := normalizeGTIN(b.GtinUpc)
		if why != "" {
			return nil
		}
		cur, ok := best[gtin]
		if !ok || newer(b, brandedFood{FdcID: cur.fdcID, AvailableDate: cur.availableDate, ModifiedDate: cur.modifiedDate}) {
			best[gtin] = keptVersion{b.FdcID, b.AvailableDate, b.ModifiedDate}
		}
		return nil
	})
	if err != nil {
		return res, err
	}
	keep := make(map[int]string, len(best))
	for gtin, k := range best {
		keep[k.fdcID] = gtin
	}
	best = nil

	// Pass 2: the kept version of each product, through the remaining rules, encoded.
	lines := map[string][]byte{}
	err = eachBranded(path, func(raw json.RawMessage) error {
		var b brandedFood
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		gtin, ok := keep[b.FdcID]
		if !ok || strings.TrimSpace(b.DiscontinuedDate) != "" {
			return nil
		}
		n := mapNutrients(b.asFDC(), vocab)
		if implausible(b, n) != "" {
			return nil
		}
		line, err := marshalPlain(brandedRecord(b, gtin, n, release))
		if err != nil {
			return err
		}
		if problems := v.ValidateAs("food", line); problems != nil {
			return fmt.Errorf("fdcId %d: %s", b.FdcID, strings.Join(problems, "; "))
		}
		lines[gtin] = line
		return nil
	})
	if err != nil {
		return res, err
	}

	gtins := make([]string, 0, len(lines))
	for g := range lines {
		gtins = append(gtins, g)
	}
	sort.Strings(gtins)
	f, err := os.Create(out)
	if err != nil {
		return res, err
	}
	defer f.Close()
	sum := sha256.New()
	counted := &countingWriter{}
	gz, err := gzip.NewWriterLevel(io.MultiWriter(f, sum, counted), gzip.BestCompression)
	if err != nil {
		return res, err
	}
	w := bufio.NewWriterSize(gz, 1<<20)
	for _, g := range gtins {
		if _, err := w.Write(append(lines[g], '\n')); err != nil {
			return res, err
		}
	}
	if err := w.Flush(); err != nil {
		return res, err
	}
	if err := gz.Close(); err != nil {
		return res, err
	}
	if err := f.Close(); err != nil {
		return res, err
	}
	res = brandedResult{Records: len(gtins), Bytes: counted.n, SHA256: hex.EncodeToString(sum.Sum(nil))}
	check := fmt.Sprintf("%s  %s\n", res.SHA256, filepath.Base(out))
	if err := os.WriteFile(out+".sha256", []byte(check), 0o644); err != nil {
		return res, err
	}
	return res, nil
}

// runBranded is `ofl catalogue branded <download.json> [out]`.
func runBranded(args []string) error {
	if len(args) < 1 || len(args) > 2 {
		return errors.New("usage: ofl catalogue branded <FoodData_Central_branded_food_json_YYYY-MM-DD.json> [branded-foods.jsonl.gz]")
	}
	out := "branded-foods.jsonl.gz"
	if len(args) == 2 {
		out = args[1]
	}
	start := time.Now()
	res, err := buildBranded(args[0], out)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d records, %d bytes (%.1f MB), sha256 %s, in %s\n", out, res.Records, res.Bytes, float64(res.Bytes)/1e6, res.SHA256, time.Since(start).Round(time.Second))
	return nil
}
