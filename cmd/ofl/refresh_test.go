package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCatalogueIsRefreshed fails when a record rule or sources/ changed and `ofl catalogue
// refresh` wasn't run, so the committed catalogue is always what its rules give.
func TestCatalogueIsRefreshed(t *testing.T) {
	root := filepath.Join("..", "..")
	foods, report, err := refreshCatalogue(root)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{"foods.jsonl": foods, "REPORT.md": report} {
		have, err := os.ReadFile(filepath.Join(root, "catalogue", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(have, want) {
			t.Errorf("catalogue/%s is stale: run go run ./cmd/ofl catalogue refresh", name)
		}
	}
}

func TestPlaceholderPortions(t *testing.T) {
	template := func() []portionDoc {
		return []portionDoc{
			{ID: "1-fl-oz-with-ice", Label: "1 fl oz (with ice)", Grams: 23},
			{ID: "quantity-not-specified", Label: "Quantity not specified", Grams: 248, IsDefault: true},
			{ID: "1-fl-oz-no-ice", Label: "1 fl oz (no ice)", Grams: 31},
			{ID: "1-fl-oz-nfs", Label: "1 fl oz (NFS)", Grams: 31},
		}
	}
	survey := sourceDoc{Name: "USDA FoodData Central, FNDDS survey foods (2024-10-31)", ID: "2710807"}
	ingredient := &catalogueFood{
		ID: "ofc:tomatoes-as-ingredient-in-omelet", Portions: template(),
		Category:   &categoryDoc{Name: "Not included in a food category", Code: "9999"},
		Provenance: provenanceDoc{Sources: []sourceDoc{survey, {Name: survey.Name + ": energy_kj worked out by OFL v0 section 4", ID: survey.ID}}},
	}
	if !dropPlaceholderPortions(ingredient) || ingredient.Portions != nil {
		t.Fatalf("the ingredient kept %+v", ingredient.Portions)
	}
	last := ingredient.Provenance.Sources[len(ingredient.Provenance.Sources)-1]
	if last.Name != survey.Name+placeholderPortionsNote || last.ID != survey.ID {
		t.Errorf("the note is %+v", last)
	}
	if dropPlaceholderPortions(ingredient) {
		t.Error("dropping twice changed the food again")
	}

	// A drink keeps the same measures: they're real for it.
	juice := &catalogueFood{ID: "ofc:lemon-juice", Portions: template(), Category: &categoryDoc{Name: "Mustard and other condiments", Code: "5702"}, Provenance: provenanceDoc{Sources: []sourceDoc{survey}}}
	// An ingredient code with portions of its own keeps them, and so does one with an extra portion.
	herb := &catalogueFood{ID: "ofc:basil", Portions: []portionDoc{{ID: "1-tablespoon", Label: "1 tablespoon", Grams: 3}}, Category: &categoryDoc{Code: "9999"}, Provenance: provenanceDoc{Sources: []sourceDoc{survey}}}
	extra := &catalogueFood{ID: "ofc:x", Portions: append(template()[:3], portionDoc{ID: "1-cup", Label: "1 cup", Grams: 240}), Category: &categoryDoc{Code: "9999"}, Provenance: provenanceDoc{Sources: []sourceDoc{survey}}}
	for _, f := range []*catalogueFood{juice, herb, extra} {
		if dropPlaceholderPortions(f) || len(f.Portions) == 0 {
			t.Errorf("%s lost its portions", f.ID)
		}
	}
}

func TestWithPortionCounts(t *testing.T) {
	report := "x\n| Source | Read | Kept without portions |\n| --- | --- | --- |\n| " + sources[0].label + " | 1 | 2 | 3 |\n| " + sources[1].label + " | 4 | 5 | 6 |\n| " + sources[2].label + " | 7 | 8 | 9 |\n\n| " + sources[1].label + " | 99 |\n"
	got, err := withPortionCounts([]byte(report), map[string]int{sources[1].label: 42})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "| "+sources[1].label+" | 4 | 5 | 42 |") || !strings.Contains(string(got), "| "+sources[0].label+" | 1 | 2 | 0 |") {
		t.Errorf("got %s", got)
	}
	if !strings.Contains(string(got), "| "+sources[1].label+" | 99 |") {
		t.Errorf("a later table changed: %s", got)
	}
	if _, err := withPortionCounts([]byte("no table"), nil); err == nil {
		t.Error("a report without the table was accepted")
	}
}
