# Open Food Log

A draft open standard for a personal food log, the nutrition sister of the Open Workout Schema
(`docs/ows`) and the Open Exercise Schema. Why it exists and what else is out there:
[`docs/research/nutrition.md`](../research/nutrition.md).

**Status: v0.2 draft, 2026-11-13** (v0.1 2026-11-12, v0 2026-11-09). Lifterclub implements v0 and is the first
implementation (`docs/nutrition.md`). Published to
[`openexercise/open-food-log`](https://github.com/openexercise/open-food-log) by
`scripts/publish-openfoodlog.sh` and served at [openfoodlog.org](https://openfoodlog.org); this
directory stays the source of truth until v1.

| | |
| --- | --- |
| [`v0/SPEC.md`](v0/SPEC.md) | The specification |
| [`v0/food.json`](v0/food.json), [`v0/meal.json`](v0/meal.json), [`v0/recipe.json`](v0/recipe.json), [`v0/saved-meal.json`](v0/saved-meal.json) (v0.2), [`v0/targets.json`](v0/targets.json), [`v0/log.json`](v0/log.json) | JSON Schemas (draft 2020-12) |
| [`v0/nutrients.json`](v0/nutrients.json) | 52 nutrient keys, mapped to INFOODS, FoodData Central, Health Connect and HealthKit |
| [`v0/amount-vectors.json`](v0/amount-vectors.json), [`v0/recipe-vectors.json`](v0/recipe-vectors.json), [`v0/target-vectors.json`](v0/target-vectors.json) | Cases every implementation must agree on: amounts to grams to nutrients, recipes, and targets met or not |
| [`examples`](../examples) | Foods (generic, packaged with a label, a supplement), meals, a recipe, a saved meal, targets and a log |
| [`CHANGELOG.md`](CHANGELOG.md) | What changed |

`go test ./internal/ofl` checks that the schemas compile, every example validates and adds up, every
nutrient key used anywhere is in the vocabulary, the vocabulary matches the schema, the schemas
refuse what they should, and the vectors agree with a reference calculation.

The generic examples and the vectors' `ofc:` foods were generated from FoodData Central records
171287 (egg), 173904 (oats), 173944 (banana) and 171265 (whole milk), fetched 2026-11-09. The milk's
density is its cup portion (244 g) over a US cup (236.59 ml). Foods in the `example:` namespace (a
can of cola, a granola bar, a vitamin D softgel) are illustrative labels, not real products. The
nutrient mappings were checked on 2026-11-12 against FoodData Central's `nutrient.csv`, the FAO
INFOODS tagname lists (with their 2008 and 2010 additions and the Guidelines for Food Matching
v1.2), and the Health Connect and HealthKit references.

## Versions

Documents name the version they were written to in `ofl_version` (`"0.2"`); none means 0.0. A
minor version only adds: optional members, nutrient keys, enum values, documents. A document valid
under one 0.x stays valid under the next, and a reader keeps what it doesn't understand rather than
dropping it. Breaking changes before v1 are rare, fix mistakes, and are marked in the changelog.
The rules are section 13 of the specification.

## Extensions

Producers extend a food, a meal, an item, a recipe or a target with members named `x-<vendor>-…`,
and the nutrient vocabulary with keys named `x-<vendor>-<name>_<unit>`. Consumers ignore what they
don't understand. To register a vendor prefix, open a pull request to `open-food-log` adding a row
here; an extension more than one producer needs is a candidate for the core.

| Prefix | Who | Contact |
| --- | --- | --- |
| `x-lifterclub-` | Lifterclub, the reference implementation | `lifterclub.com` |

Names from before v0.1, without a vendor, and what replaces them:

| Name | Used by | Replaced by (v0.1) |
| --- | --- | --- |
| `x-nutrients_per_100ml` (food) | Lifterclub, drinks from Open Food Facts | `nutrients_basis: "100ml"` and `nutrients_per_100ml` |
| `x-serving_ml` (food) | Lifterclub | `label.serving.ml` |
