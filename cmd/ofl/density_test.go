package main

import (
	"math"
	"testing"
)

func TestPortionML(t *testing.T) {
	cases := map[string]float64{
		"1 cup":                    236.5882365,
		"0.5 cup":                  118.29411825,
		"1 fl oz":                  29.5735295625,
		"1 fl oz (no ice)":         29.5735295625,
		"1 fl oz (NFS)":            29.5735295625,
		"2 tbsp":                   29.5735295625,
		"1 cup (8 fl oz)":          236.588236,
		"1 bottle (12 fl oz)":      354.88235475,
		"1 fl oz (with ice)":       0, // the ice takes up room
		"1 cup, chopped":           0, // a solid by the cup
		"1 tablespoon, liquid oil": 0,
		"Quantity not specified":   0,
		"1 package (10 oz)":        0, // a weight
	}
	for label, want := range cases {
		if got := portionML(label); math.Abs(got-want) > 0.001 {
			t.Errorf("portionML(%q) = %v, want %v", label, got, want)
		}
	}
}

func TestIsLiquid(t *testing.T) {
	sr := func(category, name string) fdcFood {
		var f fdcFood
		f.Description, f.FoodCategory.Description = name, category
		return f
	}
	survey := func(category, name string) fdcFood {
		var f fdcFood
		f.Description, f.WweiaFoodCategory.Description = name, category
		return f
	}
	cases := []struct {
		food fdcFood
		want bool
	}{
		{survey("Milk, whole", "Milk, whole"), true},
		{survey("Coffee", "Coffee, instant, reconstituted"), true},
		{survey("Not included in a food category", "Coffee, instant, not reconstituted"), false},
		{survey("Formula, prepared from powder", "Infant formula, Similac Advance, powder, made with tap water"), true},
		{survey("Mustard and other condiments", "Lemon juice, 100%, freshly squeezed"), true},
		{survey("Mustard and other condiments", "Worcestershire sauce"), false},
		{sr("Dairy and Egg Products", "Milk, whole, 3.25% milkfat, with added vitamin D"), true},
		{sr("Dairy and Egg Products", "Milk, dry, whole, with added vitamin D"), false},
		{sr("Dairy and Egg Products", "Milk dessert bar, frozen, made from lowfat milk"), false},
		{sr("Beverages", "Beverages, coffee, instant, regular, half the caffeine"), false},
		{sr("Beverages", "Beverages, coffee, instant, regular, prepared with water"), true},
		{sr("Fats and Oils", "Oil, olive, salad or cooking"), true},
		{sr("Fats and Oils", "Margarine, regular, 80% fat"), false},
		{sr("Fruits and Fruit Juices", "Orange juice, raw"), true},
		{sr("Fruits and Fruit Juices", "Peaches, canned, juice pack, solids and liquids"), false},
		{sr("Spices and Herbs", "Vinegar, cider"), true},
		{sr("Sweets", "Syrups, maple"), true},
		{sr("Sweets", "Toppings, nuts in syrup"), false},
		{sr("Soups, Sauces, and Gravies", "Soup, chicken broth, ready-to-serve"), true},
		{sr("Soups, Sauces, and Gravies", "Soup, onion, dry, mix"), false},
	}
	for _, c := range cases {
		if got := isLiquid(c.food); got != c.want {
			t.Errorf("isLiquid(%q) = %v, want %v", c.food.Description, got, c.want)
		}
	}
}

func TestDensity(t *testing.T) {
	food := &catalogueFood{Portions: []portionDoc{{Label: "1 fl oz", Grams: 30.5}, {Label: "1 cup", Grams: 244}}}
	d, how, ok := density(fdcFood{}, food)
	if !ok || d != 1.0313 || how != `portion "1 cup", 244 g in 236.588 ml` {
		t.Errorf("density = %v, %q, %v; want the cup's 1.0313", d, how, ok)
	}
	// A portion whose weight and volume disagree past any liquid's density is left out.
	food = &catalogueFood{Portions: []portionDoc{{Label: "1 can (8 fl oz)", Grams: 355}}}
	if _, _, ok := density(fdcFood{}, food); ok {
		t.Error("kept a density of 1.5 g/ml")
	}
}
