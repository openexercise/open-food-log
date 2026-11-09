package main

import (
	"slices"
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
	rules, generated := attachAliases(food, model)
	if rules != 1 || generated != 2 {
		t.Fatalf("rules %d, generated %d; want 1 and 2", rules, generated)
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
