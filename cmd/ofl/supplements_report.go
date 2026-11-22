package main

import (
	"archive/zip"
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// `ofl catalogue supplements-measure` reads the DSLD bulk download, a zip with one JSON file per
// label, straight from the zip and a few hundred labels at a time, and writes
// catalogue/SUPPLEMENTS-REPORT.md. It writes no records: the sizes it reports come from encoding
// a draft record for each barcode and throwing the bytes away.

// dsldSource is where the bulk download comes from, and how NIH asks for it to be named.
const (
	dsldSource   = "https://api.ods.od.nih.gov/dsld/s3/data/DSLD-full-database-JSON.zip"
	dsldCitation = "US Department of Health and Human Services, National Institutes of Health, Office of Dietary Supplements. Dietary Supplement Label Database (DSLD). [Internet]. Available from: https://dsld.od.nih.gov/dsld/."
	dsldLicence  = "DSLD data are in the public domain and they are not copyrighted. They are published under CC0 1.0 Universal (CC0 1.0)."
)

// supplementsOptions are what the measure can't read from the download itself.
type supplementsOptions struct {
	// Retrieved is the day the download was fetched, YYYY-MM-DD.
	Retrieved string
	// BrandedGTINs is a file of the branded catalogue's GTINs, one 14-digit GTIN a line, or "".
	BrandedGTINs string
}

func runSupplementsMeasure(args []string) error {
	fs := flag.NewFlagSet("supplements-measure", flag.ContinueOnError)
	var opts supplementsOptions
	fs.StringVar(&opts.Retrieved, "retrieved", "", "the day the download was fetched, YYYY-MM-DD")
	fs.StringVar(&opts.BrandedGTINs, "branded-gtins", "", "a file of the branded catalogue's GTINs, one a line, to count the overlap")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: ofl catalogue supplements-measure --retrieved YYYY-MM-DD [--branded-gtins <file>] <DSLD-full-database-JSON.zip>")
	}
	start := time.Now()
	res, err := supplementsMeasure(fs.Arg(0), ".", opts)
	if err != nil {
		return err
	}
	fmt.Printf("supplements report: %d labels read, %d that didn't parse, %d on market with a valid GTIN (%d GTINs), in %s\n",
		res.Read, res.Failed, res.Candidates, res.CandidateGTINs, time.Since(start).Round(time.Second))
	return nil
}

type supplementsResult struct {
	Read, Failed, Candidates, CandidateGTINs int
}

// levelCounts counts ingredient rows at one depth: the panel's own rows, or the rows nested under
// them.
type levelCounts struct {
	rows                                        int
	categories, groups, units, states, iuGroups tally
}

func newLevelCounts() levelCounts {
	return levelCounts{categories: tally{}, groups: tally{}, units: tally{}, states: tally{}, iuGroups: tally{}}
}

// supplementCounts is everything counted over one set of labels: all of them, or the candidates.
type supplementCounts struct {
	labels                                              int
	productTypes, physicalStates, servingUnits          tally
	multiServing, rangeServing, noServing, perContainer int
	noBrand, noName, noAdults, withNP, withBlend        int
	eras                                                tally // labels by the daily values their percents agree with
	relationships                                       tally
	levels                                              [2]levelCounts // 0 top-level, 1 nested

	status, statusAmount                            tally // rows by mapping status, all and with an amount
	typeRows, typeMapped, typeAmount, typeAmountMap tally // by product type; parts of a mapped row left out
	labelFit                                        tally
	unmappedLabels, unmappedRows, unmappedAmounts   tally // by ingredient group
	namedOther                                      tally // "Group: name" of rows a name rule kept out
	keyLabels, keyRows                              tally
	hows                                            tally
	howByKey, iuForms, dv                           map[string]tally
}

func newSupplementCounts() *supplementCounts {
	return &supplementCounts{
		productTypes: tally{}, physicalStates: tally{}, servingUnits: tally{}, relationships: tally{}, eras: tally{},
		levels: [2]levelCounts{newLevelCounts(), newLevelCounts()},
		status: tally{}, statusAmount: tally{},
		typeRows: tally{}, typeMapped: tally{}, typeAmount: tally{}, typeAmountMap: tally{},
		labelFit: tally{}, unmappedLabels: tally{}, unmappedRows: tally{}, unmappedAmounts: tally{},
		namedOther: tally{}, keyLabels: tally{}, keyRows: tally{}, hows: tally{},
		howByKey: map[string]tally{}, iuForms: map[string]tally{}, dv: map[string]tally{},
	}
}

func bump(m map[string]tally, key, what string) {
	if m[key] == nil {
		m[key] = tally{}
	}
	m[key][what]++
}

// Label fits: how much of a label's stated amounts the vocabulary holds.
const (
	fitAll      = "every row with an amount maps"
	fitSome     = "some rows with an amount map"
	fitNone     = "no row with an amount maps"
	fitNoAmount = "no row states an amount"
)

func (c *supplementCounts) add(l *dsldLabel, rows []rowRead, checks []dvRead) {
	c.labels++
	productType := orNone(l.ProductType.String())
	c.productTypes[productType]++
	c.physicalStates[orNone(l.PhysicalState.String())]++
	if strings.TrimSpace(l.BrandName) == "" {
		c.noBrand++
	}
	if strings.TrimSpace(l.FullName) == "" {
		c.noName++
	}
	c.addServings(l)
	seen := map[string]bool{}
	for _, r := range l.LabelRelationships {
		if !seen[r.Type] {
			seen[r.Type] = true
			c.relationships[r.Type]++
		}
	}

	adults := false
	for _, g := range l.UserGroups {
		adults = adults || g.Name == adultsGroup
	}
	if !adults {
		c.noAdults++
	}

	var amounts, mapped int
	var np, blend bool
	groupsSeen, keysSeen := map[string]bool{}, map[string]bool{}
	for _, r := range rows {
		c.addRow(r, productType)
		group := orNone(strings.TrimSpace(r.row.IngredientGroup))
		np = np || r.state == amountNP
		blend = blend || isBlend(group)
		if r.status == whyNoKey && !groupsSeen[group] {
			groupsSeen[group] = true
			c.unmappedLabels[group]++
		}
		if !r.hasAmount() || r.status == whyPartOfMapped {
			continue
		}
		amounts++
		if r.status == statusMapped {
			mapped++
			keysSeen[r.key] = true
		}
	}
	for k := range keysSeen {
		c.keyLabels[k]++
	}
	switch {
	case amounts == 0:
		c.labelFit[fitNoAmount]++
	case mapped == amounts:
		c.labelFit[fitAll]++
	case mapped > 0:
		c.labelFit[fitSome]++
	default:
		c.labelFit[fitNone]++
	}
	if np {
		c.withNP++
	}
	if blend {
		c.withBlend++
	}
	var current, before, checked bool
	for _, d := range checks {
		checked = true
		bump(c.dv, d.key, d.verdict)
		if d.verdict == dvAgrees || d.era == dvNarrowly {
			bump(c.dv, d.key, d.era)
		}
		current = current || (d.verdict == dvAgrees && d.era == eraCurrent)
		before = before || (d.verdict == dvAgrees && d.era == eraBefore)
	}
	switch {
	case !checked:
		c.eras[eraNoPercent]++
	case current && before:
		c.eras[eraMixed]++
	case current:
		c.eras[eraCurrent]++
	case before:
		c.eras[eraBefore]++
	default:
		c.eras[eraUnclear]++
	}
}

