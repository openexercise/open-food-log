package main

import (
	"bytes"
	"encoding/json"
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

func TestAcceptableAlias(t *testing.T) {
	cases := []struct {
		alias, name string
		want        bool
	}{
		{"hard boiled egg", "Egg, whole, cooked, hard-boiled", true},
		{"2% milk", "Milk, reduced fat (2%)", true},
		{"2 eggs", "Egg, whole, raw", false},
		{"Oj", "Orange juice, raw", false},
		{" oj", "Orange juice, raw", false},
		{"x", "X", false},
	}
	for _, c := range cases {
		if got := acceptableAlias(c.alias, c.name); got != c.want {
			t.Errorf("acceptableAlias(%q, %q) = %v", c.alias, c.name, got)
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
