package main

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// The branded report measures USDA's Global Branded Food Products Database against the filtering
// rules planned for a branded catalogue (Lifterclub's docs/fdc-branded.md, phase 0). It reads the
// download twice, counts what each rule drops, and writes catalogue/BRANDED-REPORT.md. It writes
// no records: the sizes it reports come from encoding a draft record for each kept product and
// throwing the bytes away.

// brandedFood is one record of the Branded Foods JSON download.
type brandedFood struct {
	FdcID                    int      `json:"fdcId"`
	Description              string   `json:"description"`
	BrandOwner               string   `json:"brandOwner"`
	BrandName                string   `json:"brandName"`
	SubbrandName             string   `json:"subbrandName"`
	GtinUpc                  string   `json:"gtinUpc"`
	Ingredients              string   `json:"ingredients"`
	ServingSize              *float64 `json:"servingSize"`
	ServingSizeUnit          string   `json:"servingSizeUnit"`
	HouseholdServingFullText string   `json:"householdServingFullText"`
	PackageWeight            string   `json:"packageWeight"`
	BrandedFoodCategory      string   `json:"brandedFoodCategory"`
	DataSource               string   `json:"dataSource"`
	MarketCountry            string   `json:"marketCountry"`
	ModifiedDate             string   `json:"modifiedDate"`
	AvailableDate            string   `json:"availableDate"`
	PublicationDate          string   `json:"publicationDate"`
	DiscontinuedDate         string   `json:"discontinuedDate"`
	LabelNutrients           map[string]struct {
		Value *float64 `json:"value"`
	} `json:"labelNutrients"`
	FoodNutrients []struct {
		Nutrient struct {
			Number   string `json:"number"`
			UnitName string `json:"unitName"`
		} `json:"nutrient"`
		Amount *float64 `json:"amount"`
	} `json:"foodNutrients"`
}

// asFDC gives the record's nutrients in the shape mapNutrients reads.
func (b brandedFood) asFDC() fdcFood { return fdcFood{FoodNutrients: b.FoodNutrients} }

// fdcDate reads FoodData Central's M/D/YYYY dates.
func fdcDate(s string) (time.Time, bool) {
	t, err := time.Parse("1/2/2006", strings.TrimSpace(s))
	return t, err == nil
}

// normalizeGTIN reads a GTIN as 14 digits with a valid check digit. A code shorter than 14 digits
// is zero-padded on the left, as Lifterclub's NormalizeGTIN pads 8, 12 and 13: the download also
// gives codes that lost leading zeros (an 11-digit UPC-A), and padding gives the same 14 digits a
// scan of the package does. The reason names why a code isn't one.
func normalizeGTIN(code string) (gtin, reason string) {
	code = strings.TrimSpace(code)
	switch {
	case code == "":
		return "", "none"
	case strings.Trim(code, "0123456789") != "":
		return "", "not digits"
	case len(code) > 14:
		return "", "over 14 digits"
	}
	code = strings.Repeat("0", 14-len(code)) + code
	sum := 0
	for i := 0; i < 13; i++ {
		d := int(code[i] - '0')
		if i%2 == 0 {
			d *= 3
		}
		sum += d
	}
	if (10-sum%10)%10 != int(code[13]-'0') {
		return "", "check digit"
	}
	return code, ""
}

// isPadded says whether a code is a length a scanner doesn't read as it is (not 8, 12, 13 or 14).
func isPadded(code string) bool {
	switch len(strings.TrimSpace(code)) {
	case 8, 12, 13, 14:
		return false
	}
	return true
}

// newer says whether a is the version of a product to keep over b: the later available date, then
// the later modified date, then the higher FDC ID.
func newer(a, b brandedFood) bool {
	for _, pair := range [][2]string{{a.AvailableDate, b.AvailableDate}, {a.ModifiedDate, b.ModifiedDate}} {
		ta, oka := fdcDate(pair[0])
		tb, okb := fdcDate(pair[1])
		switch {
		case oka && !okb:
			return true
		case okb && !oka:
			return false
		case oka && okb && !ta.Equal(tb):
			return ta.After(tb)
		}
	}
	return a.FdcID > b.FdcID
}

