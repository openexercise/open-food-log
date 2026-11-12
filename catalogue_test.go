package openfoodlog

import (
	"bufio"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

// TestCatalogue checks every record in catalogue/foods.jsonl: valid against the food schema, an
// ofc: ID in order and used once, portions with unique IDs and at most one default, and a source
// for every density.
func TestCatalogue(t *testing.T) {
	f, err := os.Open("catalogue/foods.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	v := validator(t)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	prev, count, densities := "", 0, 0
	for scanner.Scan() {
		line := scanner.Bytes()
		count++
		if problems := v.ValidateAs("food", line); problems != nil {
			t.Fatalf("line %d: %v", count, problems)
		}
		var food Food
		if err := json.Unmarshal(line, &food); err != nil {
			t.Fatal(err)
		}
		if food.ID <= prev {
			t.Errorf("line %d: %s is out of order or repeated", count, food.ID)
		}
		prev = food.ID
		ids, defaults := map[string]bool{}, 0
		for _, p := range food.Portions {
			if ids[p.ID] {
				t.Errorf("%s: portion %s twice", food.ID, p.ID)
			}
			ids[p.ID] = true
			if p.IsDefault {
				defaults++
			}
		}
		if defaults > 1 {
			t.Errorf("%s: %d default portions", food.ID, defaults)
		}
		if _, ok := food.NutrientsPer100g["energy_kcal"]; !ok {
			t.Errorf("%s: no energy", food.ID)
		}
		checkWorkedOut(t, food, string(line))
		if food.DensityGPerML > 0 {
			densities++
			if !strings.Contains(string(line), `density_g_per_ml from `) {
				t.Errorf("%s: a density with no source in provenance", food.ID)
			}
		}
		if food.ID == "ofc:milk-whole" || food.ID == "ofc:milk-nfs" {
			if food.DensityGPerML == 0 {
				t.Errorf("%s: no density, so 250 ml can't be weighed", food.ID)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count < 10000 {
		t.Errorf("only %d foods", count)
	}
	if densities < 1000 {
		t.Errorf("only %d foods with a density", densities)
	}
}

// checkWorkedOut holds the values the catalogue works out to section 4's rules: salt is sodium ×
// 2.5, available carbohydrate is carbohydrate less fibre, and each needs what it's worked out from.
func checkWorkedOut(t *testing.T, food Food, line string) {
	t.Helper()
	n := food.NutrientsPer100g
	if salt, ok := n["salt_g"]; ok {
		if na, has := n["sodium_mg"]; !has || math.Abs(salt-na*0.0025) > 0.00001 {
			t.Errorf("%s: salt_g %v isn't sodium × 2.5", food.ID, salt)
		}
	}
	if avail, ok := n["carbohydrate_available_g"]; ok {
		carb, hasCarb := n["carbohydrate_g"]
		fiber, hasFiber := n["fiber_g"]
		if !hasCarb || !hasFiber || math.Abs(avail-(carb-fiber)) > 0.001 {
			t.Errorf("%s: carbohydrate_available_g %v isn't carbohydrate − fibre", food.ID, avail)
		}
	}
	// FoodData Central's own kJ stands even where it disagrees with its kcal (SR Legacy's brown rice
	// flour is 1580 kJ and 363 kcal); only a worked-out value must be kcal × 4.184.
	// The build names the keys it worked out first in a source's name ("…: energy_kj, salt_g …").
	if strings.Contains(line, `: energy_kj`) {
		if kj := n["energy_kj"]; math.Abs(kj-n["energy_kcal"]*4.184) > 0.05 {
			t.Errorf("%s: worked-out energy_kj %v isn't energy_kcal × 4.184", food.ID, kj)
		}
	}
}
