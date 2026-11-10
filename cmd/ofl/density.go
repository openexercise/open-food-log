package main

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Densities let an amount in ml, l or fl oz be weighed (OFL §6). They come only from FoodData
// Central itself, for liquids: a measured specific gravity where the food has one, else the weight
// FDC gives for one of the food's own volume portions ("1 cup" of whole milk is 244 g, so 244 g in
// 236.588 ml). Nothing is assumed, and solids measured by the cup (chopped, sliced) get none.

// mlPer is each volume unit FoodData Central uses, in US millilitres.
var mlPer = map[string]float64{
	"cup": 236.5882365, "cups": 236.5882365,
	"fl oz": 29.5735295625,
	"tbsp":  14.78676478125, "tablespoon": 14.78676478125, "tablespoons": 14.78676478125,
	"tsp": 4.92892159375, "teaspoon": 4.92892159375, "teaspoons": 4.92892159375,
	"pint": 473.176473, "quart": 946.352946, "gallon": 3785.411784,
	"ml": 1, "liter": 1000, "l": 1000,
}

var (
	// plainVolume is a portion that is only a volume: "1 cup", "0.5 cup", "1 fl oz". Survey
	// drinks qualify theirs: "(no ice)" and "(NFS)" are the drink alone; "(with ice)" isn't.
	plainVolume = regexp.MustCompile(`^(\d+(?:\.\d+)?) (cups?|fl oz|tbsp|tablespoons?|tsp|teaspoons?|pint|quart|gallon|ml|liter|l)(?: \((?:no ice|NFS)\))?$`)
	// statedFlOz is a container that states its volume: "1 bottle (12 fl oz)", "1 cup (8 fl oz)".
	statedFlOz = regexp.MustCompile(`^1 [^()]*\((\d+(?:\.\d+)?) fl oz\)$`)
)

// portionML is the volume a portion's label states, or 0 when it isn't a plain volume.
func portionML(label string) float64 {
	if strings.Contains(label, "ice") && !strings.Contains(label, "no ice") {
		return 0
	}
	if m := statedFlOz.FindStringSubmatch(label); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v * mlPer["fl oz"]
	}
	if m := plainVolume.FindStringSubmatch(label); m != nil {
		v, _ := strconv.ParseFloat(m[1], 64)
		return v * mlPer[m[2]]
	}
	return 0
}

// surveyLiquids are the WWEIA categories (FNDDS survey foods) that are drinks or soups throughout.
var surveyLiquids = map[string]bool{
	"Apple juice": true, "Baby juice": true, "Baby water": true, "Beer": true, "Bottled water": true,
	"Citrus juice": true, "Coffee": true, "Cream and cream substitutes": true, "Diet soft drinks": true,
	"Diet sport and energy drinks": true, "Enhanced water": true, "Flavored milk, lowfat": true,
	"Flavored milk, nonfat": true, "Flavored milk, reduced fat": true, "Flavored milk, whole": true,
	"Flavored or carbonated water": true, "Formula, prepared from powder": true, "Formula, ready-to-feed": true,
	"Fruit drinks": true, "Human milk": true, "Liquor and cocktails": true, "Milk shakes and other dairy drinks": true,
	"Milk, lowfat": true, "Milk, nonfat": true, "Milk, reduced fat": true, "Milk, whole": true,
	"Nutritional beverages": true, "Other diet drinks": true, "Other fruit juice": true, "Plant-based milk": true,
	"Ramen and Asian broth-based soups": true, "Smoothies and grain drinks": true, "Soft drinks": true,
	"Soups, broth-based": true, "Soups, cream-based": true, "Sport and energy drinks": true, "Tap water": true,
	"Tea": true, "Vegetable juice": true, "Wine": true,
}