// Which daily values a label's percents agree with, beside eraCurrent and eraBefore.
const (
	eraNoPercent = "no percent to check"
	eraMixed     = "some rows with each"
	eraUnclear   = "can't tell (every percent agrees with either, or none agrees)"
)

// isBlend says whether an ingredient group is one of DSLD's blends ("Proprietary Blend
// (Herb/Botanical)", "Blend (Combination)").
func isBlend(group string) bool {
	base, _ := groupBase(group)
	return base == "blend" || base == "proprietary blend"
}

func (c *supplementCounts) addServings(l *dsldLabel) {
	if len(l.ServingSizes) == 0 {
		c.noServing++
	}
	if len(l.ServingSizes) > 1 {
		c.multiServing++
	}
	ranged := false
	for _, s := range l.ServingSizes {
		c.servingUnits[orNone(strings.TrimSpace(s.Unit))]++
		if s.MinQuantity != nil && s.MaxQuantity != nil && *s.MinQuantity != *s.MaxQuantity {
			ranged = true
		}
	}
	if ranged {
		c.rangeServing++
	}
	if l.ServingsPerContainer != nil && strings.TrimSpace(*l.ServingsPerContainer) != "" {
		c.perContainer++
	}
}

func (c *supplementCounts) addRow(r rowRead, productType string) {
	level := &c.levels[0]
	if r.nested {
		level = &c.levels[1]
	}
	group := orNone(strings.TrimSpace(r.row.IngredientGroup))
	level.rows++
	level.categories[orNone(strings.TrimSpace(r.row.Category))]++
	level.groups[group]++
	level.states[r.state]++
	if r.hasAmount() {
		level.units[orNone(r.unit)]++
	}
	if r.unit == "IU" {
		level.iuGroups[group]++
	}

	c.status[r.status]++
	if r.hasAmount() {
		c.statusAmount[r.status]++
	}
	switch r.status {
	case whyNoKey:
		c.unmappedRows[group]++
		if r.hasAmount() {
			c.unmappedAmounts[group]++
		}
	case whyNamedOther:
		c.namedOther[group+": "+strings.TrimSpace(r.row.Name)]++
	case whyPartOfMapped:
		return
	}
	c.typeRows[productType]++
	if r.hasAmount() {
		c.typeAmount[productType]++
	}
	if r.status != statusMapped {
		return
	}
	c.typeMapped[productType]++
	if !r.hasAmount() {
		return
	}
	c.typeAmountMap[productType]++
	c.keyRows[r.key]++
	c.hows[r.how]++
	bump(c.howByKey, r.key, r.how)
	if r.how == howIU {
		bump(c.iuForms, r.key, iuForm(r.key, r.row))
	}
}

// gtinSeen is what the measure keeps for each barcode.
type gtinSeen struct {
	labels, onMarket          int
	name, brand               string // of the first label read, lowercased
	onName                    string // of the first label on the market
	otherOnName               bool   // labels on the market differ in name
	first, second             string // the first label's name and brand as written, and the first that differs
	otherName, otherBrand     bool
	otherType                 bool
	productType               string
	flavours, sizes, pictures bool
}

// barcodeCounts counts the labels' `upcSku`.
type barcodeCounts struct {
	withSKU, fnsku  int
	reasons         tally
	shapes          tally
	examples        map[string][]string
	lengths         tally // digits left after spaces and dashes go, where only digits are left
	elevenPadded    int   // 11-digit codes that are a valid UPC-A once a leading zero is put back
	gtins           map[string]*gtinSeen
	candidateLabels int
}

func (b *barcodeCounts) add(l *dsldLabel) (gtin string) {
	sku := strings.TrimSpace(l.UpcSku)
	if sku == "" {
		return ""
	}
	b.withSKU++
	gtin, why := supplementGTIN(sku)
	code := skuSeparators.Replace(sku)
	if strings.Trim(code, "0123456789") == "" {
		b.lengths[strconv.Itoa(len(code))]++
		if g, _ := normalizeGTIN(code); len(code) == 11 && g != "" {
			b.elevenPadded++
		}
	}
	if why != "" {
		b.reasons[why]++
		shape := why + "|" + skuShape(sku)
		b.shapes[shape]++
		if len(b.examples[shape]) < 3 {
			b.examples[shape] = append(b.examples[shape], sku)
		}
		if fnsku.MatchString(sku) {
			b.fnsku++
		}
		return ""
	}
	g := b.gtins[gtin]
	name, brand := strings.ToLower(strings.TrimSpace(l.FullName)), strings.ToLower(strings.TrimSpace(l.BrandName))
	written := strings.TrimSpace(l.FullName) + " (" + strings.TrimSpace(l.BrandName) + ")"
	if g == nil {
		g = &gtinSeen{name: name, brand: brand, productType: l.ProductType.Code, first: written}
		b.gtins[gtin] = g
	}
	g.labels++
	if name != g.name && g.second == "" {
		g.second = written
	}
	g.otherName = g.otherName || name != g.name
	g.otherBrand = g.otherBrand || brand != g.brand
	g.otherType = g.otherType || l.ProductType.Code != g.productType
	for _, r := range l.LabelRelationships {
		switch r.Type {
		case "Flavor difference":
			g.flavours = true
		case "Package size difference, same product":
			g.sizes = true
		case "Image difference, same product":
			g.pictures = true
		}
	}
	if l.OffMarket == 0 {
		if g.onMarket == 0 {
			g.onName = name
		}
		g.otherOnName = g.otherOnName || name != g.onName
		g.onMarket++
		b.candidateLabels++
	}
	return gtin
}

// fnsku is the shape of the codes Amazon prints on stock it holds for a seller (X00 and seven
// more letters or digits), which a label shows in place of a barcode.
var fnsku = regexp.MustCompile(`^X00[0-9A-Z]{7}$`)

// gs1Range names the part of the GS1 prefix table a GTIN's first digits fall in.
func gs1Range(gtin string) string {
	if strings.HasPrefix(gtin, "000000") {
		return "an EAN-8"
	}
	p, _ := strconv.Atoi(gtin[1:4])
	switch {
	case p <= 19, p >= 30 && p <= 39, p >= 60 && p <= 139:
		return "GS1 US (000–019, 030–039, 060–139)"
	case p >= 20 && p <= 29, p >= 40 && p <= 49, p >= 200 && p <= 299:
		return "restricted, for use inside a company or shop (020–029, 040–049, 200–299)"
	case p >= 50 && p <= 59:
		return "coupons (050–059)"
	}
	return "another GS1 organisation"
}

var postcode = regexp.MustCompile(`^[0-9 -]+$`)

