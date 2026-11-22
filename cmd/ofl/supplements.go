package main

import (
	"math"
	"regexp"
	"strings"
)

// The supplements measure reads NIH's Dietary Supplement Label Database (DSLD) bulk download and
// counts what a supplements catalogue could be built from (Lifterclub's docs/supplements.md,
// phase 0). This file holds the pieces that read one label: its barcode, each ingredient row's
// nutrient key, its unit, and the check of an amount against its percent daily value.
// supplements_report.go reads the download and writes catalogue/SUPPLEMENTS-REPORT.md. Neither
// writes a catalogue record.

// dsldLabel is one label of the download, a JSON file of its own: the fields the measure reads.
type dsldLabel struct {
	ID                   int             `json:"id"`
	FullName             string          `json:"fullName"`
	BrandName            string          `json:"brandName"`
	UpcSku               string          `json:"upcSku"`
	ServingsPerContainer *string         `json:"servingsPerContainer"`
	OffMarket            int             `json:"offMarket"`
	EntryDate            string          `json:"entryDate"`
	ProductType          dsldCode        `json:"productType"`
	PhysicalState        dsldCode        `json:"physicalState"`
	ServingSizes         []dsldServing   `json:"servingSizes"`
	IngredientRows       []dsldRow       `json:"ingredientRows"`
	OtherIngredients     dsldOther       `json:"otheringredients"`
	LabelRelationships   []dsldRelation  `json:"labelRelationships"`
	Contacts             []dsldContact   `json:"contacts"`
	UserGroups           []dsldUserGroup `json:"userGroups"`
}

// dsldCode is a LanguaL code and its description.
type dsldCode struct {
	Code        string `json:"langualCode"`
	Description string `json:"langualCodeDescription"`
}

func (c dsldCode) String() string {
	if c.Code == "" && c.Description == "" {
		return ""
	}
	return strings.TrimSpace(c.Code + " " + strings.TrimSpace(c.Description))
}

type dsldServing struct {
	MinQuantity *float64 `json:"minQuantity"`
	MaxQuantity *float64 `json:"maxQuantity"`
	Unit        string   `json:"unit"`
}

type dsldOther struct {
	Ingredients []struct {
		Name string `json:"name"`
	} `json:"ingredients"`
}

type dsldRelation struct {
	Type string `json:"type"`
}

type dsldContact struct {
	Details struct {
		Country string `json:"country"`
	} `json:"contactDetails"`
}

type dsldUserGroup struct {
	Name string `json:"dailyValueTargetGroupName"`
}

// dsldRow is one row of the Supplement Facts panel. Rows nest: the parts of a blend, the folic
// acid inside a folate row, added sugars under total sugars.
type dsldRow struct {
	Name            string         `json:"name"`
	Category        string         `json:"category"`
	IngredientGroup string         `json:"ingredientGroup"`
	Notes           string         `json:"notes"`
	Forms           []dsldForm     `json:"forms"`
	Quantity        []dsldQuantity `json:"quantity"`
	NestedRows      []dsldRow      `json:"nestedRows"`
}

type dsldForm struct {
	Name string `json:"name"`
}

// dsldQuantity is a row's amount for one of the label's serving sizes, with the percent daily
// value the label prints for each group of people it names.
type dsldQuantity struct {
	Operator    string        `json:"operator"`
	Quantity    *float64      `json:"quantity"`
	Unit        string        `json:"unit"`
	DailyValues []dsldPercent `json:"dailyValueTargetGroup"`
}

type dsldPercent struct {
	Name     string   `json:"name"`
	Operator string   `json:"operator"`
	Percent  *float64 `json:"percent"`
}

// adultsGroup is the daily value group the %DV check reads.
const adultsGroup = "Adults and children 4 or more years of age"

var (
	skuSeparators = strings.NewReplacer(" ", "", "-", "", " ", "", "\t", "")
	skuDigit      = regexp.MustCompile(`[0-9]`)
	skuLetter     = regexp.MustCompile(`\pL`)
)