// liquidNames are the liquids in categories that also hold solids, by the words of their names.
// "Beverages" (SR Legacy, Foundation) is a drink throughout, so any name will do.
var liquidNames = map[string]*regexp.Regexp{
	"Beverages":                          regexp.MustCompile(``),
	"Dairy and Egg Products":             regexp.MustCompile(`^(Milk,|Buttermilk|Kefir|Eggnog|Cream, fluid|Cream, heavy|Cream, half and half|Half and half|Cream substitute, liquid)|(?i)\b(yogurt drink|drinkable yogurt)\b`),
	"Fats and Oils":                      regexp.MustCompile(`^Oil\b`),
	"Fruits and Fruit Juices":            regexp.MustCompile(`(?i)\b(juice|nectar)\b`),
	"Vegetables and Vegetable Products":  regexp.MustCompile(`(?i)\bjuice\b`),
	"Legumes and Legume Products":        regexp.MustCompile(`^(Soymilk|Soy milk|Soy sauce)\b`),
	"Nut and Seed Products":              regexp.MustCompile(`(?i)\b(milk|coconut water)\b`),
	"Soups, Sauces, and Gravies":         regexp.MustCompile(`^(Soup|Broth|Stock)\b|\b(broth|stock|bouillon)\b`),
	"Spices and Herbs":                   regexp.MustCompile(`^Vinegar\b`),
	"Sweets":                             regexp.MustCompile(`(?i)^(syrups?|honey|molasses|fruit syrup)\b`),
	"Baby Foods":                         regexp.MustCompile(`(?i)\b(juice|formula|water)\b`),
	"Salad dressings and vegetable oils": regexp.MustCompile(`(?i)(^|\b)oil\b`),
	"Jams, syrups, toppings":             regexp.MustCompile(`(?i)\bsyrups?\b`),
	"Sugars and honey":                   regexp.MustCompile(`(?i)\bhoney\b`),
	"Mustard and other condiments":       regexp.MustCompile(`(?i)\b(vinegar|juice)\b`),
	"Soy-based condiments":               regexp.MustCompile(`^(Soy sauce|Fish sauce)\b`),
	"Yogurt, regular":                    regexp.MustCompile(`(?i)\b(drink|drinkable|kefir|lassi)\b`),
	"Yogurt, Greek":                      regexp.MustCompile(`(?i)\b(drink|drinkable|kefir|lassi)\b`),
}

// notLiquid are words that make a food in a liquid category something else: a powder or mix
// before it's made up, a whipped topping, a juice-packed fruit. A powder "prepared" or "made with"
// water, or "reconstituted", is a liquid again, but one "unprepared" or "not reconstituted" is not.
var (
	notLiquid = regexp.MustCompile(`(?i)\b(dry|dried|dehydrated|freeze-dried|powder|powdered|crystals|granules|whipped|leaves|juice pack|in juice)\b`)
	// SR Legacy names an instant powder "instant" alone; survey foods add "dry" or "not reconstituted".
	srInstant = regexp.MustCompile(`(?i)\binstant\b`)
	madeUp    = regexp.MustCompile(`(?i)\b(prepared|reconstituted|made with)\b`)
	notMadeUp = regexp.MustCompile(`(?i)\b(unprepared|not reconstituted|not prepared)\b`)
)

// isLiquid says whether a food is a liquid, by its FoodData Central category and name.
func isLiquid(f fdcFood) bool {
	name := f.Description
	if notMadeUp.MatchString(name) || (notLiquid.MatchString(name) && !madeUp.MatchString(name)) {
		return false
	}
	if cat := f.WweiaFoodCategory.Description; cat != "" {
		if surveyLiquids[cat] {
			return true
		}
		re, ok := liquidNames[cat]
		return ok && re.MatchString(name)
	}
	if srInstant.MatchString(name) && !madeUp.MatchString(name) {
		return false
	}
	re, ok := liquidNames[f.FoodCategory.Description]
	return ok && re.MatchString(name)
}

// specificGravity is FoodData Central's nutrient 227, measured relative to water.
const specificGravity = "227"

// A liquid food weighs between about 0.85 g/ml (oils are 0.92) and 1.45 g/ml (corn syrup is 1.44).
// A portion outside that isn't the liquid's volume alone: a powder by the spoon, a shake's foam,
// a can whose label and weight disagree. Its value is left out, not corrected.
const minDensity, maxDensity = 0.85, 1.45

// density finds a liquid's grams per millilitre and says where it came from, or why it has none.
// It reads the record's own portions, so anyone can check it against the record.
func density(f fdcFood, food *catalogueFood) (float64, string, bool) {
	for _, n := range f.FoodNutrients {
		if n.Nutrient.Number == specificGravity && n.Amount != nil && *n.Amount > 0 {
			return *n.Amount, fmt.Sprintf("specific gravity %s (nutrient 227), as g/ml", strconv.FormatFloat(*n.Amount, 'f', -1, 64)), true
		}
	}
	var best portionDoc
	bestML := 0.0
	for _, p := range food.Portions {
		if ml := portionML(p.Label); ml > bestML {
			best, bestML = p, ml
		}
	}
	if bestML == 0 {
		return 0, "no portion in ml, fl oz, cups or spoons", false
	}
	d := math.Round(best.Grams/bestML*1e4) / 1e4
	how := fmt.Sprintf("portion %q, %s g in %s ml", best.Label, strconv.FormatFloat(best.Grams, 'f', -1, 64), strconv.FormatFloat(math.Round(bestML*1000)/1000, 'f', -1, 64))
	if d < minDensity || d > maxDensity {
		return 0, fmt.Sprintf("%s is %s g/ml, outside %.2f to %.2f", how, strconv.FormatFloat(d, 'f', -1, 64), minDensity, maxDensity), false
	}
	return d, how, true
}
