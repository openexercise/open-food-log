package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestRuleAliases(t *testing.T) {
	cases := map[string][]string{
		"Apple, raw":                       {"raw apple"},
		"Oatmeal, NFS":                     {"oatmeal"},
		"Banana, baked":                    {"baked banana"},
		"Rice, white, puffed":              nil, // three parts: left to the model
		"Apples, with skin":                nil, // a clause, not a modifier
		"Peanut butter":                    nil, // already what people say
		"Chicken breast, oven-roasted":     nil,
		"Milk, reduced fat (2%)":           nil,
		"Salmon, Atlantic, farmed, raw":    nil,
		"Beans, from canned, no added fat": nil,
		"Ham and noodles, no sauce":        nil, // a negation, not a modifier
		"Pancakes, school":                 nil,
	}
	for name, want := range cases {
		if got := ruleAliases(name); !slices.Equal(got, want) {
			t.Errorf("ruleAliases(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestAttachAliases(t *testing.T) {
	food := &catalogueFood{ID: "ofc:apple-raw", Name: "Apple, raw"}
	model := map[string]modelAliases{"ofc:apple-raw": {Aliases: []string{"apple", "raw apple", "Apple pie", "an apple"}, Model: "m", At: "2026-11-09"}}
	got := attachAliases(food, nil, model)
	if got != (aliasCounts{rules: 1, generated: 2}) {
		t.Fatalf("attached %+v; want 1 rule and 2 generated", got)
	}
	var names []string
	for _, a := range food.Aliases {
		names = append(names, a.Name)
	}
	if !slices.Equal(names, []string{"raw apple", "apple", "an apple"}) {
		t.Errorf("aliases %q", names)
	}
	if !slices.Equal(food.Provenance.Generated, []string{"/aliases/1", "/aliases/2"}) {
		t.Errorf("generated %q", food.Provenance.Generated)
	}
	if len(food.Provenance.Sources) != 1 || food.Provenance.Sources[0].Name != "Aliases suggested by m" {
		t.Errorf("sources %+v", food.Provenance.Sources)
	}
}

func TestCuratedAliases(t *testing.T) {
	food := &catalogueFood{ID: "ofc:apple-raw", Name: "Apple, raw"}
	curated := map[string]curatedAliases{"ofc:apple-raw": {Aliases: []string{"apple", "raw apple"}}}
	model := map[string]modelAliases{"ofc:apple-raw": {Aliases: []string{"apple", "eating apple"}, Model: "m", At: "2026-11-09"}}
	got := attachAliases(food, curated, model)
	if got != (aliasCounts{rules: 1, curated: 1, generated: 1}) {
		t.Fatalf("attached %+v", got)
	}
	// A person's alias isn't a model's: only the model's is listed as generated.
	if !slices.Equal(food.Provenance.Generated, []string{"/aliases/2"}) {
		t.Errorf("generated %q", food.Provenance.Generated)
	}
	if len(food.Provenance.Sources) != 2 || food.Provenance.Sources[0].Name != curatedSourceName {
		t.Errorf("sources %+v", food.Provenance.Sources)
	}

	// Taking them off and attaching them again changes nothing.
	before, _ := json.Marshal(food)
	dropAliases(food)
	attachAliases(food, curated, model)
	if after, _ := json.Marshal(food); !bytes.Equal(before, after) {
		t.Errorf("attaching again changed the food:\n%s\n%s", before, after)
	}
}

func TestCheckCurated(t *testing.T) {
	names := map[string]string{"ofc:egg-whole-raw": "Egg, whole, raw"}
	for _, c := range []struct {
		curated curatedAliases
		want    string
	}{
		{curatedAliases{ID: "ofc:egg-whole-raw", Aliases: []string{"raw egg"}}, ""},
		{curatedAliases{ID: "ofc:no-such-food", Aliases: []string{"egg"}}, "isn't in the catalogue"},
		{curatedAliases{ID: "ofc:egg-whole-raw", Aliases: []string{"Raw Egg"}}, "lowercase"},
		{curatedAliases{ID: "ofc:egg-whole-raw", Aliases: []string{"2 eggs"}}, "lowercase"},
	} {
		err := checkCurated(map[string]curatedAliases{c.curated.ID: c.curated}, names)
		if (err == nil) != (c.want == "") || (err != nil && !strings.Contains(err.Error(), c.want)) {
			t.Errorf("%+v: %v, want %q", c.curated, err, c.want)
		}
	}
}

// TestCuratedEggs keeps the reason the curated file exists: each everyday egg phrase names one
// food, with one egg as its usual portion, so "2 eggs" can be counted without asking.
func TestCuratedEggs(t *testing.T) {
	curated, err := readCuratedAliases(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	holders := map[string][]string{}
	for id, c := range curated {
		for _, a := range c.Aliases {
			holders[normalize(a)] = append(holders[normalize(a)], id)
		}
	}
	for _, phrase := range []string{"egg", "fried egg", "boiled egg", "scrambled egg", "omelet", "omelette", "egg white", "cheese omelette", "omelet with cheese", "scrambled eggs with cheese"} {
		if len(holders[phrase]) != 1 {
			t.Errorf("%q is held by %v, want one food", phrase, holders[phrase])
		}
	}
}

// TestAliasChecks holds the built catalogue to sources/alias-checks.jsonl: each phrase is a name or
// alias that one food alone holds, the food named, which a matcher can take as a sure match, or,
// with the ID "none", a name no food holds, so a matcher asks. A
// model run that moves a plain name, or gives it to two foods, fails here.
func TestAliasChecks(t *testing.T) {
	root := filepath.Join("..", "..")
	data, err := os.ReadFile(filepath.Join(root, "sources", "alias-checks.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(root, "catalogue", "foods.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	holders := map[string][]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var r struct {
			ID      string     `json:"id"`
			Name    string     `json:"name"`
			Aliases []aliasDoc `json:"aliases"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		names := []string{normalize(r.Name)}
		for _, a := range r.Aliases {
			names = append(names, normalize(a.Name))
		}
		for _, n := range names {
			if !slices.Contains(holders[n], r.ID) {
				holders[n] = append(holders[n], r.ID)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var c struct{ Phrase, ID string }
		if err := json.Unmarshal([]byte(line), &c); err != nil || c.Phrase == "" || c.ID == "" {
			t.Fatalf("alias-checks.jsonl line %d: want {phrase, id}: %v", i+1, err)
		}
		got := holders[normalize(c.Phrase)]
		if c.ID == "none" {
			if len(got) > 0 {
				t.Errorf("%q is held by %v, want no food (a matcher asks)", c.Phrase, got)
			}
			continue
		}
		if !slices.Equal(got, []string{c.ID}) {
			t.Errorf("%q is held by %v, want %s alone", c.Phrase, got, c.ID)
		}
	}
}

func TestSoleModelAliases(t *testing.T) {
	withDefault := []portionDoc{{ID: "q", Label: "Quantity not specified", Grams: 100, IsDefault: true}}
	foods := []*catalogueFood{
		{ID: "ofc:meat-loaf-ns", Name: "Meat loaf, NS as to type of meat", Portions: withDefault},
		{ID: "ofc:beef-loaf", Name: "Beef, ground, loaf, cooked"},
		{ID: "ofc:fries-fast-food", Name: "Fast foods, potato, french fried", Portions: withDefault},
		{ID: "ofc:fries-home", Name: "Potato, french fries, home made", Portions: withDefault},
		{ID: "ofc:fries-frozen", Name: "Potatoes, french fried, frozen"},
		{ID: "ofc:oatmeal-nfs", Name: "Oatmeal, NFS"},
		{ID: "ofc:oatmeal-instant", Name: "Oatmeal, instant"},
	}
	model := map[string]modelAliases{
		"ofc:meat-loaf-ns":    {Aliases: []string{"meatloaf"}},
		"ofc:beef-loaf":       {Aliases: []string{"meatloaf", "beef loaf"}},
		"ofc:fries-fast-food": {Aliases: []string{"french fries", "fast food fries"}},
		"ofc:fries-home":      {Aliases: []string{"french fries"}},
		"ofc:fries-frozen":    {Aliases: []string{"french fries"}},
		"ofc:oatmeal-instant": {Aliases: []string{"oatmeal", "instant oatmeal"}}, // "oatmeal" is the NFS record's by rule
	}
	got := soleModelAliases(foods, nil, model, map[string]bool{"fast food fries": true})
	want := map[string][]string{
		"ofc:meat-loaf-ns":    {"meatloaf"}, // unspecified wins
		"ofc:beef-loaf":       {"beef loaf"},
		"ofc:fries-fast-food": nil,              // "fast food fries" is withheld
		"ofc:fries-home":      {"french fries"}, // a default portion and the shorter name
		"ofc:fries-frozen":    nil,
		"ofc:oatmeal-instant": {"instant oatmeal"},
	}
	for id, w := range want {
		if !slices.Equal(got[id].Aliases, w) {
			t.Errorf("%s: %q, want %q", id, got[id].Aliases, w)
		}
	}
}

func TestCheckWithheld(t *testing.T) {
	withheld := map[string]bool{"baby peaches": true}
	if err := checkWithheld(withheld, map[string]curatedAliases{"ofc:x": {Aliases: []string{"peaches"}}}); err != nil {
		t.Errorf("a different name: %v", err)
	}
	if err := checkWithheld(withheld, map[string]curatedAliases{"ofc:x": {Aliases: []string{"Baby Peaches"}}}); err == nil {
		t.Error("a name both withheld and curated passed")
	}
}
