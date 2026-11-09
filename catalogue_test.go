package openfoodlog

import (
	"bufio"
	"encoding/json"
	"os"
	"testing"
)

// TestCatalogue checks every record in catalogue/foods.jsonl: valid against the food schema, an
// ofc: ID in order and used once, portions with unique IDs and at most one default.
func TestCatalogue(t *testing.T) {
	f, err := os.Open("catalogue/foods.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	v := validator(t)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	prev, count := "", 0
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
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count < 10000 {
		t.Errorf("only %d foods", count)
	}
}
