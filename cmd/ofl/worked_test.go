package main

import (
	"reflect"
	"testing"
)

func TestDeriveNutrients(t *testing.T) {
	cases := []struct {
		name   string
		in     map[string]float64
		want   map[string]float64
		worked []string
	}{
		{
			name:   "all three",
			in:     map[string]float64{"energy_kcal": 100, "sodium_mg": 574, "carbohydrate_g": 7.46, "fiber_g": 1.2},
			want:   map[string]float64{"energy_kcal": 100, "energy_kj": 418.4, "sodium_mg": 574, "salt_g": 1.435, "carbohydrate_g": 7.46, "fiber_g": 1.2, "carbohydrate_available_g": 6.26},
			worked: []string{"energy_kj", "salt_g", "carbohydrate_available_g"},
		},
		{
			name: "FDC's own kJ wins",
			in:   map[string]float64{"energy_kcal": 100, "energy_kj": 420},
			want: map[string]float64{"energy_kcal": 100, "energy_kj": 420},
		},
		{
			name: "no fibre, no available carbohydrate",
			in:   map[string]float64{"carbohydrate_g": 10},
			want: map[string]float64{"carbohydrate_g": 10},
		},
		{
			name: "fibre above the total",
			in:   map[string]float64{"carbohydrate_g": 1, "fiber_g": 1.5},
			want: map[string]float64{"carbohydrate_g": 1, "fiber_g": 1.5},
		},
		{
			name:   "zero fibre",
			in:     map[string]float64{"carbohydrate_g": 4.8, "fiber_g": 0},
			want:   map[string]float64{"carbohydrate_g": 4.8, "fiber_g": 0, "carbohydrate_available_g": 4.8},
			worked: []string{"carbohydrate_available_g"},
		},
	}
	for _, c := range cases {
		worked := deriveNutrients(c.in)
		if !reflect.DeepEqual(c.in, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, c.in, c.want)
		}
		if !reflect.DeepEqual(worked, c.worked) {
			t.Errorf("%s: worked out %v, want %v", c.name, worked, c.worked)
		}
	}
}

func TestCategory(t *testing.T) {
	var sr fdcFood
	sr.FoodCategory.Description = "Baked Products"
	if got := category(sr); got == nil || *got != (categoryDoc{Name: "Baked Products", Scheme: "fdc", Basis: "inferred"}) {
		t.Errorf("SR Legacy: %+v", got)
	}
	var survey fdcFood
	survey.WweiaFoodCategory.Description = "Milk, whole"
	survey.FoodAttributes = append(survey.FoodAttributes, struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{"WWEIA Category number", "1002"})
	if got := category(survey); got == nil || *got != (categoryDoc{Name: "Milk, whole", Code: "1002", Scheme: "fdc", Basis: "inferred"}) {
		t.Errorf("survey: %+v", got)
	}
	if got := category(fdcFood{}); got != nil {
		t.Errorf("none: %+v", got)
	}
}