// supplementGTIN reads a label's `upcSku`, the barcode as printed ("3 0005-4451-76 7"), as 14
// digits. Spaces and dashes go; what's left must be 8, 12, 13 or 14 digits (EAN-8, UPC-A, EAN-13,
// GTIN-14) with a valid check digit, checked by normalizeGTIN as branded foods' are. Unlike
// branded foods, a shorter run of digits isn't zero-padded: the field also holds product codes,
// and one in ten of those would pass a check digit by chance. The reason names why it isn't one.
func supplementGTIN(sku string) (gtin, reason string) {
	code := skuSeparators.Replace(strings.TrimSpace(sku))
	switch {
	case code == "":
		return "", "none"
	case strings.Trim(code, "0123456789") != "":
		return "", "not digits"
	}
	switch len(code) {
	case 8, 12, 13, 14:
		return normalizeGTIN(code)
	}
	return "", "not 8, 12, 13 or 14 digits"
}

// skuShape is a barcode field's shape: every digit as 9 and every letter as a, so codes written
// the same way count together.
func skuShape(sku string) string {
	return skuLetter.ReplaceAllString(skuDigit.ReplaceAllString(strings.TrimSpace(sku), "9"), "a")
}

// supplementRule maps DSLD ingredient groups onto one vocabulary key. A group is compared by its
// base, the lowercased text before any parenthesis ("Vitamin E (mixed tocopherols)" is "vitamin
// e"). Only, when set, must match the row's lowercased name; not must match neither the name nor
// the group's parenthesis.
type supplementRule struct {
	key    string
	groups []string
	only   *regexp.Regexp
	not    *regexp.Regexp
}

func mineralRule(key, element string) supplementRule {
	return supplementRule{key: key, groups: []string{element}, only: regexp.MustCompile(`^(elemental |total )?` + element + `$`)}
}

