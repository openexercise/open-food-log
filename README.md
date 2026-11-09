# Open Food Log

An open format for a personal food log: what someone ate, the food it matched, how much, what it held, and which numbers were guessed. The sister of [Open Workout](https://openworkout.dev) and [Open Exercise](https://openexercise.org). Site: [openfoodlog.org](https://openfoodlog.org).

**Status: v0 development draft.** The format may change until v1, and every change is listed in its changelog. v1 freezes when the first application outside Lifterclub writes it in production; after that, changes are additive.

| | |
| --- | --- |
| `schema/v0/SPEC.md` | The specification |
| `schema/v0/meal.json` | One meal or snack: items eaten, each with the person's words, the matched food, the amount, grams and nutrients, plus provenance and estimates |
| `schema/v0/food.json` | One food: nutrients per 100 g, named portions with their weight, density, identifiers and provenance |
| `schema/v0/log.json` | A whole log as one file: meals, the person's own foods and deleted meals |
| `schema/v0/nutrients.json` | The nutrient keys, each ending in its unit, mapped to FAO INFOODS tagnames, USDA FoodData Central numbers, Health Connect and HealthKit |
| `schema/v0/amount-vectors.json` | Worked cases every implementation must agree on: an amount on a food, as grams, then as nutrients |
| `schema/CHANGELOG.md` | What changed in v0 |
| `examples` | A food, a breakfast and a log, built from USDA FoodData Central (CC0) |
| `openfoodlog.go` | Package `openfoodlog`: the embedded schemas, a validator, and the reference calculation (`Grams`, `Nutrients`) |
| `cmd/ofl` | `ofl validate`, which checks documents against the schemas, and `ofl site`, which builds the website |

```bash
go run ./cmd/ofl validate
go test ./...
go run ./cmd/ofl site dist
```

Until v1 these files are published here from [Lifterclub](https://lifterclub.com), the reference implementation. Issues and proposals belong here.

Licence: Apache-2.0. The example foods' numbers come from USDA FoodData Central, which is in the public domain (CC0 1.0).