// contactCountry reads a contact's country, written many ways, into "United States", "" or the
// text itself.
func contactCountry(s string) string {
	s = strings.TrimSpace(s)
	switch strings.ToUpper(strings.NewReplacer(".", "", " ", "").Replace(s)) {
	case "":
		return ""
	case "USA", "US", "UNITEDSTATES", "UNITEDSTATESOFAMERICA":
		return "United States"
	}
	if postcode.MatchString(s) {
		return ""
	}
	return s
}

// supplementDraft is the record the size estimate encodes: an OFL branded food with a label and
// no per-100 basis, the unmapped rows kept as written. It is a guess at a shape, for sizing only.
type supplementDraft struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Kind            string            `json:"kind"`
	Brand           string            `json:"brand,omitempty"`
	GTIN            []string          `json:"gtin"`
	Label           labelDoc          `json:"label"`
	Ingredients     []draftIngredient `json:"ingredients,omitempty"`
	IngredientsText *ingredientsDoc   `json:"ingredients_text,omitempty"`
	Identifiers     map[string]int    `json:"identifiers"`
	Provenance      provenanceDoc     `json:"provenance"`
}

type draftIngredient struct {
	Name   string   `json:"name"`
	Amount *float64 `json:"amount,omitempty"`
	Unit   string   `json:"unit,omitempty"`
}

func draftRecord(l *dsldLabel, gtin string, rows []rowRead, retrieved string) supplementDraft {
	rec := supplementDraft{
		ID: "gtin:" + gtin, Name: strings.TrimSpace(l.FullName), Kind: "branded", Brand: strings.TrimSpace(l.BrandName),
		GTIN: []string{gtin}, Label: labelDoc{Region: "us", NutrientsPerServing: map[string]float64{}},
		Identifiers: map[string]int{"dsld_id": l.ID},
		Provenance: provenanceDoc{
			Sources: []sourceDoc{{
				Name: "NIH Office of Dietary Supplements, Dietary Supplement Label Database (retrieved " + retrieved + ")",
				ID:   strconv.Itoa(l.ID), License: "CC0-1.0", URL: "https://dsld.od.nih.gov/label/" + strconv.Itoa(l.ID),
			}},
			License: "CC0-1.0", Review: "unreviewed",
		},
	}
	if len(l.ServingSizes) > 0 && l.ServingSizes[0].MinQuantity != nil {
		s := l.ServingSizes[0]
		rec.Label.Serving = &servingDoc{Text: strconv.FormatFloat(*s.MinQuantity, 'f', -1, 64) + " " + strings.TrimSpace(s.Unit)}
	}
	for _, r := range rows {
		switch {
		case r.status == statusMapped && r.how == howDirect:
			rec.Label.NutrientsPerServing[r.key] = r.value
		case r.hasAmount():
			rec.Ingredients = append(rec.Ingredients, draftIngredient{strings.TrimSpace(r.row.Name), r.row.Quantity[0].Quantity, r.unit})
		default:
			rec.Ingredients = append(rec.Ingredients, draftIngredient{Name: strings.TrimSpace(r.row.Name)})
		}
	}
	var other []string
	for _, o := range l.OtherIngredients.Ingredients {
		other = append(other, strings.TrimSpace(o.Name))
	}
	if len(other) > 0 {
		rec.IngredientsText = &ingredientsDoc{Text: strings.Join(other, ", "), Locale: "en", Basis: "declared"}
	}
	return rec
}

// dvExample is one disagreement between an amount and its percent daily value, for the report.
type dvExample struct {
	id             int
	product, brand string
	dvRead
}

// fileSHA256 is a file's size and SHA-256.
func fileSHA256(path string) (size int64, sum string, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, "", err
	}
	defer f.Close()
	h := sha256.New()
	size, err = io.Copy(h, f)
	return size, hex.EncodeToString(h.Sum(nil)), err
}

var labelFile = regexp.MustCompile(`(^|/)([0-9]+)\.json$`)

// dsldFiles lists the zip's label files, ordered by DSLD ID, and reads its ReadMe.
func dsldFiles(z *zip.Reader) (files []*zip.File, readme string, err error) {
	ids := map[*zip.File]int{}
	for _, f := range z.File {
		if m := labelFile.FindStringSubmatch(f.Name); m != nil {
			ids[f], _ = strconv.Atoi(m[2])
			files = append(files, f)
			continue
		}
		if strings.HasSuffix(strings.ToLower(f.Name), "readme.csv") {
			raw, err := readZipFile(f)
			if err != nil {
				return nil, "", err
			}
			readme = string(raw)
		}
	}
	sort.Slice(files, func(i, j int) bool { return ids[files[i]] < ids[files[j]] })
	return files, readme, nil
}