// isUS says whether a record is for the US market: the download writes it "United States" or "US".
func isUS(market string) bool {
	m := strings.TrimSpace(market)
	return m == "United States" || m == "US"
}

// unitBasis is the nutrient basis a serving unit names: FoodData Central gives values per 100 g or
// per 100 ml, "depending on which was received from the data provider", by the serving's unit.
func unitBasis(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "ml", "mlt":
		return "100ml"
	case "g", "grm", "gm":
		return "100g"
	}
	return ""
}

var (
	liquidWords = regexp.MustCompile(`(?i)\bfl\.?\s*oz\b|\bml\b|\blit(er|re)s?\b|\d\s*l\b`)
	solidWords  = regexp.MustCompile(`(?i)\d\s*(g|kg|lbs?)\b`)
)

// basis is the record's nutrient basis and how it was read. A serving in g or ml names it. Some
// records give their serving in MG, IU or MC, but the number is the grams or millilitres the
// nutrients are per 100 of: the label's calories per serving equal energy per 100 × serving / 100.
// Where they agree (within 10% or 5 kcal), the serving is read as millilitres when the household
// serving or package names fluid ounces, millilitres or litres and the package names no weight,
// else as grams. Where they don't, the basis is unknown and the record is dropped.
func basis(b brandedFood, n map[string]float64) (bs, how string) {
	if bs = unitBasis(b.ServingSizeUnit); bs != "" {
		return bs, "unit"
	}
	kcal, ok := n["energy_kcal"]
	label := b.LabelNutrients["calories"].Value
	if !ok || label == nil || b.ServingSize == nil || *b.ServingSize <= 0 {
		return "", ""
	}
	perServing := kcal * *b.ServingSize / 100
	if d := math.Abs(perServing - *label); d > 0.10**label && d > 5 {
		return "", ""
	}
	words := b.HouseholdServingFullText + " " + b.PackageWeight
	if liquidWords.MatchString(words) && !solidWords.MatchString(b.PackageWeight) {
		return "100ml", "label calories"
	}
	return "100g", "label calories"
}

// Plausibility reasons, in the order they're checked; a record counts under the first it fails.
const (
	whyNegative      = "a negative value, or a serving of 0"
	whyNoEnergy      = "no energy in kcal"
	whyNoBasis       = "serving unit not g or ml, and the label's calories don't confirm the serving as either"
	whyOverLimit     = "protein + fat + carbohydrate over 100 g per 100"
	whyOverEnergy    = "energy over 900 kcal per 100"
	whyEnergyAtwater = "energy more than 20% and 40 kcal from 4·protein + 4·carbohydrate + 9·fat (+ 7·alcohol)"
)

var plausibilityOrder = []string{whyNegative, whyNoEnergy, whyNoBasis, whyOverLimit, whyOverEnergy, whyEnergyAtwater}

// atwater is 4·protein + 4·carbohydrate + 9·fat + 7·alcohol, and whether the three macros are given.
func atwater(n map[string]float64) (float64, bool) {
	p, okP := n["protein_g"]
	c, okC := n["carbohydrate_g"]
	f, okF := n["fat_g"]
	if !okP || !okC || !okF {
		return 0, false
	}
	return 4*p + 4*c + 9*f + 7*n["alcohol_g"], true
}

// implausible returns the first plausibility rule a record breaks, or "".
func implausible(b brandedFood, n map[string]float64) string {
	if b.ServingSize != nil && *b.ServingSize <= 0 {
		return whyNegative
	}
	for _, fn := range b.FoodNutrients {
		if fn.Amount != nil && *fn.Amount < 0 {
			return whyNegative
		}
	}
	kcal, ok := n["energy_kcal"]
	if !ok {
		return whyNoEnergy
	}
	if bs, _ := basis(b, n); bs == "" {
		return whyNoBasis
	}
	if n["protein_g"]+n["fat_g"]+n["carbohydrate_g"] > 100 {
		return whyOverLimit
	}
	if kcal > 900 {
		return whyOverEnergy
	}
	if want, ok := atwater(n); ok && energyOff(kcal, want, 0.20, 40) {
		return whyEnergyAtwater
	}
	return ""
}