// supplementRules is the group-to-key table, in the order tried: the first rule whose groups hold
// the row's group and whose name rules pass decides it. Where keys share a group (added and total
// sugars, which DSLD also files under Carbohydrate; folic acid and folate), the specific one comes
// first.
//
// A name rule keeps a compound out of its element's key: DSLD puts "Sodium Alginate" in the group
// Sodium and "Choline Bitartrate" in Choline, and their amounts are the compound's weight.
// Vitamin groups take any name (a label writes vitamin A as "Vitamin A Palmitate"), less the
// forms the key doesn't mean: vitamin E's key is alpha-tocopherol and vitamin K's phylloquinone.
var supplementRules = []supplementRule{
	{key: "energy_kcal", groups: []string{"calories", "header"}, only: regexp.MustCompile(`^(total )?(calories|energy)$`)},
	{key: "protein_g", groups: []string{"protein"}, only: regexp.MustCompile(`^(total |complete )?proteins?$`)},
	{key: "fat_g", groups: []string{"fat"}, only: regexp.MustCompile(`^(total )?fats?( total)?$`)},
	{key: "saturated_fat_g", groups: []string{"saturated fat"}},
	{key: "monounsaturated_fat_g", groups: []string{"monounsaturated fat"}},
	{key: "polyunsaturated_fat_g", groups: []string{"polyunsaturated fat"}},
	{key: "trans_fat_g", groups: []string{"trans fat"}},
	{key: "ala_g", groups: []string{"alpha-linolenic acid"}},
	{key: "epa_g", groups: []string{"epa"}},
	{key: "dha_g", groups: []string{"dha"}},
	{key: "cholesterol_mg", groups: []string{"cholesterol"}, only: regexp.MustCompile(`^cholesterol$`)},
	{key: "added_sugars_g", groups: []string{"sugar", "carbohydrate"}, only: regexp.MustCompile(`^added sugars?$`)},
	{key: "sugars_g", groups: []string{"sugar", "carbohydrate"}, only: regexp.MustCompile(`^(total )?sugars?$`)},
	{key: "carbohydrate_g", groups: []string{"carbohydrate"}, only: regexp.MustCompile(`^(total )?carbohydrates?$`)},
	{key: "fiber_g", groups: []string{"fiber"}, only: regexp.MustCompile(`^(total )?(dietary )?fib(er|re)s?$`)},
	{key: "starch_g", groups: []string{"starch"}, only: regexp.MustCompile(`^starch$`)},
	{key: "polyols_g", groups: []string{"sugar alcohol"}},
	{key: "sodium_mg", groups: []string{"sodium"}, only: regexp.MustCompile(`^sodium$`)},
	mineralRule("potassium_mg", "potassium"),
	mineralRule("calcium_mg", "calcium"),
	mineralRule("iron_mg", "iron"),
	mineralRule("magnesium_mg", "magnesium"),
	{key: "phosphorus_mg", groups: []string{"phosphorus"}, only: regexp.MustCompile(`^(elemental |total )?phosphoro?us$`)},
	mineralRule("zinc_mg", "zinc"),
	mineralRule("copper_mg", "copper"),
	mineralRule("manganese_mg", "manganese"),
	mineralRule("selenium_ug", "selenium"),
	mineralRule("iodine_ug", "iodine"),
	mineralRule("chromium_ug", "chromium"),
	mineralRule("molybdenum_ug", "molybdenum"),
	mineralRule("chloride_mg", "chloride"),
	{key: "vitamin_a_rae_ug", groups: []string{"vitamin a"}},
	{key: "vitamin_c_mg", groups: []string{"vitamin c"}},
	{key: "vitamin_d_ug", groups: []string{"vitamin d", "vitamin d3"}},
	{key: "vitamin_e_mg", groups: []string{"vitamin e"}, not: regexp.MustCompile(`gamma|delta|beta|tocotrienol|mixed|tocopherols|toco(ph|f)ersolan`)},
	{key: "vitamin_k_ug", groups: []string{"vitamin k"}, not: regexp.MustCompile(`k-?2|mena|mk-?[0-9]|mixed`)},
	{key: "thiamin_mg", groups: []string{"thiamin", "vitamin b1"}},
	{key: "riboflavin_mg", groups: []string{"riboflavin", "vitamin b2"}},
	{key: "niacin_mg", groups: []string{"niacin", "vitamin b3"}, not: regexp.MustCompile(`riboside`)},
	{key: "pantothenic_acid_mg", groups: []string{"pantothenic acid", "vitamin b5"}, not: regexp.MustCompile(`pantethine`)},
	{key: "vitamin_b6_mg", groups: []string{"vitamin b6", "pyridoxine"}},
	{key: "biotin_ug", groups: []string{"biotin", "vitamin b7"}},
	{key: "folic_acid_ug", groups: []string{"folate", "vitamin b9", "folic acid"}, only: regexp.MustCompile(`^(coenzyme )?folic acid`)},
	{key: "folate_dfe_ug", groups: []string{"folate", "vitamin b9", "folic acid"}},
	{key: "vitamin_b12_ug", groups: []string{"vitamin b12"}},
	{key: "choline_mg", groups: []string{"choline"}, only: regexp.MustCompile(`^(elemental |total )?choline$`)},
	{key: "caffeine_mg", groups: []string{"caffeine"}, only: regexp.MustCompile(`caffeine|trimethylxanthine`), not: regexp.MustCompile(`malate|citrate|pterostilbene|extract`)},
	{key: "alcohol_g", groups: []string{"alcohol"}, only: regexp.MustCompile(`^(alcohol|ethanol)$`)},
}

// Why a row has no key.
const (
	whyNoKey         = "no key"          // the vocabulary has nothing for the row's group
	whyNamedOther    = "named otherwise" // the group has a key, but the row names a compound or a form the key doesn't mean
	whyPartOfMapped  = "part of a mapped row"
	statusMapped     = "mapped"
	groupNotRecorded = "(none)"
)

// groupBase splits an ingredient group into its lowercased base and the text in its parenthesis.
func groupBase(group string) (base, qualifier string) {
	g := strings.ToLower(strings.TrimSpace(group))
	if i := strings.Index(g, " ("); i >= 0 {
		return strings.TrimSpace(g[:i]), strings.Trim(g[i+2:], " )")
	}
	return g, ""
}