func readZipFile(f *zip.File) ([]byte, error) {
	r, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// eachLabel reads the labels in order, parsing a batch at a time on every processor so memory
// holds one batch. fn gets a nil label and the error for a file that doesn't parse.
func eachLabel(files []*zip.File, fn func(name string, l *dsldLabel, err error) error) error {
	const batch = 512
	labels := make([]*dsldLabel, batch)
	errs := make([]error, batch)
	workers := runtime.NumCPU()
	for start := 0; start < len(files); start += batch {
		chunk := files[start:min(start+batch, len(files))]
		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Go(func() {
				for i := w; i < len(chunk); i += workers {
					labels[i], errs[i] = parseLabel(chunk[i])
				}
			})
		}
		wg.Wait()
		for i, f := range chunk {
			if err := fn(f.Name, labels[i], errs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseLabel(f *zip.File) (*dsldLabel, error) {
	raw, err := readZipFile(f)
	if err != nil {
		return nil, err
	}
	var l dsldLabel
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, err
	}
	return &l, nil
}

// readGTINs reads a file of GTINs, one a line.
func readGTINs(path string) (map[string]bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	set := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if g := strings.TrimSpace(sc.Text()); g != "" {
			set[g] = true
		}
	}
	return set, sc.Err()
}

var readmeLine = regexp.MustCompile(`(?m)^Date of download,(.*)$|(?m)^"?(Version [^"\n]*)"?\s*$`)

// supplementsMeasure reads the DSLD download at path and writes catalogue/SUPPLEMENTS-REPORT.md
// under root.
func supplementsMeasure(path, root string, opts supplementsOptions) (supplementsResult, error) {
	var res supplementsResult
	if _, err := time.Parse("2006-01-02", opts.Retrieved); err != nil {
		return res, errors.New("want --retrieved YYYY-MM-DD, the day the download was fetched")
	}
	vocab, err := readVocabulary(root)
	if err != nil {
		return res, err
	}
	keyUnits := map[string]string{}
	for _, v := range vocab {
		keyUnits[v.Key] = v.Unit
	}
	for _, r := range supplementRules {
		if _, ok := keyUnits[r.key]; !ok {
			return res, fmt.Errorf("the mapping names %q, which isn't in the nutrient vocabulary", r.key)
		}
	}
	var branded map[string]bool
	if opts.BrandedGTINs != "" {
		if branded, err = readGTINs(opts.BrandedGTINs); err != nil {
			return res, err
		}
	}
	size, sum, err := fileSHA256(path)
	if err != nil {
		return res, err
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		return res, err
	}
	defer zr.Close()
	files, readme, err := dsldFiles(&zr.Reader)
	if err != nil {
		return res, err
	}
	if len(files) == 0 {
		return res, fmt.Errorf("%s: no label files (<id>.json)", path)
	}

	all, cand := newSupplementCounts(), newSupplementCounts()
	bar := &barcodeCounts{reasons: tally{}, shapes: tally{}, examples: map[string][]string{}, lengths: tally{}, gtins: map[string]*gtinSeen{}}
	years, yearsOff, yearsCand := tally{}, tally{}, tally{}
	var failed []string
	var onMarket int
	var examples []dvExample
	userGroups, countries, brandedTypes := tally{}, tally{}, tally{}
	var countryNone, countryUS, countryOther int
	full, lean := newSizer(), newSizer()
	sized := map[string]bool{}
	err = eachLabel(files, func(name string, l *dsldLabel, perr error) error {
		res.Read++
		if perr != nil {
			res.Failed++
			if len(failed) < 5 {
				failed = append(failed, fmt.Sprintf("`%s`: %v", filepath.Base(name), perr))
			}
			return nil
		}
		year := "(none)"
		if len(l.EntryDate) >= 4 {
			year = l.EntryDate[:4]
		}
		years[year]++
		if l.OffMarket != 0 {
			yearsOff[year]++
		} else {
			onMarket++
		}
		for _, g := range l.UserGroups {
			userGroups[orNone(g.Name)]++
		}
		rows := readRows(l.IngredientRows, keyUnits)
		checks := readDailyValues(rows, keyUnits)
		all.add(l, rows, checks)
		gtin := bar.add(l)
		if gtin == "" || l.OffMarket != 0 {
			return nil
		}
		cand.add(l, rows, checks)
		yearsCand[year]++
		for _, d := range checks {
			if d.verdict == dvThousand || (d.verdict == dvDisagrees && d.era != dvNarrowly) {
				examples = append(examples, dvExample{l.ID, strings.TrimSpace(l.FullName), strings.TrimSpace(l.BrandName), d})
			}
		}
		if branded[gtin] {
			brandedTypes[orNone(l.ProductType.String())]++
		}
		other := tally{}
		us := false
		for _, c := range l.Contacts {
			switch country := contactCountry(c.Details.Country); country {
			case "":
			case "United States":
				us = true
			default:
				other[country]++
			}
		}
		switch {
		case len(other) > 0:
			countryOther++
			for k := range other {
				countries[k]++
			}
		case us:
			countryUS++
		default:
			countryNone++
		}
		if sized[gtin] {
			return nil
		}
		sized[gtin] = true
		rec := draftRecord(l, gtin, rows, opts.Retrieved)
		if err := full.add(rec); err != nil {
			return err
		}
		rec.Ingredients, rec.IngredientsText = nil, nil
		return lean.add(rec)
	})
	if err != nil {
		return res, err
	}
	for _, s := range []*sizer{full, lean} {
		if err := s.gz.Close(); err != nil {
			return res, err
		}
	}
	res.Candidates = cand.labels
	res.CandidateGTINs = len(sized)

	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }
	parsed := res.Read - res.Failed

	w("# Supplements report\n\n")
	w("What `ofl catalogue supplements-measure` found in the NIH Office of Dietary Supplements' Dietary Supplement Label Database (DSLD), the bulk download of every label as JSON. It writes no records: this is phase 0 of a supplements plan, the same measuring USDA's branded foods had first ([BRANDED-REPORT.md](BRANDED-REPORT.md)), and nothing here is in `foods.jsonl` or `branded-foods.jsonl.gz`.\n\n")
	w("| | |\n| --- | --- |\n")
	w("| File | `%s` |\n", filepath.Base(path))
	w("| Size | %d bytes |\n", size)
	w("| SHA-256 | `%s` |\n", sum)
	w("| From | %s |\n", dsldSource)
	w("| Retrieved | %s |\n", opts.Retrieved)
	for _, m := range readmeLine.FindAllStringSubmatch(readme, -1) {
		if m[1] != "" {
			w("| DSLD's own date of download (its `ReadMe.csv`) | %s |\n", strings.TrimSpace(m[1]))
		}
		if m[2] != "" {
			w("| DSLD release (its `ReadMe.csv`) | %s |\n", cell(m[2]))
		}
	}
	w("| Cite as | *%s* |\n", dsldCitation)
	w("| Licence | CC0 1.0. NIH's API guide: \"%s\" |\n", dsldLicence)
	overlap := ""
	if branded != nil {
		overlap = " --branded-gtins <a file of the branded catalogue's GTINs, one a line>"
	}
	w("\nWritten by `ofl catalogue supplements-measure --retrieved %s%s %s`, which gives the same report from the same file.\n", opts.Retrieved, overlap, filepath.Base(path))
	w("\nThe zip holds one JSON file per label, named by its DSLD ID; the command reads them from the zip in ID order and never unpacks it. **The candidate set** below is every label that is on the market (`offMarket` 0) and whose `upcSku` reads as a valid GTIN: what a first build could be made from.\n")

	// 1. Labels.
	w("\n## 1. Labels\n\n| | Labels |\n| --- | --- |\n")
	w("| Label files read | %d |\n", res.Read)
	w("| Didn't parse | %d |\n", res.Failed)
	w("| On the market (`offMarket` 0) | %d |\n", onMarket)
	w("| Off the market (`offMarket` 1) | %d |\n", parsed-onMarket)
	for _, f := range failed {
		w("\nDidn't parse: %s\n", f)
	}
	w("\nBy the year of `entryDate`, when DSLD took the label in (not when the product was sold):\n\n| Year | Labels | On the market | Off the market | Candidate set |\n| --- | --- | --- | --- | --- |\n")
	for _, y := range sortedKeys(years) {
		w("| %s | %d | %d | %d | %d |\n", y, years[y], years[y]-yearsOff[y], yearsOff[y], yearsCand[y])
	}
	w("\n%d labels have no `brandName` and %d no `fullName` (%d and %d in the candidate set).\n", all.noBrand, all.noName, cand.noBrand, cand.noName)

	// 2. Barcodes.
	var valid int
	dist := tally{}
	var several, oneOn, noneOn, manyOn, candGTINs int
	var otherNames, otherBrands, otherTypes, flavourGTINs, sizeGTINs, pictureGTINs, maxLabels, otherOnNames int
	var crowded []string
	ranges := tally{}
	for gtin, g := range bar.gtins {
		valid += g.labels
		maxLabels = max(maxLabels, g.labels)
		switch {
		case g.labels == 1:
			dist["1"]++
		case g.labels == 2:
			dist["2"]++
		case g.labels <= 5:
			dist["3–5"]++
		default:
			dist["6 or more"]++
		}
		if g.onMarket > 0 {
			candGTINs++
			ranges[gs1Range(gtin)]++
		}
		if g.labels == 1 {
			continue
		}
		several++
		if g.otherOnName {
			otherOnNames++
		}
		if g.labels >= 10 {
			crowded = append(crowded, gtin)
		}
		switch g.onMarket {
		case 0:
			noneOn++
		case 1:
			oneOn++
		default:
			manyOn++
		}
		for _, c := range []struct {
			is bool
			n  *int
		}{{g.otherName, &otherNames}, {g.otherBrand, &otherBrands}, {g.otherType, &otherTypes}, {g.flavours, &flavourGTINs}, {g.sizes, &sizeGTINs}, {g.pictures, &pictureGTINs}} {
			if c.is {
				*c.n++
			}
		}
	}
	w("\n## 2. Barcodes\n\n`upcSku` is the barcode as the label prints it, as free text (\"3 0005-4451-76 7\"). Spaces and dashes are dropped; what's left counts as a GTIN when it is 8, 12, 13 or 14 digits (EAN-8, UPC-A, EAN-13, GTIN-14) with a valid check digit, held as 14 digits the way branded foods' are (`normalizeGTIN`). Unlike branded foods, shorter runs of digits aren't zero-padded: the field also holds product codes, and one in ten of those would pass a check digit by chance.\n\n| | Labels | Share of labels |\n| --- | --- | --- |\n")
	w("| A non-empty `upcSku` | %d | %.1f%% |\n", bar.withSKU, pct(bar.withSKU, parsed))
	w("| A valid GTIN | %d | %.1f%% |\n", valid, pct(valid, parsed))
	w("| On the market with a valid GTIN (**the candidate set**) | %d | %.1f%% |\n", bar.candidateLabels, pct(bar.candidateLabels, parsed))
	w("\n| | GTINs |\n| --- | --- |\n| Distinct GTINs | %d |\n| Distinct GTINs with a label on the market (the candidate set's) | %d |\n", len(bar.gtins), candGTINs)
	w("\nWhy a non-empty `upcSku` isn't a GTIN:\n\n| Reason | Labels |\n| --- | --- |\n")
	for _, k := range bar.reasons.sorted() {
		w("| %s | %d |\n", k, bar.reasons[k])
	}
	w("\nThe shapes of those (a digit shown as 9, a letter as a), the 25 most common:\n\n| Reason | Shape | Labels | Examples |\n| --- | --- | --- | --- |\n")
	for i, k := range bar.shapes.sorted() {
		if i == 25 {
			break
		}
		why, shape, _ := strings.Cut(k, "|")
		w("| %s | `%s` | %d | %s |\n", why, shape, bar.shapes[k], "`"+strings.Join(bar.examples[k], "`, `")+"`")
	}
	w("\nDigits left once spaces and dashes go, where only digits are left:\n\n| Digits | Labels |\n| --- | --- |\n")
	for _, k := range bar.lengths.sorted() {
		w("| %s | %d |\n", k, bar.lengths[k])
	}
	w("\n%d of the 11-digit codes are a valid UPC-A once a leading zero is put back (a tenth would be by chance); the measure doesn't count them as GTINs.\n", bar.elevenPadded)
	w("\nLabels per GTIN:\n\n| Labels carrying the GTIN | GTINs |\n| --- | --- |\n")
	for _, k := range []string{"1", "2", "3–5", "6 or more"} {
		w("| %s | %d |\n", k, dist[k])
	}
	w("\nThe most labels on one GTIN is %d. Of the %d GTINs with several labels:\n\n| | GTINs | Share |\n| --- | --- | --- |\n", maxLabels, several)
	w("| Exactly one label on the market | %d | %.1f%% |\n", oneOn, pct(oneOn, several))
	w("| Several on the market | %d | %.1f%% |\n", manyOn, pct(manyOn, several))
	w("| … whose on-market labels have different `fullName`s | %d | %.1f%% |\n", otherOnNames, pct(otherOnNames, several))
	w("| None on the market | %d | %.1f%% |\n", noneOn, pct(noneOn, several))
	w("| Labels with different `fullName`s | %d | %.1f%% |\n", otherNames, pct(otherNames, several))
	w("| Labels with different `brandName`s | %d | %.1f%% |\n", otherBrands, pct(otherBrands, several))
	w("| Labels with different `productType`s | %d | %.1f%% |\n", otherTypes, pct(otherTypes, several))
	w("| A label DSLD relates to another as \"Flavor difference\" | %d | %.1f%% |\n", flavourGTINs, pct(flavourGTINs, several))
	w("| … as \"Package size difference, same product\" | %d | %.1f%% |\n", sizeGTINs, pct(sizeGTINs, several))
	w("| … as \"Image difference, same product\" | %d | %.1f%% |\n", pictureGTINs, pct(pictureGTINs, several))
	w("\nThe relation rows say a label of that GTIN has such a relation to some label, not that the related label shares the GTIN.\n")
	sort.Slice(crowded, func(i, j int) bool {
		a, c := bar.gtins[crowded[i]], bar.gtins[crowded[j]]
		if a.labels != c.labels {
			return a.labels > c.labels
		}
		return crowded[i] < crowded[j]
	})
	w("\n%d GTINs carry 10 labels or more. The eight with the most:\n\n| GTIN | Labels | On the market | The first label | A label named otherwise |\n| --- | --- | --- | --- | --- |\n", len(crowded))
	for i, gtin := range crowded {
		if i == 8 {
			break
		}
		g := bar.gtins[gtin]
		w("| %s | %d | %d | %s | %s |\n", gtin, g.labels, g.onMarket, cell(g.first), cell(g.second))
	}
	w("\n%d labels (%.1f%% of those with an `upcSku`) give a code shaped like the ones Amazon prints on stock it holds for a seller (`X00` and seven letters or digits) where a barcode would be.\n", bar.fnsku, pct(bar.fnsku, bar.withSKU))
	w("\nThe candidate set's GTINs by GS1 prefix (the three digits after a GTIN-14's first, which name the GS1 organisation that issued the company's prefix, not where the product is made or sold):\n\n| Prefix | GTINs |\n| --- | --- |\n")
	for _, k := range ranges.sorted() {
		w("| %s | %d |\n", k, ranges[k])
	}

	// 3. Overlap.
	w("\n## 3. Overlap with branded foods\n\n")
	if branded == nil {
		w("Not counted: the command ran without `--branded-gtins`, the list of GTINs in `branded-foods.jsonl.gz`.\n")
	} else {
		var allIn, candIn int
		for gtin, g := range bar.gtins {
			if !branded[gtin] {
				continue
			}
			allIn++
			if g.onMarket > 0 {
				candIn++
			}
		}
		w("Against the %d GTINs of USDA's branded foods as the catalogue publishes them (`%s`):\n\n| | GTINs | Also a branded food | Share |\n| --- | --- | --- | --- |\n", len(branded), filepath.Base(opts.BrandedGTINs))
		w("| Every valid GTIN | %d | %d | %.1f%% |\n", len(bar.gtins), allIn, pct(allIn, len(bar.gtins)))
		w("| The candidate set's GTINs | %d | %d | %.1f%% |\n", candGTINs, candIn, pct(candIn, candGTINs))
		w("\nCandidate labels whose GTIN is also a branded food, by product type:\n\n| Product type | Labels |\n| --- | --- |\n")
		for _, k := range brandedTypes.sorted() {
			w("| %s | %d |\n", k, brandedTypes[k])
		}
	}

	// 4. Products.
	twoColumns := func(title, column string, a, c tally, limit int) {
		w("\n%s\n\n| %s | All labels | Candidate set |\n| --- | --- | --- |\n", title, column)
		for i, k := range a.sorted() {
			if limit > 0 && i == limit {
				break
			}
			w("| %s | %d | %d |\n", k, a[k], c[k])
		}
	}
	w("\n## 4. Products\n")
	twoColumns("By `productType` (a LanguaL code and its description):", "Product type", all.productTypes, cand.productTypes, 0)
	twoColumns("By `physicalState`:", "Physical state", all.physicalStates, cand.physicalStates, 0)
	twoColumns("Labels DSLD relates to another label (`labelRelationships`), by the kind of relation:", "Relation", all.relationships, cand.relationships, 0)
	w("\nThe groups of people the labels give daily values for (`userGroups`), all labels:\n\n| Group | Labels |\n| --- | --- |\n")
	for _, k := range userGroups.sorted() {
		w("| %s | %d |\n", k, userGroups[k])
	}

	// 5. Servings.
	w("\n## 5. Servings\n\n| | All labels | Candidate set |\n| --- | --- | --- |\n")
	w("| More than one serving size | %d | %d |\n", all.multiServing, cand.multiServing)
	w("| A serving size given as a range (`minQuantity` below `maxQuantity`) | %d | %d |\n", all.rangeServing, cand.rangeServing)
	w("| No serving size | %d | %d |\n", all.noServing, cand.noServing)
	w("| `servingsPerContainer` present | %d | %d |\n", all.perContainer, cand.perContainer)
	twoColumns(fmt.Sprintf("Serving units as written, one count per serving size: %d different spellings in all, the 40 most common here.", len(all.servingUnits)), "`servingSizes[].unit`", all.servingUnits, cand.servingUnits, 40)

	// 6. Ingredient rows.
	w("\n## 6. Ingredient rows\n\nThe rows of the Supplement Facts panel (`ingredientRows`), and the rows nested under them (`nestedRows`, to any depth: the parts of a blend, the folic acid in a folate row, added sugars under total sugars). A row's amount and unit are read from its first `quantity`, the one for the label's first serving size.\n\n| | All labels: top-level | nested | Candidate set: top-level | nested |\n| --- | --- | --- | --- | --- |\n")
	w("| Rows | %d | %d | %d | %d |\n", all.levels[0].rows, all.levels[1].rows, cand.levels[0].rows, cand.levels[1].rows)
	for _, k := range []string{amountPresent, amountZero, amountNP, amountNone} {
		w("| … with %s | %d | %d | %d | %d |\n", k, all.levels[0].states[k], all.levels[1].states[k], cand.levels[0].states[k], cand.levels[1].states[k])
	}
	w("\nDSLD writes a row the label gives no amount for (an ingredient of a proprietary blend) with the unit `NP` and a quantity of 0.\n")
	fourColumns := func(title, column string, pick func(levelCounts) tally, limit int) {
		w("\n%s\n\n| %s | All labels: top-level | nested | Candidate set: top-level | nested |\n| --- | --- | --- | --- | --- |\n", title, column)
		total := tally{}
		for _, c := range []*supplementCounts{all} {
			for _, lv := range c.levels {
				for k, n := range pick(lv) {
					total[k] += n
				}
			}
		}
		for i, k := range total.sorted() {
			if limit > 0 && i == limit {
				break
			}
			w("| %s | %d | %d | %d | %d |\n", k, pick(all.levels[0])[k], pick(all.levels[1])[k], pick(cand.levels[0])[k], pick(cand.levels[1])[k])
		}
	}
	fourColumns("Rows by `category`:", "Category", func(l levelCounts) tally { return l.categories }, 0)
	groupsSeen := map[string]bool{}
	for _, lv := range all.levels {
		for k := range lv.groups {
			groupsSeen[k] = true
		}
	}
	fourColumns(fmt.Sprintf("The 60 most common `ingredientGroup` values, of %d:", len(groupsSeen)), "Ingredient group", func(l levelCounts) tally { return l.groups }, 60)
	unitsSeen := map[string]bool{}
	for _, lv := range all.levels {
		for k := range lv.units {
			unitsSeen[k] = true
		}
	}
	fourColumns(fmt.Sprintf("Units of the rows that state an amount, read into one spelling each (\"Gram(s)\", \"g\" and \"grams\" are g; \"Calorie(s)\" and \"{Calories}\" are kcal; \"UI\" is IU): the 40 most common of %d.", len(unitsSeen)), "Unit", func(l levelCounts) tally { return l.units }, 40)
	fourColumns("Rows in IU, by ingredient group (the 15 most common):", "Ingredient group", func(l levelCounts) tally { return l.iuGroups }, 15)

	// 7. Mapping.
	w("\n## 7. Mapping to Open Food Log nutrient keys\n\nThe table the measure maps by, from DSLD's `ingredientGroup` to a key of `schema/v0/nutrients.json` (%d keys). A group is compared by the text before its parenthesis (\"Vitamin E (mixed tocopherols)\" is \"vitamin e\"). A name rule keeps a compound out of its element's key: DSLD puts \"Sodium Alginate\" in the group Sodium and \"Choline Bitartrate\" in Choline, and the amount on such a row is the compound's weight.\n\n| Key | Unit | DSLD ingredient groups | The row's name must | must not (name or the group's parenthesis) |\n| --- | --- | --- | --- | --- |\n", len(vocab))
	ruled := map[string]bool{}
	for _, r := range supplementRules {
		ruled[r.key] = true
		only, not := "", ""
		if r.only != nil {
			only = "match `" + strings.ReplaceAll(r.only.String(), "|", "\\|") + "`"
		}
		if r.not != nil {
			not = "match `" + strings.ReplaceAll(r.not.String(), "|", "\\|") + "`"
		}
		w("| `%s` | %s | %s | %s | %s |\n", r.key, keyUnits[r.key], strings.Join(r.groups, ", "), only, not)
	}
	var unruled []string
	for _, v := range vocab {
		if !ruled[v.Key] {
			unruled = append(unruled, "`"+v.Key+"`")
		}
	}
	w("\nKeys with no rule: %s. Energy in kJ is read through `energy_kcal`'s rule, by its unit; salt and water are ingredients on these labels, not declared nutrients; \"net carbohydrates\" isn't available carbohydrate as the vocabulary defines it.\n", strings.Join(unruled, ", "))
	w("\nA nested row whose key a row above it already carries is **a part of a mapped row** (the beta-carotene inside vitamin A, the vitamin K1 inside vitamin K): mapping it would count the amount twice, so it is counted apart and left out of the shares.\n")
	statusTable := func(title string, pick func(*supplementCounts) tally) {
		w("\n%s\n\n| | All labels | Share | Candidate set | Share |\n| --- | --- | --- | --- | --- |\n", title)
		a, c := pick(all), pick(cand)
		totalA := a[statusMapped] + a[whyNamedOther] + a[whyNoKey]
		totalC := c[statusMapped] + c[whyNamedOther] + c[whyNoKey]
		for _, row := range []struct{ name, key string }{
			{"Mapped to a key", statusMapped},
			{"In a group with a key, but named otherwise (a compound, or a form the key doesn't mean)", whyNamedOther},
			{"In a group with no key", whyNoKey},
		} {
			w("| %s | %d | %.1f%% | %d | %.1f%% |\n", row.name, a[row.key], pct(a[row.key], totalA), c[row.key], pct(c[row.key], totalC))
		}
		w("| **Rows counted** | **%d** | | **%d** | |\n", totalA, totalC)
		w("| Parts of a mapped row, left out | %d | | %d | |\n", a[whyPartOfMapped], c[whyPartOfMapped])
	}
	statusTable("Every row, top-level and nested:", func(c *supplementCounts) tally { return c.status })
	statusTable("Rows that state an amount (a stated zero included):", func(c *supplementCounts) tally { return c.statusAmount })
	w("\nBy product type, candidate set (parts of a mapped row left out):\n\n| Product type | Rows | Mapped | Share | Rows with an amount | Mapped | Share |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, k := range cand.typeRows.sorted() {
		w("| %s | %d | %d | %.1f%% | %d | %d | %.1f%% |\n", k, cand.typeRows[k], cand.typeMapped[k], pct(cand.typeMapped[k], cand.typeRows[k]), cand.typeAmount[k], cand.typeAmountMap[k], pct(cand.typeAmountMap[k], cand.typeAmount[k]))
	}
	w("\nLabels by how much of what they state the vocabulary holds (rows with an amount, parts of a mapped row left out):\n\n| | All labels | Share | Candidate set | Share |\n| --- | --- | --- | --- | --- |\n")
	for _, k := range []string{fitAll, fitSome, fitNone, fitNoAmount} {
		w("| %s | %d | %.1f%% | %d | %.1f%% |\n", strings.ToUpper(k[:1])+k[1:], all.labelFit[k], pct(all.labelFit[k], all.labels), cand.labelFit[k], pct(cand.labelFit[k], cand.labels))
	}
	w("\nThe 40 ingredient groups with no key that the most candidate labels carry (a label counts once per group; rows count every row, top-level or nested):\n\n| Ingredient group | Candidate labels | Share of them | Rows | Rows with an amount |\n| --- | --- | --- | --- | --- |\n")
	for i, k := range cand.unmappedLabels.sorted() {
		if i == 40 {
			break
		}
		w("| %s | %d | %.1f%% | %d | %d |\n", k, cand.unmappedLabels[k], pct(cand.unmappedLabels[k], cand.labels), cand.unmappedRows[k], cand.unmappedAmounts[k])
	}
	w("\nThe 30 most common rows a name rule kept out of their group's key, candidate set:\n\n| Group: name | Rows |\n| --- | --- |\n")
	for i, k := range cand.namedOther.sorted() {
		if i == 30 {
			break
		}
		w("| %s | %d |\n", k, cand.namedOther[k])
	}
	w("\nMapped rows with an amount, by key, candidate set, and how the amount reaches the key's unit: directly (grams, milligrams and micrograms into each other, kcal and kJ, and a unit naming the key's own equivalent: mcg RAE, mcg DFE, mg NE, mg alpha-TE); through an IU conversion (vitamins A, D and E only); folate in plain mcg or mg, which a label from before the 2016 rule meant as micrograms of folate and a later one means as DFE with the letters dropped; or not at all (IU for another nutrient, mcg RE, a percentage, a unit that isn't a mass).\n\n| Key | Labels | Rows | Directly | IU | Folate, not stated as DFE | Can't be converted |\n| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, v := range vocab {
		if cand.keyRows[v.Key] == 0 {
			continue
		}
		h := cand.howByKey[v.Key]
		w("| `%s` | %d | %d | %d | %d | %d | %d |\n", v.Key, cand.keyLabels[v.Key], cand.keyRows[v.Key], h[howDirect], h[howIU], h[howNotDFE], h[howNoneUnit])
	}
	var mappedAmounts int
	for _, n := range cand.hows {
		mappedAmounts += n
	}
	w("| **All** | | **%d** | **%d** (%.1f%%) | **%d** (%.1f%%) | **%d** (%.1f%%) | **%d** (%.1f%%) |\n", mappedAmounts,
		cand.hows[howDirect], pct(cand.hows[howDirect], mappedAmounts), cand.hows[howIU], pct(cand.hows[howIU], mappedAmounts),
		cand.hows[howNotDFE], pct(cand.hows[howNotDFE], mappedAmounts), cand.hows[howNoneUnit], pct(cand.hows[howNoneUnit], mappedAmounts))
	w("\nThe rows in IU, by the form the row states (its name, its `forms` and its `notes`), candidate set. One IU of vitamin D is 0.025 mcg whatever the form. One IU of vitamin A is 0.3 mcg RAE as retinol and, in a supplement, as beta-carotene (0.05 as the beta-carotene of a food). One IU of vitamin E is 0.67 mg as natural d-alpha-tocopherol and 0.45 mg as synthetic dl-alpha-tocopherol, so a vitamin E row that doesn't say which can't be converted.\n\n| Key | Form stated | Rows |\n| --- | --- | --- |\n")
	for _, v := range vocab {
		for _, k := range cand.iuForms[v.Key].sorted() {
			w("| `%s` | %s | %d |\n", v.Key, k, cand.iuForms[v.Key][k])
		}
	}

	// 8. The %DV check.
	w("\n## 8. The %%DV check\n\nFor every mapped row, each amount that carries a percent daily value for \"%s\", both stated exactly (`=`, not `<`) and above zero, in a unit that converts: the amount against the percent × FDA's daily value (21 CFR 101.9(c)(8)(iv) and (c)(9)). A label from before the 2016 rule states percents of the old daily values, so an amount **agrees** when it is the percent of the current daily value or of the old one, within 10%% or one percentage point (labels round the percent). It is **off by a factor of 1000** when a thousandth or a thousand times the amount would agree (mcg written for mg, or the reverse), and **disagrees** otherwise. An amount in IU is read against the old daily value in IU, and against the current one through the IU factors above (either form's, for vitamin E).\n", adultsGroup)
	dvTable := func(title string, c *supplementCounts) {
		w("\n%s\n\n| Key | Checked | Agree | … only with the current DV | … only with the DV before 2016 | … with either | Off by 1000 | Disagree | … narrowly | Not agreeing | … leaving out the narrow ones |\n| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |\n", title)
		sum := tally{}
		for _, v := range vocab {
			t := c.dv[v.Key]
			n := t[dvAgrees] + t[dvThousand] + t[dvDisagrees]
			if n == 0 {
				continue
			}
			w("| `%s` | %d | %d | %d | %d | %d | %d | %d | %d | %.1f%% | %.1f%% |\n", v.Key, n, t[dvAgrees], t[eraCurrent], t[eraBefore], t[eraEither], t[dvThousand], t[dvDisagrees], t[dvNarrowly], pct(t[dvThousand]+t[dvDisagrees], n), pct(t[dvThousand]+t[dvDisagrees]-t[dvNarrowly], n))
			for k, x := range t {
				sum[k] += x
			}
		}
		n := sum[dvAgrees] + sum[dvThousand] + sum[dvDisagrees]
		w("| **All** | **%d** | **%d** | **%d** | **%d** | **%d** | **%d** | **%d** | **%d** | **%.1f%%** | **%.1f%%** |\n", n, sum[dvAgrees], sum[eraCurrent], sum[eraBefore], sum[eraEither], sum[dvThousand], sum[dvDisagrees], sum[dvNarrowly], pct(sum[dvThousand]+sum[dvDisagrees], n), pct(sum[dvThousand]+sum[dvDisagrees]-sum[dvNarrowly], n))
	}
	dvTable("Candidate set:", cand)
	dvTable("All labels:", all)
	w("\n\"With either\" is where the two daily values are the same or within a tenth of each other (iron, iodine, folate, saturated fat, cholesterol, protein, sodium, magnesium, carbohydrate; vitamin E, whose old 30 IU is 13.5 mg as the synthetic form, within a tenth of today's 15 mg), or where the percent is small enough that both round to it. A disagreement is **narrow** when the amount is within 25%% or three percentage points of the percent: labels round their amounts too (fat to the half gram, sodium to 5 or 10 mg), so a small amount's percent can sit a point or two from the one worked out here. Protein's percent is of protein corrected for its quality (21 CFR 101.9(c)(7)), so it can sit below amount ÷ 50 g without being wrong.\n")
	w("\nLabels by the daily values their percents agree with, which says which rule a label was printed under (a row counts when its percent agrees with one daily value and not the other):\n\n| | All labels | Share | Candidate set | Share |\n| --- | --- | --- | --- | --- |\n")
	for _, row := range []struct{ name, key string }{
		{"Only the current daily values", eraCurrent}, {"Only the daily values before 2016", eraBefore}, {"Some rows with each", eraMixed},
		{"Can't tell (every percent agrees with either, or none agrees)", eraUnclear}, {"No percent to check", eraNoPercent},
	} {
		w("| %s | %d | %.1f%% | %d | %.1f%% |\n", row.name, all.eras[row.key], pct(all.eras[row.key], all.labels), cand.eras[row.key], pct(cand.eras[row.key], cand.labels))
	}
	writeExamples(&b, examples)

	// 9. Size.
	n := len(sized)
	w("\n## 9. Size estimate\n\nOne draft record for each of the candidate set's %d GTINs, from the first on-market label read for it (the lowest DSLD ID), encoded and thrown away: an OFL branded food (`gtin:` ID, name, brand, the label's serving as text, `label.nutrients_per_serving` for the mapped rows that convert directly, `dsld_id`, provenance), with or without the other rows as written (name, amount, unit) and the label's other ingredients as text. The shape is a guess for sizing, not a decision. Gzipped at the default level.\n\n| | JSONL | Per record | Gzipped | Per record |\n| --- | --- | --- | --- | --- |\n", n)
	for _, s := range []struct {
		name string
		s    *sizer
	}{{"With every row and the other ingredients", full}, {"Mapped nutrients only", lean}} {
		w("| %s | %.1f MB | %d bytes | %.1f MB | %d bytes |\n", s.name, float64(s.s.raw.n)/1e6, s.s.raw.n/int64(max(n, 1)), float64(s.s.zipped.n)/1e6, s.s.zipped.n/int64(max(n, 1)))
	}
	w("\nA record per candidate label, not per GTIN, would be about %.2f times that (%d labels).\n", float64(cand.labels)/float64(max(n, 1)), cand.labels)

	// 10. Other things a build's rules need.
	w("\n## 10. Other things a build's rules need\n\nCounts the sections above don't hold.\n\n| | All labels | Share | Candidate set | Share |\n| --- | --- | --- | --- | --- |\n")
	for _, row := range []struct {
		name string
		a, c int
	}{
		{"A row the label gives no amount for (unit `NP`)", all.withNP, cand.withNP},
		{"A blend row (`ingredientGroup` Blend or Proprietary Blend, with or without a parenthesis)", all.withBlend, cand.withBlend},
		{"No daily values for adults and children 4 or more (only for infants, children under 4, or pregnant and lactating women)", all.noAdults, cand.noAdults},
		{"More than one serving size, each with its own amounts", all.multiServing, cand.multiServing},
		{"A serving size given as a range", all.rangeServing, cand.rangeServing},
	} {
		w("| %s | %d | %.1f%% | %d | %.1f%% |\n", row.name, row.a, pct(row.a, all.labels), row.c, pct(row.c, cand.labels))
	}
	w("\nContacts (`contacts[].contactDetails.country`: free text, often empty, sometimes a postcode) of the candidate labels:\n\n| | Candidate labels |\n| --- | --- |\n")
	w("| A contact naming a country other than the United States | %d |\n", countryOther)
	w("| Contacts naming only the United States | %d |\n", countryUS)
	w("| No contact names a country | %d |\n", countryNone)
	w("\nThe 12 most named other countries, as written:\n\n| Country | Candidate labels |\n| --- | --- |\n")
	for i, k := range countries.sorted() {
		if i == 12 {
			break
		}
		w("| %s | %d |\n", k, countries[k])
	}

	if err := os.WriteFile(filepath.Join(root, "catalogue", "SUPPLEMENTS-REPORT.md"), []byte(b.String()), 0o644); err != nil {
		return res, err
	}
	return res, nil
}

// writeExamples lists the candidate set's amounts that don't agree with their percent: the first
// by DSLD ID for each key in turn, ten that disagree and five off by a factor of 1000.
func writeExamples(b *strings.Builder, examples []dvExample) {
	sort.SliceStable(examples, func(i, j int) bool { return examples[i].id < examples[j].id })
	pick := func(verdict string, limit int) []dvExample {
		var out []dvExample
		used := map[int]bool{}
		for len(out) < limit {
			before := len(out)
			keys := map[string]bool{}
			for i, e := range examples {
				if len(out) == limit {
					break
				}
				if e.verdict != verdict || used[i] || keys[e.key] {
					continue
				}
				used[i], keys[e.key] = true, true
				out = append(out, e)
			}
			if len(out) == before {
				break
			}
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].id < out[j].id })
		return out
	}
	for _, part := range []struct {
		title, verdict string
		limit          int
	}{
		{"Ten that disagree by more than narrowly, candidate set (the lowest DSLD IDs, one key at a time):", dvDisagrees, 10},
		{"Five off by a factor of 1000, candidate set:", dvThousand, 5},
	} {
		fmt.Fprintf(b, "\n%s\n\n| DSLD ID | Product | Row | Key | Amount | Percent DV |\n| --- | --- | --- | --- | --- | --- |\n", part.title)
		for _, e := range pick(part.verdict, part.limit) {
			fmt.Fprintf(b, "| %d | %s (%s) | %s | `%s` | %s %s | %s%% |\n", e.id, cell(e.product), cell(e.brand), cell(e.name), e.key,
				strconv.FormatFloat(e.amount, 'f', -1, 64), e.unit, strconv.FormatFloat(e.percent, 'f', -1, 64))
		}
	}
}

// cell makes text safe inside a table cell.
func cell(s string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(s), " "), "|", "\\|")
}

func sortedKeys(t tally) []string {
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
