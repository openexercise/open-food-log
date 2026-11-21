package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Added sugars aren't in the FoodData Central downloads for generic foods. Survey foods take
// them from USDA's Food Patterns Equivalents Database (FPED) 2021-2023, which gives added sugars
// in teaspoon equivalents per 100 g for every FNDDS 2021-2023 food code.
const (
	// fpedFile holds FPED's ADD_SUGARS column as the workbook gives it, by FNDDS food code.
	fpedFile = "sources/fped-2123-added-sugars.csv"
	// foodCodesFile maps a survey food's FDC ID to its FNDDS food code, which the built
	// catalogue doesn't keep.
	foodCodesFile = "sources/fndds-food-codes.csv"

	fpedSourceName = "USDA Food Patterns Equivalents Database 2021-2023 (ARS Food Surveys Research Group): added_sugars_g from ADD_SUGARS teaspoon equivalents × 4.2 g, an estimate from FNDDS recipes"
	fpedCappedNote = ", capped at sugars_g"
	fpedURL        = "https://www.ars.usda.gov/Services/docs.htm?docid=24615"
	fpedRetrieved  = "2026-11-21"
	// fpedLicense is the US government's public domain label, which data.gov lists FPED under.
	// There's no SPDX identifier for it, and USDA doesn't publish FPED as CC0 as it does
	// FoodData Central.
	fpedLicense = "https://www.usa.gov/publicdomain/label/1.0/"

	// gramsPerTeaspoon is USDA's own factor: "Added sugars (g) / 4.2 = teaspoon equivalents of
	// added sugars" (FPED 2021-2023 documentation, p. 31).
	gramsPerTeaspoon = 4.2
	// addedSugarsSlack is how far the estimate may sit above a food's total sugars and still be
	// read as that total: 2.1 g, the half teaspoon FPED rounds some multi-part ingredients to.
	addedSugarsSlack = 2.1

	foodCodesHeader = `# FNDDS 2021-2023 food codes for FoodData Central's survey foods, so the catalogue's survey
# records (identifiers.fdc_id) can be joined to sources that go by food code.
# Written by "ofl catalogue added-sugars <dir>" (and by "ofl catalogue <dir>") from fdcId and
# foodCode in surveyDownload.json, FoodData_Central_survey_food_json_2024-10-31.zip,
# https://fdc.nal.usda.gov/download-datasets (public domain, CC0 1.0). Don't edit by hand.
`
)

// foodCode is a survey food's FNDDS food code. The download writes it as a string; a number is
// read the same way.
type foodCode string