// supplementKey is the vocabulary key an ingredient row's group and name mean, or why there is
// none (whyNoKey or whyNamedOther).
func supplementKey(group, name string) (key, why string) {
	base, qualifier := groupBase(group)
	n := strings.ToLower(strings.TrimSpace(name))
	known := false
	for _, r := range supplementRules {
		if !contains(r.groups, base) {
			continue
		}
		known = true
		if r.only != nil && !r.only.MatchString(n) {
			continue
		}
		if r.not != nil && (r.not.MatchString(n) || r.not.MatchString(qualifier)) {
			continue
		}
		return r.key, ""
	}
	if known {
		return "", whyNamedOther
	}
	return "", whyNoKey
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

var unitBraces = strings.NewReplacer("{", "", "}", "")

// supplementUnit reads a unit as DSLD writes it ("Gram(s)", "{Calories}", "mg {alpha}-TE") into
// one spelling. No unit ("NP", "Not Present", empty) is ""; a unit it doesn't know comes back as
// written.
func supplementUnit(unit string) string {
	switch unitBraces.Replace(strings.ToLower(strings.TrimSpace(unit))) {
	case "", "np", "not present", "none":
		return ""
	case "g", "gram(s)", "grams", "gram", "gm":
		return "g"
	case "mg", "milligram(s)", "milligrams":
		return "mg"
	case "mcg", "ug", "µg", "μg", "microgram(s)":
		return "mcg"
	case "iu", "ui", "i.u.":
		return "IU"
	case "calorie(s)", "calories", "kcal", "kcal(s)":
		return "kcal"
	case "kj":
		return "kJ"
	case "mcg rae":
		return "mcg RAE"
	case "mcg re":
		return "mcg RE"
	case "mcg dfe":
		return "mcg DFE"
	case "mg dfe":
		return "mg DFE"
	case "mg ne":
		return "mg NE"
	case "mg alpha-te", "mg a-te", "mg at", "mg a-tocopherol":
		return "mg alpha-TE"
	}
	return strings.TrimSpace(unit)
}

// How a mapped row's amount reaches its key's unit.
const (
	howDirect   = "converts directly"
	howIU       = "needs an IU conversion"
	howNotDFE   = "folate in mcg or mg, not stated as DFE"
	howNoneUnit = "can't be converted"
)

// micrograms is how many micrograms a mass unit holds.
var micrograms = map[string]float64{"g": 1e6, "mg": 1e3, "mcg": 1, "µg": 1}

// equivalentUnits are the units that name an equivalent, the mass unit each is counted in, and the
// one key each means.
var equivalentUnits = map[string]struct{ mass, key string }{
	"mcg RAE":     {"mcg", "vitamin_a_rae_ug"},
	"mcg DFE":     {"mcg", "folate_dfe_ug"},
	"mg DFE":      {"mg", "folate_dfe_ug"},
	"mg NE":       {"mg", "niacin_mg"},
	"mg alpha-TE": {"mg", "vitamin_e_mg"},
}

// supplementAmount gives an amount in its key's unit and says how it got there. Only a direct
// conversion returns a value: kcal and kJ for energy, grams, milligrams and micrograms for a
// mass, and an equivalent unit for the key it belongs to. Folate in plain micrograms is its own
// case: a label from before 2020 meant micrograms of folate or folic acid, a later one means DFE
// and DSLD may have dropped the letters. IU converts only for vitamins A, D and E.
func supplementAmount(key, keyUnit string, amount float64, unit string) (value float64, how string) {
	if keyUnit == "kcal" || keyUnit == "kJ" {
		switch {
		case unit == keyUnit:
			return amount, howDirect
		case unit == "kJ":
			return amount / 4.184, howDirect
		case unit == "kcal":
			return amount * 4.184, howDirect
		}
		return 0, howNoneUnit
	}
	if eq, ok := equivalentUnits[unit]; ok {
		if eq.key != key {
			return 0, howNoneUnit
		}
		return amount * micrograms[eq.mass] / micrograms[keyUnit], howDirect
	}
	if per, ok := micrograms[unit]; ok {
		if key == "folate_dfe_ug" {
			return 0, howNotDFE
		}
		return amount * per / micrograms[keyUnit], howDirect
	}
	if unit == "IU" && len(iuFactors[key]) > 0 {
		return 0, howIU
	}
	return 0, howNoneUnit
}

// iuFactors is what one IU weighs in the key's unit, per form where the form changes it. From the
// NIH Office of Dietary Supplements' unit conversions for the 2016 labelling rule: vitamin D 1 IU
// = 0.025 mcg whatever the form; vitamin A 1 IU = 0.3 mcg RAE as retinol or as beta-carotene in a
// supplement (0.05 as beta-carotene in food); vitamin E 1 IU = 0.67 mg as natural d-alpha-
// tocopherol and 0.45 mg as synthetic dl-alpha-tocopherol.
var iuFactors = map[string][]float64{
	"vitamin_d_ug":     {0.025},
	"vitamin_a_rae_ug": {0.3},
	"vitamin_e_mg":     {0.67, 0.45},
}

var (
	retinolWords   = regexp.MustCompile(`retin|palmitate|acetate`)
	caroteneWords  = regexp.MustCompile(`caroten|pro-? ?vitamin a`)
	naturalEWords  = regexp.MustCompile(`\bd-alpha|\brrr|natural`)
	syntheticWords = regexp.MustCompile(`\bdl-alpha|all-rac|synthetic`)
)

// iuForm is the form a row in IU states, read from its name, its forms and its notes: what an IU
// conversion needs for vitamin E (natural or synthetic) and, for a food source, vitamin A.
func iuForm(key string, row dsldRow) string {
	words := []string{row.Name, row.Notes}
	for _, f := range row.Forms {
		words = append(words, f.Name)
	}
	text := strings.ToLower(strings.Join(words, " | "))
	switch key {
	case "vitamin_d_ug":
		return "not needed (one factor for D2 and D3)"
	case "vitamin_a_rae_ug":
		retinol, carotene := retinolWords.MatchString(text), caroteneWords.MatchString(text)
		switch {
		case retinol && carotene:
			return "retinol and beta-carotene"
		case retinol:
			return "retinol"
		case carotene:
			return "beta-carotene"
		}
	case "vitamin_e_mg":
		natural, synthetic := naturalEWords.MatchString(text), syntheticWords.MatchString(text)
		switch {
		case natural && synthetic:
			return "natural and synthetic"
		case natural:
			return "natural (d-alpha)"
		case synthetic:
			return "synthetic (dl-alpha)"
		}
	}
	return "not stated"
}

// dailyValue is FDA's daily value for adults and children 4 or more years of age, in the key's
// unit: the current one, and the ones before the 2016 rule where they differ or a label in IU has
// to be read.
type dailyValue struct {
	current float64
	before  []float64
	// beforeIU is the old daily value as it was stated, in IU.
	beforeIU float64
}

// dailyValues are the Reference Daily Intakes and Daily Reference Values of 21 CFR 101.9(c)(8)(iv)
// and (c)(9) for adults and children 4 or more years of age. Current: as amended by "Food
// Labeling: Revision of the Nutrition and Supplement Facts Labels", 81 FR 33742 (27 May 2016),
// required on labels from 1 January 2020 (2021 for smaller manufacturers). Before: the same
// paragraphs as they stood before that rule (FDA's "Daily Value on the Nutrition and Supplement
// Facts Labels" lists both side by side). Old values stated in IU are also given in the key's
// unit: vitamin A 5,000 IU as 1,500 mcg, vitamin D 400 IU as 10 mcg, vitamin E 30 IU as 20.1 mg
// natural or 13.5 mg synthetic. Folic acid has no daily value of its own: its row is read against
// folate's, 400 mcg before and 400 mcg DFE now (235 mcg of folic acid, at 1.7 DFE each).
// Choline and added sugars had no daily value before the rule.
var dailyValues = map[string]dailyValue{
	"fat_g":               {current: 78, before: []float64{65}},
	"saturated_fat_g":     {current: 20, before: []float64{20}},
	"cholesterol_mg":      {current: 300, before: []float64{300}},
	"carbohydrate_g":      {current: 275, before: []float64{300}},
	"fiber_g":             {current: 28, before: []float64{25}},
	"added_sugars_g":      {current: 50},
	"protein_g":           {current: 50, before: []float64{50}},
	"sodium_mg":           {current: 2300, before: []float64{2400}},
	"potassium_mg":        {current: 4700, before: []float64{3500}},
	"calcium_mg":          {current: 1300, before: []float64{1000}},
	"iron_mg":             {current: 18, before: []float64{18}},
	"magnesium_mg":        {current: 420, before: []float64{400}},
	"phosphorus_mg":       {current: 1250, before: []float64{1000}},
	"zinc_mg":             {current: 11, before: []float64{15}},
	"copper_mg":           {current: 0.9, before: []float64{2}},
	"manganese_mg":        {current: 2.3, before: []float64{2}},
	"selenium_ug":         {current: 55, before: []float64{70}},
	"iodine_ug":           {current: 150, before: []float64{150}},
	"chromium_ug":         {current: 35, before: []float64{120}},
	"molybdenum_ug":       {current: 45, before: []float64{75}},
	"chloride_mg":         {current: 2300, before: []float64{3400}},
	"vitamin_a_rae_ug":    {current: 900, before: []float64{1500}, beforeIU: 5000},
	"vitamin_c_mg":        {current: 90, before: []float64{60}},
	"vitamin_d_ug":        {current: 20, before: []float64{10}, beforeIU: 400},
	"vitamin_e_mg":        {current: 15, before: []float64{20.1, 13.5}, beforeIU: 30},
	"vitamin_k_ug":        {current: 120, before: []float64{80}},
	"thiamin_mg":          {current: 1.2, before: []float64{1.5}},
	"riboflavin_mg":       {current: 1.3, before: []float64{1.7}},
	"niacin_mg":           {current: 16, before: []float64{20}},
	"pantothenic_acid_mg": {current: 5, before: []float64{10}},
	"vitamin_b6_mg":       {current: 1.7, before: []float64{2}},
	"biotin_ug":           {current: 30, before: []float64{300}},
	"folate_dfe_ug":       {current: 400, before: []float64{400}},
	"folic_acid_ug":       {current: 400 / 1.7, before: []float64{400}},
	"vitamin_b12_ug":      {current: 2.4, before: []float64{6}},
	"choline_mg":          {current: 550},
}

// What the %DV check finds.
const (
	dvAgrees     = "agree"
	dvThousand   = "off by a factor of 1000"
	dvDisagrees  = "disagree"
	dvNotChecked = ""
	eraCurrent   = "current"
	eraBefore    = "before 2016"
	eraEither    = "either"
	// dvNarrowly marks a disagreement within 25% or three percentage points: a label rounds its
	// amounts too (fat to the half gram, sodium to 5 or 10 mg), so the percent of a small amount
	// can sit a point or two from the one worked out here.
	dvNarrowly = "narrowly"
)

// near says whether an amount is the stated percent of a daily value: within the share of it
// given, or within that many percentage points. The check's own bounds are 10% and one point,
// since a label rounds its percent (to a whole number now; before the rule to the nearest 2 up
// to 10% and the nearest 5 up to 50%).
func near(amount, percent, dv, share, points float64) bool {
	want := percent / 100 * dv
	return math.Abs(amount-want) <= share*want || math.Abs(amount/dv*100-percent) <= points
}

// checkDailyValue compares a mapped row's amount with its percent daily value for adults. The
// amount is in a unit supplementUnit gives. It agrees when the amount is that percent of the
// current daily value or of the one before 2016 (era says which, or either); failing that, it's
// off by a factor of 1000 when a thousandth or a thousand times the amount would agree (mcg
// written for mg, or the reverse). A unit with no conversion, or a key with no daily value, isn't
// checked. A disagreement's era is dvNarrowly when it is a near miss.
func checkDailyValue(key, keyUnit string, amount float64, unit string, percent float64) (verdict, era string) {
	dv, ok := dailyValues[key]
	if !ok || amount <= 0 || percent <= 0 {
		return dvNotChecked, ""
	}
	// Each pair is the amount and a daily value in the same unit.
	type pair struct {
		amount, dv float64
		era        string
	}
	var pairs []pair
	value, how := supplementAmount(key, keyUnit, amount, unit)
	switch how {
	case howDirect:
		pairs = append(pairs, pair{value, dv.current, eraCurrent})
		for _, b := range dv.before {
			pairs = append(pairs, pair{value, b, eraBefore})
		}
	case howNotDFE:
		value = amount * micrograms[unit] / micrograms[keyUnit]
		pairs = append(pairs, pair{value, dv.current, eraCurrent}, pair{value, dv.before[0], eraBefore})
	case howIU:
		for _, f := range iuFactors[key] {
			pairs = append(pairs, pair{amount * f, dv.current, eraCurrent})
		}
		pairs = append(pairs, pair{amount, dv.beforeIU, eraBefore})
	default:
		return dvNotChecked, ""
	}
	agrees := map[string]bool{}
	thousand, narrowly := false, false
	for _, p := range pairs {
		if near(p.amount, percent, p.dv, 0.10, 1) {
			agrees[p.era] = true
		}
		if near(p.amount*1000, percent, p.dv, 0.10, 1) || near(p.amount/1000, percent, p.dv, 0.10, 1) {
			thousand = true
		}
		if near(p.amount, percent, p.dv, 0.25, 3) {
			narrowly = true
		}
	}
	switch {
	case agrees[eraCurrent] && agrees[eraBefore]:
		return dvAgrees, eraEither
	case agrees[eraCurrent]:
		return dvAgrees, eraCurrent
	case agrees[eraBefore]:
		return dvAgrees, eraBefore
	case thousand:
		return dvThousand, ""
	case narrowly:
		return dvDisagrees, dvNarrowly
	}
	return dvDisagrees, ""
}

// Amount states of a row, read from its first quantity (the label's first serving size).
const (
	amountNone    = "no quantity at all"
	amountNP      = "not present on the label (unit NP)"
	amountZero    = "a stated zero"
	amountPresent = "an amount"
)

func amountState(q []dsldQuantity) string {
	switch {
	case len(q) == 0:
		return amountNone
	case supplementUnit(q[0].Unit) == "":
		return amountNP
	case q[0].Quantity == nil:
		return amountNone
	case *q[0].Quantity == 0:
		return amountZero
	}
	return amountPresent
}

// rowRead is what the measure makes of one ingredient row.
type rowRead struct {
	row    dsldRow
	nested bool
	state  string // amountNone … amountPresent
	unit   string // supplementUnit of the first quantity
	key    string // the vocabulary key, when status is statusMapped or whyPartOfMapped
	status string // statusMapped, whyPartOfMapped, whyNamedOther or whyNoKey
	how    string // howDirect …, for a mapped row with an amount
	value  float64
}

// hasAmount says whether the row states an amount, a zero included.
func (r rowRead) hasAmount() bool { return r.state == amountPresent || r.state == amountZero }

// readRows flattens a label's rows, nested ones after their parent, and reads each. A nested row
// whose key an ancestor already carries is a part of that row (the beta-carotene inside vitamin
// A): it would count the amount twice, so it's marked whyPartOfMapped, not mapped.
func readRows(rows []dsldRow, keyUnits map[string]string) []rowRead {
	var out []rowRead
	var walk func(rows []dsldRow, nested bool, above []string)
	walk = func(rows []dsldRow, nested bool, above []string) {
		for _, row := range rows {
			r := rowRead{row: row, nested: nested, state: amountState(row.Quantity)}
			if len(row.Quantity) > 0 {
				r.unit = supplementUnit(row.Quantity[0].Unit)
			}
			key, why := supplementKey(row.IngredientGroup, row.Name)
			switch {
			case why != "":
				r.status = why
			case contains(above, key):
				r.key, r.status = key, whyPartOfMapped
			default:
				r.key, r.status = key, statusMapped
			}
			if r.status == statusMapped && r.hasAmount() {
				r.value, r.how = supplementAmount(key, keyUnits[key], *row.Quantity[0].Quantity, r.unit)
			}
			out = append(out, r)
			next := above
			if key != "" {
				next = append(append([]string{}, above...), key)
			}
			walk(row.NestedRows, true, next)
		}
	}
	walk(rows, false, nil)
	return out
}

// dvRead is one %DV check: a mapped row's amount for one serving size against the percent the
// label prints for adults.
type dvRead struct {
	key, name, unit string
	amount, percent float64
	verdict, era    string
}

// readDailyValues checks every amount of the mapped rows that carries a percent daily value for
// adults, both stated exactly ("=", not "<").
func readDailyValues(rows []rowRead, keyUnits map[string]string) []dvRead {
	var out []dvRead
	for _, r := range rows {
		if r.status != statusMapped {
			continue
		}
		for _, q := range r.row.Quantity {
			if q.Quantity == nil || q.Operator != "=" {
				continue
			}
			for _, p := range q.DailyValues {
				if p.Name != adultsGroup || p.Percent == nil || p.Operator != "=" {
					continue
				}
				unit := supplementUnit(q.Unit)
				verdict, era := checkDailyValue(r.key, keyUnits[r.key], *q.Quantity, unit, *p.Percent)
				if verdict == dvNotChecked {
					continue
				}
				out = append(out, dvRead{r.key, r.row.Name, unit, *q.Quantity, *p.Percent, verdict, era})
			}
		}
	}
	return out
}
