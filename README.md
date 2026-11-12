# Open Food Log

An open format for a personal food log: what someone ate, the food it matched, how much, what it held, and which numbers were guessed. The sister of [Open Workout](https://openworkout.dev) and [Open Exercise](https://openexercise.org). Site: [openfoodlog.org](https://openfoodlog.org).

**Status: v0 development draft.** The format may change until v1, and every change is listed in its changelog. v1 freezes when the first application outside Lifterclub writes it in production; after that, changes are additive.

| | |
| --- | --- |
| `schema/v0/SPEC.md` | The specification |
| `schema/v0/meal.json` | One meal or snack: items eaten, each with the person's words, the matched food, the amount, grams and nutrients, plus provenance and estimates |
| `schema/v0/food.json` | One food: nutrients per 100 g, named portions with their weight, density, identifiers and provenance |
| `schema/v0/recipe.json` | A food made of other foods: lines shaped like meal items, servings and yield |
| `schema/v0/targets.json` | A person's nutrition targets: a minimum, maximum or target per day or meal, by weekday, from a date |
| `schema/v0/log.json` | A whole log as one file: meals, the person's own foods, recipes, targets and deleted meals |
| `schema/v0/nutrients.json` | The nutrient keys, each ending in its unit, mapped to FAO INFOODS tagnames, USDA FoodData Central numbers, Health Connect and HealthKit |
| `schema/v0/amount-vectors.json` | Worked cases every implementation must agree on: an amount on a food, as grams, millilitres or servings, then as nutrients |
| `schema/v0/recipe-vectors.json`, `target-vectors.json` | The same for a recipe's yield and nutrients, and for which target applies and whether it was met |
| `schema/README.md` | Versions and the registry of `x-<vendor>-` extensions |
| `schema/CHANGELOG.md` | What changed in v0 |
| `examples` | Foods (generic, packaged with its label, a supplement), meals, a recipe, targets and a log |
| `mcp/tools.json` | The MCP tool profile, generated from the reference server |
| `catalogue` | The Open Food Catalogue: 12,442 generic foods with `ofc:` IDs from USDA FoodData Central, CC0. See its README |
| `openfoodlog.go` | Package `openfoodlog`: the embedded schemas, a validator, and the reference calculation of section 6 (`Measure`; `Grams` and `Nutrients` for weighed amounts) |
| `cmd/ofl` | `ofl validate`, which checks documents against the schemas; `ofl catalogue`, which rebuilds the catalogue from FoodData Central; and `ofl site`, which builds the website |

```bash
go run ./cmd/ofl validate
go test ./...
go run ./cmd/ofl site dist
```

Until v1 these files are published here from [Lifterclub](https://lifterclub.com), the reference implementation. Issues and proposals belong here.

Licence: Apache-2.0 for the code and schemas. The catalogue is CC0 1.0, like USDA FoodData Central, where its numbers and the examples' come from.