// energyOff says whether stated energy is more than pct of itself and more than kcal away from
// the Atwater estimate.
func energyOff(stated, want, pct, kcal float64) bool {
	d := math.Abs(stated - want)
	return d > pct*stated && d > kcal
}

// sentenceCase lowers a description written all in capitals, keeping its first letter.
func sentenceCase(s string) string {
	for _, r := range s {
		if unicode.IsLower(r) {
			return s
		}
	}
	rs := []rune(strings.ToLower(s))
	for i, r := range rs {
		if unicode.IsLetter(r) {
			rs[i] = unicode.ToUpper(r)
			break
		}
	}
	return string(rs)
}

// draftRecord is close to what phase A will write for a branded food, encoded only to measure size.
func draftRecord(b brandedFood, gtin string, n map[string]float64, withIngredients bool) map[string]any {
	brand := strings.TrimSpace(b.BrandName)
	if brand == "" {
		brand = strings.TrimSpace(b.BrandOwner)
	}
	rec := map[string]any{
		"$schema":     "https://openfoodlog.org/schema/v0/food.json",
		"id":          "gtin:" + gtin,
		"name":        sentenceCase(strings.TrimSpace(b.Description)),
		"kind":        "branded",
		"brand":       brand,
		"gtin":        gtin,
		"identifiers": map[string]int{"fdc_id": b.FdcID},
		"provenance": map[string]any{
			"sources": []map[string]string{
				{"name": "USDA FoodData Central, Global Branded Food Products Database (2026-10-29)", "id": strconv.Itoa(b.FdcID), "url": fmt.Sprintf("https://fdc.nal.usda.gov/food-details/%d/nutrients", b.FdcID), "license": "CC0-1.0"},
				{"name": strings.TrimSpace(b.BrandOwner) + " (data provider)"},
			},
			"license": "CC0-1.0",
			"review":  "unreviewed",
		},
	}
	bs, _ := basis(b, n)
	if bs == "100ml" {
		rec["nutrients_basis"] = "100ml"
		rec["nutrients_per_100ml"] = n
	} else {
		rec["nutrients_per_100g"] = n
	}
	if b.ServingSize != nil {
		serving := map[string]any{"label": strings.TrimSpace(b.HouseholdServingFullText)}
		if bs == "100ml" {
			serving["ml"] = *b.ServingSize
		} else {
			serving["grams"] = *b.ServingSize
		}
		label := map[string]any{"serving": serving, "region": "us"}
		perServing := map[string]float64{}
		for k, v := range b.LabelNutrients {
			if v.Value != nil {
				perServing[k] = *v.Value
			}
		}
		if len(perServing) > 0 {
			label["nutrients_per_serving"] = perServing
		}
		rec["label"] = label
	}
	if c := strings.TrimSpace(b.BrandedFoodCategory); c != "" {
		rec["category"] = map[string]string{"name": c, "scheme": "fdc", "basis": "inferred"}
	}
	if withIngredients && strings.TrimSpace(b.Ingredients) != "" {
		rec["ingredients_text"] = strings.TrimSpace(b.Ingredients)
	}
	return rec
}

// eachBranded streams the download's BrandedFoods array, handing over each record's raw JSON.
func eachBranded(path string, fn func(json.RawMessage) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(bufio.NewReaderSize(f, 1<<20))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return fmt.Errorf("%s: want a JSON object", path)
	}
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return err
		}
		if key != "BrandedFoods" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return err
			}
			continue
		}
		if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
			return fmt.Errorf("%s: BrandedFoods isn't an array", path)
		}
		for dec.More() {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return err
			}
			if err := fn(raw); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("%s: no BrandedFoods", path)
}

// countingWriter counts what passes through it.
type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

// sizer measures a JSONL file and its gzip without keeping either.
type sizer struct {
	raw, zipped countingWriter
	gz          *gzip.Writer
}

func newSizer() *sizer {
	s := &sizer{}
	s.gz = gzip.NewWriter(&s.zipped)
	return s
}

func (s *sizer) add(rec any) error {
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	s.raw.n += int64(len(line))
	_, err = s.gz.Write(line)
	return err
}

type tally map[string]int

