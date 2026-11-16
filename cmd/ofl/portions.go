package main

import "strings"

// beverageTemplate is the portion set FNDDS gives a drink: an ounce with ice, without ice and
// not further specified, and a 248 g glass when the amount isn't said. The survey download also
// gives it, unchanged, to ingredient-only codes (FNDDS 9999xxxx: "Tomatoes as ingredient in
// omelet", "Industrial oil as ingredient in food"), which FoodData Central's own API lists with no
// portions at all.
var beverageTemplate = map[string]float64{
	"1 fl oz (with ice)":     23,
	"1 fl oz (no ice)":       31,
	"1 fl oz (NFS)":          31,
	"Quantity not specified": 248,
}

// placeholderPortionsNote ends the source that says a record's portions were left out.
const placeholderPortionsNote = ": portions left out (a drink's placeholder on an ingredient-only code; FoodData Central's API lists none)"

// hasPlaceholderPortions says a food's portions are the beverage template on a survey food filed
// under "Not included in a food category" (WWEIA 9999): a placeholder, not portions of the food.
// Lemon juice and other drinks keep the same measures, since they're real for a drink.
func hasPlaceholderPortions(food *catalogueFood) bool {
	if food.Category == nil || food.Category.Code != "9999" || len(food.Portions) != len(beverageTemplate) {
		return false
	}
	for _, p := range food.Portions {
		if grams, ok := beverageTemplate[p.Label]; !ok || grams != p.Grams {
			return false
		}
	}
	return true
}

// dropPlaceholderPortions leaves placeholder portions out and says so in the record's sources,
// naming the source the portions came from. It reports whether it changed the food.
func dropPlaceholderPortions(food *catalogueFood) bool {
	if !hasPlaceholderPortions(food) {
		return false
	}
	food.Portions = nil
	from := ""
	for _, s := range food.Provenance.Sources {
		if s.ID != "" && !strings.Contains(s.Name, ": ") {
			from = s.Name
			break
		}
	}
	note := food.Provenance.Sources[0]
	if from != "" {
		note.Name = from
	}
	note.Name += placeholderPortionsNote
	food.Provenance.Sources = append(food.Provenance.Sources, note)
	return true
}