func (c *foodCode) UnmarshalJSON(b []byte) error {
	s := string(bytes.TrimSpace(b))
	if s == "null" {
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*c = foodCode(strings.TrimSpace(v))
		return nil
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil || n < 0 || n != float64(int64(n)) {
		return fmt.Errorf("food code %s is neither a string nor a whole number", s)
	}
	*c = foodCode(strconv.FormatInt(int64(n), 10))
	return nil
}

// addedSugarsGrams turns FPED's teaspoon equivalents into grams, to one decimal place as FPED
// rounds its own amounts.
func addedSugarsGrams(teaspoons float64) float64 {
	return round(teaspoons*gramsPerTeaspoon, 1)
}

// addedSugarsFor is the value a food with these total sugars takes from an FPED amount. An
// estimate above the total by no more than addedSugarsSlack is read as the total (capped); one
// further above, or a food with no total to hold it to, gets none.
func addedSugarsFor(teaspoons float64, sugars float64, hasSugars bool) (grams float64, capped, ok bool) {
	if !hasSugars {
		return 0, false, false
	}
	grams = addedSugarsGrams(teaspoons)
	if grams <= sugars {
		return grams, false, true
	}
	if round(grams-sugars, 3) <= addedSugarsSlack {
		return sugars, true, true
	}
	return 0, false, false
}

// sugarStats counts what addSugars did, for the report.
type sugarStats struct {
	rows       int // FPED rows
	unusedRows int // FPED rows naming no survey food whose numbers the catalogue uses
	survey     int // foods whose numbers are the survey source's
	filled     int // of those, with added_sugars_g
	zero       int // of the filled, a stated 0
	capped     int // of the filled, read as sugars_g
	noRow      int // no food code, or no FPED row for it
	noSugars   int // an FPED row, but no sugars_g to hold it to
	above      int // more than addedSugarsSlack above sugars_g
}

func isFPED(s sourceDoc) bool { return strings.HasPrefix(s.Name, fpedSourceName) }

// isSurveyFood says a record's numbers are the survey source's. A survey food whose name a
// Foundation Foods record took first is listed in that record's sources, but its numbers aren't
// used, so FPED's recipe estimate doesn't describe it.
func isSurveyFood(food *catalogueFood) bool {
	return len(food.Provenance.Sources) > 0 && food.Provenance.Sources[0].Name == sourceLabel("survey")
}

func sourceLabel(key string) string {
	for _, src := range sources {
		if src.key == key {
			return src.label
		}
	}
	return ""
}

// addSugars gives every survey food its added sugars from FPED and names FPED in its sources,
// after taking away what an earlier run gave, so running it again changes nothing. codes maps a
// survey food's FDC ID to its food code; fped maps a food code to teaspoon equivalents per 100 g.
// The build and `ofl catalogue refresh` both call it, so they give the same records.
func addSugars(foods []*catalogueFood, codes map[int]string, fped map[string]float64) sugarStats {
	stats := sugarStats{rows: len(fped)}
	used := map[string]bool{}
	for _, food := range foods {
		dropSugars(food)
		if !isSurveyFood(food) {
			continue
		}
		stats.survey++
		// FoodData Central's own value, should a download ever give one, wins.
		if _, has := food.NutrientsPer100g["added_sugars_g"]; has {
			stats.filled++
			continue
		}
		code, hasCode := codes[food.Identifiers["fdc_id"]]
		teaspoons, hasRow := fped[code]
		if !hasCode || !hasRow {
			stats.noRow++
			continue
		}
		used[code] = true
		sugars, hasSugars := food.NutrientsPer100g["sugars_g"]
		grams, capped, ok := addedSugarsFor(teaspoons, sugars, hasSugars)
		switch {
		case !hasSugars:
			stats.noSugars++
			continue
		case !ok:
			stats.above++
			continue
		}
		food.NutrientsPer100g["added_sugars_g"] = grams
		name := fpedSourceName
		if capped {
			name += fpedCappedNote
			stats.capped++
		}
		if grams == 0 {
			stats.zero++
		}
		stats.filled++
		food.Provenance.Sources = append(food.Provenance.Sources, sourceDoc{Name: name, ID: code, URL: fpedURL, License: fpedLicense, RetrievedAt: fpedRetrieved})
	}
	stats.unusedRows = len(fped) - len(used)
	return stats
}

// dropSugars takes away the added sugars and the source an earlier addSugars gave a food.
func dropSugars(food *catalogueFood) {
	var kept []sourceDoc
	for _, s := range food.Provenance.Sources {
		if !isFPED(s) {
			kept = append(kept, s)
		}
	}
	if len(kept) == len(food.Provenance.Sources) {
		return
	}
	food.Provenance.Sources = kept
	delete(food.NutrientsPer100g, "added_sugars_g")
}

// readFPED reads fpedFile: a food code and FPED's teaspoon equivalents of added sugars per 100 g.
func readFPED(root string) (map[string]float64, error) {
	rows, err := readCSV(filepath.Join(root, fpedFile), "food_code", "add_sugars_tsp_eq")
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, row := range rows {
		v, err := strconv.ParseFloat(row[1], 64)
		if err != nil || v < 0 {
			return nil, fmt.Errorf("%s: food code %s has %q teaspoons", fpedFile, row[0], row[1])
		}
		if _, twice := out[row[0]]; twice {
			return nil, fmt.Errorf("%s: food code %s twice", fpedFile, row[0])
		}
		out[row[0]] = v
	}
	return out, nil
}

// readFoodCodes reads foodCodesFile: a survey food's FDC ID and its food code.
func readFoodCodes(root string) (map[int]string, error) {
	rows, err := readCSV(filepath.Join(root, foodCodesFile), "fdc_id", "food_code")
	if err != nil {
		return nil, err
	}
	out := map[int]string{}
	for _, row := range rows {
		id, err := strconv.Atoi(row[0])
		if err != nil {
			return nil, fmt.Errorf("%s: FDC ID %q", foodCodesFile, row[0])
		}
		out[id] = row[1]
	}
	return out, nil
}

// readCSV reads a two-column file whose first row names its columns, skipping lines that start
// with #.
func readCSV(path string, first, second string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comment = '#'
	r.FieldsPerRecord = 2
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if len(rows) == 0 || rows[0][0] != first || rows[0][1] != second {
		return nil, fmt.Errorf("%s: want columns %s,%s", path, first, second)
	}
	return rows[1:], nil
}

// writeFoodCodes writes foodCodesFile from the survey download's foods, by FDC ID.
func writeFoodCodes(root string, foods []fdcFood) error {
	// The build reads foods in the download's order, so the order is left as it is.
	byID := make([]fdcFood, len(foods))
	copy(byID, foods)
	sort.Slice(byID, func(i, j int) bool { return byID[i].FdcID < byID[j].FdcID })
	var b strings.Builder
	b.WriteString(foodCodesHeader + "fdc_id,food_code\n")
	for _, f := range byID {
		if f.FoodCode == "" {
			continue
		}
		fmt.Fprintf(&b, "%d,%s\n", f.FdcID, f.FoodCode)
	}
	return os.WriteFile(filepath.Join(root, foodCodesFile), []byte(b.String()), 0o644)
}

// writeAddedSugars is `ofl catalogue added-sugars <dir>`: it reads the survey download in dir
// for its food codes, keeps them in sources/, and refreshes the catalogue, which gives survey
// foods their added sugars. It needs neither of the other two downloads.
func writeAddedSugars(dir, root string) error {
	var survey source
	for _, src := range sources {
		if src.key == "survey" {
			survey = src
		}
	}
	foods, err := readSource(filepath.Join(dir, survey.file), survey.field)
	if err != nil {
		return err
	}
	if len(foods) == 0 {
		return errors.New(survey.file + " holds no survey foods")
	}
	if err := writeFoodCodes(root, foods); err != nil {
		return err
	}
	return writeRefreshedCatalogue(root)
}

// sugarReport is REPORT.md's section on added sugars.
func sugarReport(s sugarStats) string {
	var b strings.Builder
	b.WriteString("\n### Added sugars\n\n")
	fmt.Fprintf(&b, "`added_sugars_g` isn't FoodData Central's. Survey foods take it from USDA's Food Patterns Equivalents Database 2021-2023 (`%s`, %d food codes): ADD_SUGARS teaspoon equivalents per 100 g × %s g, to one decimal place, joined by FNDDS food code (`%s`). It's an estimate from FNDDS recipes; each record that has it names the database in `provenance.sources`. SR Legacy and Foundation Foods records have none.\n\n", fpedFile, s.rows, strconv.FormatFloat(gramsPerTeaspoon, 'f', -1, 64), foodCodesFile)
	slack := strconv.FormatFloat(addedSugarsSlack, 'f', -1, 64)
	b.WriteString("| Survey foods | |\n| --- | --- |\n")
	fmt.Fprintf(&b, "| Whose numbers the catalogue uses | %d |\n", s.survey)
	fmt.Fprintf(&b, "| With `added_sugars_g` | %d |\n", s.filled)
	fmt.Fprintf(&b, "| Of those, a stated 0 | %d |\n", s.zero)
	fmt.Fprintf(&b, "| Of those, capped at `sugars_g` (the estimate was above the food's total sugars by %s g or less) | %d |\n", slack, s.capped)
	fmt.Fprintf(&b, "| Left without: the estimate is more than %s g above `sugars_g` | %d |\n", slack, s.above)
	fmt.Fprintf(&b, "| Left without: no `sugars_g` to hold the estimate to | %d |\n", s.noSugars)
	fmt.Fprintf(&b, "| Left without: no row in the database for the food code | %d |\n", s.noRow)
	fmt.Fprintf(&b, "\n%d of the database's rows name no survey food whose numbers the catalogue uses: a food left out, or one whose name a Foundation Foods record took first.\n", s.unusedRows)
	return b.String()
}