func (t tally) sorted() []string {
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if t[keys[i]] != t[keys[j]] {
			return t[keys[i]] > t[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

var releaseInName = regexp.MustCompile(`(\d{4}-\d{2}-\d{2})\.json$`)

// brandedReport reads the Branded Foods JSON at path and writes catalogue/BRANDED-REPORT.md under root.
func brandedReport(path, root string) error {
	m := releaseInName.FindStringSubmatch(filepath.Base(path))
	if m == nil {
		return errors.New("want the unzipped download, named FoodData_Central_branded_food_json_YYYY-MM-DD.json")
	}
	release, _ := time.Parse("2006-01-02", m[1])
	vocab, err := readVocabulary(root)
	if err != nil {
		return err
	}

	// Pass 1: markets, GTINs, the version of each product to keep, and field coverage.
	var read int
	fields, markets, sourcesSeen, gtinDrops, units := tally{}, tally{}, tally{}, tally{}, tally{}
	labelKeys, gtinLengths, padded := tally{}, tally{}, tally{}
	best := map[string]brandedFood{}
	copies := map[string]int{}
	err = eachBranded(path, func(raw json.RawMessage) error {
		read++
		var top map[string]json.RawMessage
		if err := json.Unmarshal(raw, &top); err != nil {
			return err
		}
		for k, v := range top {
			switch strings.TrimSpace(string(v)) {
			case `""`, "null", "[]", "{}":
			default:
				fields[k]++
			}
		}
		var b brandedFood
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		markets[strings.TrimSpace(b.MarketCountry)]++
		if !isUS(b.MarketCountry) {
			return nil
		}
		sourcesSeen[b.DataSource]++
		units[strings.TrimSpace(b.ServingSizeUnit)]++
		for k := range b.LabelNutrients {
			labelKeys[k]++
		}
		gtinLengths[strconv.Itoa(len(strings.TrimSpace(b.GtinUpc)))]++
		gtin, why := normalizeGTIN(b.GtinUpc)
		if why != "" {
			gtinDrops[why]++
			return nil
		}
		if isPadded(b.GtinUpc) {
			padded[strconv.Itoa(len(strings.TrimSpace(b.GtinUpc)))]++
		}
		copies[gtin]++
		if cur, ok := best[gtin]; !ok || newer(b, cur) {
			best[gtin] = b
		}
		return nil
	})
	if err != nil {
		return err
	}
	keep := make(map[int]string, len(best))
	for gtin, b := range best {
		keep[b.FdcID] = gtin
	}
	var dupGTINs, dupRecords, maxCopies int
	for _, c := range copies {
		if c > 1 {
			dupGTINs++
			dupRecords += c - 1
		}
		maxCopies = max(maxCopies, c)
	}
	best = nil

	// Pass 2: the kept version of each product, through the remaining rules.
	var discontinued, kept int
	dropped := tally{}
	years := tally{}
	var overFive, noDate int
	var withIngr, ingrBytes, maxIngr int
	ingrLens := []int{}
	coverage := tally{}
	bases, keptSources, fromLabel := tally{}, tally{}, tally{}
	var notCheckable, onlyKJ int
	tolerance := map[string]int{}
	tolerances := []struct {
		name      string
		pct, kcal float64
	}{{"10% and 20 kcal", 0.10, 20}, {"20% and 40 kcal (the rule)", 0.20, 40}, {"30% and 50 kcal", 0.30, 50}, {"50% and 80 kcal", 0.50, 80}}
	withIngrSize, withoutIngrSize := newSizer(), newSizer()
	fiveYears := release.AddDate(-5, 0, 0)
	err = eachBranded(path, func(raw json.RawMessage) error {
		var b brandedFood
		if err := json.Unmarshal(raw, &b); err != nil {
			return err
		}
		gtin, ok := keep[b.FdcID]
		if !ok {
			return nil
		}
		if strings.TrimSpace(b.DiscontinuedDate) != "" {
			discontinued++
			return nil
		}
		n := mapNutrients(b.asFDC(), vocab)
		if _, ok := n["energy_kcal"]; !ok {
			for _, fn := range b.FoodNutrients {
				if fn.Nutrient.Number == "268" && fn.Amount != nil {
					onlyKJ++
					break
				}
			}
		}
		if kcal, ok := n["energy_kcal"]; ok {
			if want, ok := atwater(n); ok {
				for _, t := range tolerances {
					if energyOff(kcal, want, t.pct, t.kcal) {
						tolerance[t.name]++
					}
				}
			} else {
				notCheckable++
			}
		}
		if why := implausible(b, n); why != "" {
			dropped[why]++
			return nil
		}
		kept++
		bs, how := basis(b, n)
		bases[bs]++
		if how == "label calories" {
			fromLabel[strings.ToUpper(strings.TrimSpace(b.ServingSizeUnit))+" as "+bs]++
		}
		keptSources[b.DataSource]++
		deriveNutrients(n)
		for k := range n {
			coverage[k]++
		}
		date, ok := fdcDate(b.AvailableDate)
		if !ok {
			date, ok = fdcDate(b.ModifiedDate)
		}
		if !ok {
			date, ok = fdcDate(b.PublicationDate)
		}
		if ok {
			years[strconv.Itoa(date.Year())]++
			if date.Before(fiveYears) {
				overFive++
			}
		} else {
			noDate++
		}
		if ingr := strings.TrimSpace(b.Ingredients); ingr != "" {
			withIngr++
			ingrBytes += len(ingr)
			maxIngr = max(maxIngr, len(ingr))
			ingrLens = append(ingrLens, len(ingr))
		}
		if err := withIngrSize.add(draftRecord(b, gtin, n, true)); err != nil {
			return err
		}
		return withoutIngrSize.add(draftRecord(b, gtin, n, false))
	})
	if err != nil {
		return err
	}
	for _, s := range []*sizer{withIngrSize, withoutIngrSize} {
		if err := s.gz.Close(); err != nil {
			return err
		}
	}
	sort.Ints(ingrLens)

	us := markets["United States"] + markets["US"]
	validGTIN := us
	for _, c := range gtinDrops {
		validGTIN -= c
	}
	unique := validGTIN - dupRecords
	plausible := unique - discontinued
	for _, c := range dropped {
		plausible -= c
	}
	if plausible != kept {
		return fmt.Errorf("counts disagree: %d by subtraction, %d kept", plausible, kept)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# Branded foods report\n\n")
	fmt.Fprintf(&b, "What `ofl catalogue branded-report` found in USDA FoodData Central's Global Branded Food Products Database, the JSON download of %s (`%s`), measured against the filtering rules planned for a branded catalogue. It writes no records: this is phase 0 of the plan, and nothing here is in `foods.jsonl`.\n\n", release.Format("2006-01-02"), filepath.Base(path))
	b.WriteString("## Each rule, in order\n\n| Step | Records | Dropped |\n| --- | --- | --- |\n")
	fmt.Fprintf(&b, "| Read | %d | |\n", read)
	fmt.Fprintf(&b, "| 1. US market only (`United States` or `US`, kept as `US`) | %d | %d |\n", us, read-us)
	fmt.Fprintf(&b, "| 2. A valid GTIN (up to 14 digits, zero-padded, check digit) | %d | %d |\n", validGTIN, us-validGTIN)
	fmt.Fprintf(&b, "| 3. One per GTIN, newest kept | %d | %d |\n", unique, dupRecords)
	fmt.Fprintf(&b, "| 4. Not discontinued | %d | %d |\n", unique-discontinued, discontinued)
	fmt.Fprintf(&b, "| 5. Plausible numbers | %d | %d |\n", kept, unique-discontinued-kept)
	fmt.Fprintf(&b, "| **Kept** | **%d** | |\n", kept)

	b.WriteString("\n### 1. Markets\n\n| Market | Records |\n| --- | --- |\n")
	for _, k := range markets.sorted() {
		name := k
		if name == "" {
			name = "(none)"
		}
		fmt.Fprintf(&b, "| %s | %d |\n", name, markets[k])
	}
	b.WriteString("\nThe download names the market both ways; a branded record keeps it as `US`, the shorter form (in OFL, `label.region` is `us`).\n")
	b.WriteString("\nUS records by `dataSource` (GDSN is GS1's network; LI is Label Insight):\n\n| Source | Records |\n| --- | --- |\n")
	for _, k := range sourcesSeen.sorted() {
		fmt.Fprintf(&b, "| %s | %d |\n", orNone(k), sourcesSeen[k])
	}

	b.WriteString("\n### 2. GTINs\n\nWhy a US record's `gtinUpc` isn't a GTIN:\n\n| Reason | Records |\n| --- | --- |\n")
	for _, k := range gtinDrops.sorted() {
		fmt.Fprintf(&b, "| %s | %d |\n", k, gtinDrops[k])
	}
	var paddedAll int
	for _, c := range padded {
		paddedAll += c
	}
	fmt.Fprintf(&b, "\nA code shorter than 14 digits is zero-padded on the left before the check digit is checked, as a scanner's 8, 12 and 13 are. %d US records with a valid GTIN gave it in a length a scanner doesn't (by length:", paddedAll)
	for _, k := range padded.sorted() {
		fmt.Fprintf(&b, " %s digits %d;", k, padded[k])
	}
	b.WriteString(" an 11-digit code is a UPC-A that lost its leading zero), and padding gives them the 14 digits a scan of the package does.\n")
	b.WriteString("\nLength of `gtinUpc` as given, US records:\n\n| Digits | Records |\n| --- | --- |\n")
	for _, k := range gtinLengths.sorted() {
		fmt.Fprintf(&b, "| %s | %d |\n", k, gtinLengths[k])
	}

	fmt.Fprintf(&b, "\n### 3. Repeated GTINs\n\nGTINs (as 14 digits) that more than one `fdcId` carries: %d, with %d records beyond the first of each and at most %d of one GTIN. Earlier versions of a product sit in each record's `foodUpdateLog`, not as records of their own. Where a GTIN repeats, the newest is kept, by `availableDate`, then `modifiedDate`, then the highest `fdcId`.\n", dupGTINs, dupRecords, maxCopies)

	fmt.Fprintf(&b, "\n### 4. Discontinued\n\n%d of the records left after step 3 have a `discontinuedDate`.\n", discontinued)

	b.WriteString("\n### 5. Plausibility\n\nA record counts under the first rule it breaks.\n\n| Rule | Records |\n| --- | --- |\n")
	for _, k := range plausibilityOrder {
		fmt.Fprintf(&b, "| %s | %d |\n", k, dropped[k])
	}
	fmt.Fprintf(&b, "\n%d records with no energy in kcal give kilojoules (268) instead. %d records with energy lack protein, fat or carbohydrate, so the Atwater check can't be made on them; they're kept.\n", onlyKJ, notCheckable)
	b.WriteString("\nThe Atwater check at other tolerances, over every remaining record with energy and the three macros, before the other plausibility rules (a record is off when it's past both bounds):\n\n| Tolerance | Off |\n| --- | --- |\n")
	for _, t := range tolerances {
		fmt.Fprintf(&b, "| %s | %d |\n", t.name, tolerance[t.name])
	}

	b.WriteString("\n## What's kept\n\n### Age\n\nYear of each kept record's `availableDate` (else `modifiedDate`, else `publicationDate`):\n\n| Year | Records |\n| --- | --- |\n")
	yearKeys := make([]string, 0, len(years))
	for k := range years {
		yearKeys = append(yearKeys, k)
	}
	sort.Strings(yearKeys)
	for _, k := range yearKeys {
		fmt.Fprintf(&b, "| %s | %d |\n", k, years[k])
	}
	fmt.Fprintf(&b, "\n%d kept records (%.1f%%) are over 5 years old (dated before %s). %d have no readable date.\n", overFive, pct(overFive, kept), fiveYears.Format("2006-01-02"), noDate)

	b.WriteString("\nKept records by `dataSource`:\n\n| Source | Records |\n| --- | --- |\n")
	for _, k := range keptSources.sorted() {
		fmt.Fprintf(&b, "| %s | %d |\n", orNone(k), keptSources[k])
	}
	b.WriteString("\n### Basis\n\nA serving in g, GRM or GM means nutrients per 100 g; ml or MLT per 100 ml. Some records give their serving in MG, IU or MC, but the number is the grams or millilitres the nutrients are per 100 of: the label's calories per serving equal energy per 100 × serving / 100 (a chocolate at 605 kcal per 100 with a serving of 38 \"MG\" labels 230 kcal). Where the two agree within 10% or 5 kcal, the serving is read as millilitres when the household serving or package names fluid ounces, millilitres or litres and the package names no weight, else as grams. Where they don't, the record is dropped (step 5).\n\n| Nutrients per | Records |\n| --- | --- |\n")
	for _, k := range bases.sorted() {
		fmt.Fprintf(&b, "| %s | %d |\n", orNone(k), bases[k])
	}
	b.WriteString("\nRead from the label's calories:\n\n| Unit given, read as | Records |\n| --- | --- |\n")
	for _, k := range fromLabel.sorted() {
		fmt.Fprintf(&b, "| %s | %d |\n", k, fromLabel[k])
	}
	b.WriteString("\nServing units as given, US records:\n\n| `servingSizeUnit` | Records |\n| --- | --- |\n")
	for _, k := range units.sorted() {
		fmt.Fprintf(&b, "| %s | %d |\n", orNone(k), units[k])
	}

	b.WriteString("\n### Ingredients\n\n")
	if withIngr > 0 {
		fmt.Fprintf(&b, "%d kept records (%.1f%%) have `ingredients`: %.1f MB of text in all, %d bytes on average, a median of %d and a longest of %d.\n", withIngr, pct(withIngr, kept), float64(ingrBytes)/1e6, ingrBytes/withIngr, ingrLens[len(ingrLens)/2], maxIngr)
	}

	b.WriteString("\n### Size\n\nEach kept record encoded as a draft OFL food (`kind: branded`, `gtin:` ID, sentence-cased name, brand, nutrients per 100 g or 100 ml with the worked-out keys, the label's serving and per-serving values, category, `fdc_id`, provenance) and gzipped at the default level. Phase A's records will differ a little.\n\n| | JSONL | Gzipped |\n| --- | --- | --- |\n")
	fmt.Fprintf(&b, "| With ingredients | %s | %s |\n", mb(withIngrSize.raw.n), mb(withIngrSize.zipped.n))
	fmt.Fprintf(&b, "| Without ingredients | %s | %s |\n", mb(withoutIngrSize.raw.n), mb(withoutIngrSize.zipped.n))

	b.WriteString("\n## Field coverage\n\nTop-level fields of every record read, and how many give a non-empty value:\n\n| Field | Records | Share |\n| --- | --- | --- |\n")
	for _, k := range fields.sorted() {
		fmt.Fprintf(&b, "| `%s` | %d | %.1f%% |\n", k, fields[k], pct(fields[k], read))
	}
	b.WriteString("\n`labelNutrients` keys, US records:\n\n| Key | Records |\n| --- | --- |\n")
	for _, k := range labelKeys.sorted() {
		fmt.Fprintf(&b, "| `%s` | %d |\n", k, labelKeys[k])
	}
	b.WriteString("\nVocabulary keys of the kept records, read by `nutrients.json`'s FoodData Central numbers as generic foods are (with `energy_kj`, `salt_g` and `carbohydrate_available_g` worked out by section 4 of the spec):\n\n| Key | Records | Share |\n| --- | --- | --- |\n")
	for _, v := range vocab {
		if coverage[v.Key] > 0 {
			fmt.Fprintf(&b, "| `%s` | %d | %.1f%% |\n", v.Key, coverage[v.Key], pct(coverage[v.Key], kept))
		}
	}
	var none []string
	for _, v := range vocab {
		if coverage[v.Key] == 0 {
			none = append(none, "`"+v.Key+"`")
		}
	}
	if len(none) > 0 {
		b.WriteString("\nNo kept record gives " + strings.Join(none, ", ") + ".\n")
	}

	if err := os.WriteFile(filepath.Join(root, "catalogue", "BRANDED-REPORT.md"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("branded report: %d read, %d kept\n", read, kept)
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

func pct(n, of int) float64 {
	if of == 0 {
		return 0
	}
	return 100 * float64(n) / float64(of)
}

func mb(n int64) string { return fmt.Sprintf("%.0f MB", float64(n)/1e6) }
