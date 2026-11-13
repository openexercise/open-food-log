# Open Food Log developer guide

How to read and write Open Food Log (OFL) documents, use the Open Food Catalogue, and build or check a
server that keeps a food log. The [specification](../schema/v0/SPEC.md) is the authority; this guide
walks through it with the examples in this repository and the commands that check your work. Where
the two disagree, the specification wins, and please open an issue.

To change the format, the catalogue or the tools, read [CONTRIBUTING.md](../CONTRIBUTING.md).

**Status.** OFL is a v0 draft: version 0.2 today. It may change until v1, which freezes when the
first application outside Lifterclub writes it in production. Minor versions only add, so a document
valid under 0.1 stays valid under 0.2.

## Contents

1. [The documents](#1-the-documents)
2. [Getting the tools](#2-getting-the-tools)
3. [Foods](#3-foods)
4. [Nutrients](#4-nutrients)
5. [Meals and items](#5-meals-and-items)
6. [Amounts, grams and nutrients](#6-amounts-grams-and-nutrients)
7. [Estimates, confidence and share](#7-estimates-confidence-and-share)
8. [Times, time zones and the local day](#8-times-time-zones-and-the-local-day)
9. [Recipes](#9-recipes)
10. [Saved meals and logging again](#10-saved-meals-and-logging-again)
11. [Targets](#11-targets)
12. [The log](#12-the-log)
13. [IDs and the Open Food Catalogue](#13-ids-and-the-open-food-catalogue)
14. [Versions and extensions](#14-versions-and-extensions)
15. [Validating documents](#15-validating-documents)
16. [Checking your amount calculation](#16-checking-your-amount-calculation)
17. [Serving a food log: the API core, MCP and A2A](#17-serving-a-food-log-the-api-core-mcp-and-a2a)
18. [Checking a server with `ofl conformance`](#18-checking-a-server-with-ofl-conformance)
19. [The Go client](#19-the-go-client)
20. [Mapping to other systems](#20-mapping-to-other-systems)

## 1. The documents

Every document is UTF-8 JSON, valid against a JSON Schema (draft 2020-12) in [`schema/v0`](../schema/v0),
and names that schema in `$schema`.

| Document | Schema (`$schema`) | Required members | Example |
| --- | --- | --- | --- |
| Food | `https://openfoodlog.org/schema/v0/food.json` | `id`, `name`, `kind`, `nutrients_per_100g` | [egg](../examples/egg-whole-raw.json), [granola bar](../examples/granola-bar.json), [vitamin D](../examples/vitamin-d3-softgel.json) |
| Meal | `…/meal.json` | `source`, `items`, and `eaten_at` or `date` | [breakfast](../examples/breakfast.json), [dinner](../examples/dinner.json) |
| Recipe | `…/recipe.json` | `id`, `name`, `lines`, `servings` | [overnight oats](../examples/overnight-oats.json) |
| Saved meal (v0.2) | `…/saved-meal.json` | `id`, `name`, `items` | [usual breakfast](../examples/usual-breakfast.json) |
| Targets | `…/targets.json` | `targets` | [targets](../examples/targets.json) |
| Log | `…/log.json` | `exported_at`, `meals` | [log](../examples/log.json) |

Three more files hold data rather than documents: [`nutrients.json`](../schema/v0/nutrients.json)
(the nutrient vocabulary) and the vectors, worked cases every implementation must agree on:
[`amount-vectors.json`](../schema/v0/amount-vectors.json),
[`recipe-vectors.json`](../schema/v0/recipe-vectors.json) and
[`target-vectors.json`](../schema/v0/target-vectors.json).

Two rules run through all of it:

- **A value left out is unknown.** A nutrient that isn't known is absent, never `0`. An allergen not
  listed is unknown, not absent from the food.
- **A guess says so.** A value that was inferred or assumed is listed in `estimates` (on a meal,
  recipe or saved meal) or `provenance.generated` (on a food), as JSON pointers to it.

## 2. Getting the tools

The `ofl` command validates documents, checks servers and checks calculations. Install a release:

```bash
go install github.com/openexercise/open-food-log/cmd/ofl@v0.7.0
```

Or work from a checkout, where `go run ./cmd/ofl …` does the same:

```bash
git clone https://github.com/openexercise/open-food-log
cd open-food-log
go run ./cmd/ofl validate
```

```
9 document(s) valid
```

Its commands, from the usage `ofl` prints with no arguments (abridged):

```
ofl validate [path...]                                 validate documents (examples/ by default); each names its schema in $schema
ofl conformance [--json] [--token T] <base-url>        check a server's REST API
ofl conformance mcp [--json] [--token T] [--read-token R] <mcp-url>
ofl conformance a2a [--json] [--token T] [--read-token R] <agent-url>
ofl vectors [--json] -- <command> [args...]            check an amount calculation against the vectors
ofl site [outdir]                                      build the website (dist/ by default)
ofl catalogue <dir>                                    rebuild catalogue/ from the FoodData Central downloads in dir
```

In Go, the root package (`import openfoodlog "github.com/openexercise/open-food-log"`) carries the
schemas (`Schemas`), a validator (`NewValidator`, `Validate`, `ValidateAs`), the MCP and A2A profiles
(`Profile`, `A2AProfile`) and the reference amount calculation (`Measure`). The package
`github.com/openexercise/open-food-log/conformance` holds the conformance checks.

## 3. Foods

A food says what 100 g of it holds, how it's measured, and where the numbers came from. Here is the
egg from [`examples/egg-whole-raw.json`](../examples/egg-whole-raw.json), shortened:

```json
{
  "$schema": "https://openfoodlog.org/schema/v0/food.json",
  "id": "ofc:egg-whole-raw",
  "name": "Egg, Whole, Raw",
  "kind": "generic",
  "aliases": [{ "name": "egg", "kind": "common" }, { "name": "eggs", "kind": "common" }],
  "nutrients_per_100g": { "energy_kcal": 143, "protein_g": 12.56, "fat_g": 9.51, "carbohydrate_g": 0.72 },
  "portions": [
    { "id": "medium", "label": "1 medium", "grams": 44 },
    { "id": "large", "label": "1 large", "grams": 50, "is_default": true }
  ],
  "identifiers": { "fdc_id": 171287 },
  "provenance": {
    "sources": [{ "name": "USDA FoodData Central (SR Legacy)", "id": "171287",
                  "url": "https://fdc.nal.usda.gov/food-details/171287/nutrients",
                  "license": "CC0-1.0", "retrieved_at": "2026-11-09" }],
    "license": "CC0-1.0",
    "review": "unreviewed"
  }
}
```

| Member | What it holds |
| --- | --- |
| `id` | `namespace:slug` ([section 13](#13-ids-and-the-open-food-catalogue)). |
| `name` | The canonical name. A food is a distinct record when its nutrients differ: raw and cooked, dry and prepared are two foods. |
| `kind` | `generic` (a food as such), `branded` (a packaged product: give `brand` and `gtin`), `restaurant` (a dish as one restaurant serves it), `custom` (a person's own) or `supplement` (dosed per serving; give `identifiers.dsld_id` when the NIH label database has it). |
| `brand`, `gtin` | For branded foods. `gtin` is a list of barcodes, each as 14 digits, zero-padded on the left. |
| `aliases` | Other names, each with a `kind`. |
| `nutrients_basis` | Which set of values the source declared: `100g` (the default), `100ml` or `serving`. |
| `nutrients_per_100g` | What 100 g holds, in the state the name says. Required; `{}` when no value per 100 g is known (a supplement). |
| `nutrients_per_100ml` | What 100 ml holds, for a drink labelled that way. |
| `density_g_per_ml` | Grams per millilitre, so a volume can be weighed. Leave it out when it isn't known. |
| `portions` | Named measures with their weight: `id`, `label`, `grams`, and at most one `is_default`, the size to assume when a person names the food without one ("an egg"). Cups and spoons are portions, not units, because what a cup weighs depends on the food. |
| `label` | The package's nutrition label as printed: `region` (`us`, `eu`, `uk`, `au`, `ca`), `serving` (`text`, and `grams` or `ml` where printed), `servings_per_container`, `nutrients_per_serving` and `percent_daily_values`. Percentages are kept as printed, never turned into amounts. |
| `category`, `allergens`, `diet_flags`, `ingredients_text`, `glycemic_index`, `glycemic_load` | Attributes, each with a `basis`: `declared` (by a label, maker or measuring database) or `inferred` (worked out or guessed). Never show an inferred one as a declaration. |
| `identifiers` | IDs in other databases (`fdc_id`, `dsld_id`). |
| `status`, `replaced_by`, `previous_ids` | How an ID retires, as in the Open Exercise Schema. |
| `provenance` | `sources` (each with `name`, and `id`, `url`, `license`, `retrieved_at` where known), the record's own `license`, `generated` (pointers to values a model produced) and `review` (`unreviewed` or `reviewed`). |

The [granola bar](../examples/granola-bar.json) shows a packaged food: `nutrients_basis: "serving"`
because its label gives values per bar, a `label` with region `us`, a 40 g serving and percent daily
values, `nutrients_per_100g` worked out from the 40 g serving, and allergens with both bases (wheat
`declared`, gluten `inferred` from the wheat, and listed in `provenance.generated`). The
[vitamin D softgel](../examples/vitamin-d3-softgel.json) is a supplement: per serving only, with a
serving that has no weight, so `nutrients_per_100g` is `{}`.

## 4. Nutrients

[`nutrients.json`](../schema/v0/nutrients.json) lists the 52 keys a document may use. Each key ends in
its unit; there's no other way to give one.

```json
{ "key": "energy_kcal", "name": "Energy", "unit": "kcal", "infoods": "ENERC",
  "fdc_nutrient_number": "208", "health_connect": "energy", "healthkit": "dietaryEnergyConsumed",
  "notes": "Metabolizable energy. INFOODS reports ENERC in kJ by default; 1 kcal = 4.184 kJ." }
```

Every entry has `key`, `name` and `unit`, and maps to FAO INFOODS (`infoods`), USDA FoodData Central
(`fdc_nutrient_number`), Android Health Connect (`health_connect`) and Apple HealthKit (`healthkit`).
`null` means there is no exact match: `ala_g` has no Health Connect or HealthKit field, and
`carbohydrate_available_g` has no FoodData Central number. Some `notes` explain a mapping.

The groups: energy (`energy_kcal`, `energy_kj`); protein; fat and its parts (`saturated_fat_g`,
`monounsaturated_fat_g`, `polyunsaturated_fat_g`, `trans_fat_g`, `ala_g`, `epa_g`, `dha_g`);
`cholesterol_mg`; carbohydrate and its parts (`carbohydrate_g`, `carbohydrate_available_g`,
`fiber_g`, `sugars_g`, `added_sugars_g`, `starch_g`, `polyols_g`); minerals (`sodium_mg`, `salt_g`,
`potassium_mg`, `calcium_mg`, `iron_mg` and the rest); vitamins (`vitamin_a_rae_ug` to
`vitamin_b12_ug`, with `folate_dfe_ug` and `folic_acid_ug`); `choline_mg`; `caffeine_mg`;
`alcohol_g`; `water_g`.

Rules a producer follows (SPEC section 4):

- **Leave out what you don't know.** Writing `0` for an unknown value is wrong. A total over items
  that don't all carry a nutrient is a lower bound; say so when you show it.
- **Carbohydrate has two bases.** `carbohydrate_g` is total carbohydrate including fibre (US,
  Canadian labels, FoodData Central). `carbohydrate_available_g` leaves fibre out (EU, UK,
  Australian labels). Put a label's figure under the key its `label.region` means. You may give the
  other when fibre is known: `carbohydrate_g` = `carbohydrate_available_g` + `fiber_g`.
- **Net carbs** are computed when read (`carbohydrate_g` − `fiber_g`), never stored.
- **Salt** is `salt_g` = sodium × 2.5; `sodium_mg` = `salt_g` × 400.
- **IU is not a unit.** Convert on input: vitamin D 1 IU = 0.025 µg; vitamin A 1 IU of retinol =
  0.3 µg RAE; vitamin E 1 IU natural = 0.67 mg, synthetic = 0.45 mg. When the form isn't known, the
  converted value is an estimate and, on a meal item, goes in `estimates`.
- **Your own nutrients** use `x-<vendor>-<name>_<unit>` keys (`x-acme-beta_glucan_g`) until the
  vocabulary adds one.

## 5. Meals and items

A meal is one eating occasion. This is part of
[`examples/breakfast.json`](../examples/breakfast.json), with each item's `nutrients` trimmed:

```json
{
  "$schema": "https://openfoodlog.org/schema/v0/meal.json",
  "ofl_version": "0.1",
  "id": "ml_01JCB2A7Q0S4M",
  "eaten_at": "2026-11-09T07:30:00-06:00",
  "timezone": "America/Chicago",
  "logged_at": "2026-11-09T07:41:12-06:00",
  "meal_type": "breakfast",
  "source": { "kind": "mcp", "client": "claude", "external_id": null },
  "items": [
    {
      "text": "3 eggs",
      "food": { "id": "ofc:egg-whole-raw", "name": "Egg, Whole, Raw", "match": "catalogue" },
      "amount": { "value": 3, "unit": "portion", "portion": "large" },
      "grams": 150.0,
      "nutrients": { "energy_kcal": 214.5, "protein_g": 18.84 }
    },
    {
      "text": "a splash of milk",
      "food": { "id": "ofc:milk-whole", "name": "Milk, Whole (3.25%)", "match": "catalogue" },
      "amount": { "value": 30, "unit": "ml" },
      "grams": 30.93,
      "nutrients": { "energy_kcal": 18.87 }
    },
    {
      "text": "coffee with oat milk",
      "food": { "id": null, "name": "coffee with oat milk", "match": "unmatched" }
    }
  ],
  "estimates": ["/items/0/amount/portion", "/items/1/amount"],
  "notes": "Before the long run"
}
```

(The full example has five items, so its pointers differ: these are renumbered for the three shown.)

Meal members:

| Member | What it holds |
| --- | --- |
| `id` | Assigned by the server that stores the meal. |
| `eaten_at` | When it was eaten, a date-time with an offset. |
| `date` | The person's local day, for a meal with no time. With `eaten_at`, it must be `eaten_at`'s day in the person's zone. One of `eaten_at` or `date` is required. |
| `timezone` | The person's IANA zone (`America/Chicago`), beside the offset. |
| `logged_at` | When it was recorded, which may be hours later. |
| `meal_type` | `breakfast`, `lunch`, `dinner` or `snack`. |
| `label` | The person's own name for the occasion (`post-workout`, `iftar`), beside or instead of `meal_type`. |
| `title`, `notes` | Free text. |
| `recorded_by` | `person` (the default), `professional` (a dietitian or coach on their behalf) or `app`. |
| `media` | References to photos the producer keeps: `url`, `media_type`, `taken_at`. OFL never carries the media itself. |
| `source` | `kind` (`app`, `mcp`, `api`, `import`), `client` (the app, such as `claude`) and `external_id` (the meal's ID in the app it came from). The same shape as an Open Workout workout's. |
| `updated_at` | When the stored meal last changed, for mirroring. |
| `items` | What was eaten. |
| `estimates` | JSON pointers to values that were inferred or assumed ([section 7](#7-estimates-confidence-and-share)). |

Item members (only `text` is required):

| Member | What it holds |
| --- | --- |
| `id` | The server's ID for the item. |
| `text` | The person's words. Always kept, beside the match. |
| `food` | The match: `id` (null when nothing matched), `name` and `match`. |
| `amount` | The measure the person used: `value`, `unit`, and `portion` when the unit is `portion`. |
| `grams` | The weight eaten, when it can be known. |
| `share` | For a shared dish, the person's part, above 0 and at most 1. |
| `nutrients` | What was counted when the item was logged. |
| `confidence` | 0 to 1, how sure the producer is of the item. |
| `notes` | Free text. |

`food.match` says how far to trust the item:

| `match` | Meaning | Count it? |
| --- | --- | --- |
| `catalogue` | Confirmed against a catalogue or database food. | Yes |
| `custom` | Confirmed: the person's own food or recipe. | Yes |
| `close` | A likely food the person hasn't confirmed. | No, ask first |
| `unmatched` | No food found. | No, ask first |
| `none` | Nothing to match: the person gave the numbers ("lunch was about 650 calories"). Carries `nutrients` directly. | Yes |

An item's `nutrients` don't change when the food's record later does. A reader that wants today's
figures recomputes from `food.id` and `grams`.

Water has no separate log. Plain water is an item like any other, and hydration is `water_g` summed
over everything eaten and drunk.

## 6. Amounts, grams and nutrients

This is the part every implementation must get the same, so it has vectors. An amount's `unit` is one
of `g`, `kg`, `oz`, `lb`, `ml`, `l`, `fl_oz` (US), `portion` (with `portion`, a portion's `id`) or
`serving` (the food's `label.serving`, or a recipe's serving). Multiply the value by `share` first,
if there is one. Then:

| Unit | Grams | Millilitres |
| --- | --- | --- |
| `g`, `kg`, `oz`, `lb` | value × 1, 1000, 28.349523125, 453.59237 | — |
| `ml`, `l`, `fl_oz` | millilitres × `density_g_per_ml`, if the food has one | value × 1, 1000, 29.5735295625 |
| `portion` | value × that portion's `grams` | — |
| `serving` | value × `label.serving.grams`, or millilitres × density | value × `label.serving.ml` |

Each nutrient comes from the first of these the amount reaches:

1. with grams, `nutrients_per_100g[key] × grams / 100`;
2. else with millilitres, `nutrients_per_100ml[key] × ml / 100`;
3. else with servings, `label.nutrients_per_serving[key] × servings`.

When it can't be done, report one of these codes rather than guess:

| Code | When |
| --- | --- |
| `density_unknown` | A volume of a food with no density, or a weight of a food known only per 100 ml. |
| `portion_unknown` | A portion the food doesn't have. |
| `serving_unknown` | A serving of a food that declares none. |
| `serving_weight_unknown` | A weight of a food known only per serving, whose serving has no weight. |

You may assume a density (1 g/ml for a drink) only if you list the item's `grams` in `estimates`.
`grams` left out because it can't be known (a drink known per 100 ml, a supplement) isn't an error.

### Worked cases, from `amount-vectors.json`

Whole milk (`ofc:milk-whole` in the vectors): `density_g_per_ml` 1.031, `energy_kcal` 61 per 100 g.

- **250 ml.** Grams = 250 × 1.031 = 257.75. Energy = 61 × 257.75 / 100 = **157.2275 kcal**.
- **8 fl oz.** Millilitres = 8 × 29.5735295625 = 236.5882365. Grams = 236.5882365 × 1.031 =
  243.9224718315. Energy = 61 × 2.439224718315 = **148.792707817215 kcal**.

Oats (`ofc:oats-dry` in the vectors): a `cup` portion of 81 g, 379 kcal per 100 g.

- **Half a cup.** Grams = 0.5 × 81 = 40.5. Energy = 379 × 0.405 = **153.495 kcal**.
- **A tablespoon.** The food has no `tbsp` portion: **`portion_unknown`**.
- **One serving.** Oats declare no label serving: **`serving_unknown`**.

A can of cola (`example:cola-can`): known per 100 ml only (`energy_kcal` 42), a declared serving of
330 ml, no density.

- **250 ml.** No density, so grams stay unknown (`null`). Energy = 42 × 250 / 100 = **105 kcal**.
- **One serving.** Millilitres = 330. Energy = 42 × 3.3 = **138.6 kcal**; grams `null`.
- **100 g.** Nothing says what 100 g is in ml: **`density_unknown`**.

A vitamin D softgel (`example:vitamin-d3-softgel`): 25 µg per serving, a serving with no weight.

- **Two servings.** **50 µg** of `vitamin_d_ug`; grams `null`.
- **1 g.** **`serving_weight_unknown`**.

Rounding: implementations may round what they store; the vectors are exact, and an answer within 0.01
agrees. In Go, `openfoodlog.Measure(food, amount, share)` is the reference calculation; it returns
`Measured{Grams, ML, Nutrients}` or one of `ErrDensityUnknown`, `ErrPortionUnknown`,
`ErrServingUnknown`, `ErrServingWeightUnknown` and `ErrUnitUnknown`, whose text is the code.
[Section 16](#16-checking-your-amount-calculation) shows how to check your own.

## 7. Estimates, confidence and share

**`estimates`** is a list of JSON pointers (RFC 6901) into the document, to values that were inferred
or assumed rather than given: the size picked for "a banana", an amount read from a photo, numbers a
model guessed. In the breakfast, the person said "3 eggs" without a size, so `large` was assumed
(`/items/0/amount/portion`), and "a splash of milk" became 30 ml (`/items/3/amount`). A value from a
label and one from a photo must not look the same.

Rules for producers:

- Never invent a number without listing it. If you assume a portion, use the food's default portion
  and list the amount.
- A pointer names the most specific value guessed: the portion (`/items/0/amount/portion`) when only
  the size was assumed, the whole amount (`/items/3/amount`) when the quantity was too.
- Pointers name positions, so when you reorder or remove items, rewrite them.

**`confidence`** (0 to 1) on an item says how sure the producer is of it, as a photo model reports it.
It adds to `estimates` and never replaces them, and two producers' confidences can't be compared.

**`share`** is for a shared dish. In [`examples/dinner.json`](../examples/dinner.json), "split a
6-egg scramble with Sam":

```json
{
  "text": "split a 6-egg scramble with Sam",
  "food": { "id": "ofc:egg-whole-raw", "name": "Egg, Whole, Raw", "match": "catalogue" },
  "amount": { "value": 6, "unit": "portion", "portion": "large" },
  "share": 0.5,
  "grams": 150.0,
  "nutrients": { "energy_kcal": 214.5 },
  "confidence": 0.7
}
```

`amount` is the whole dish; `grams` and `nutrients` are already the person's part
(6 × 50 g × 0.5 = 150 g). Don't multiply by `share` again when you read it.

## 8. Times, time zones and the local day

A meal has `eaten_at` with an offset, or `date` (the person's local day) when no time was given, or
both. `timezone` is the person's IANA zone, beside the offset, so a reader can put the meal on the
right day even after the person travels or the clocks change.

The day a meal belongs to is the person's local day, not the UTC day. This snack at 23:30 in Chicago
is 05:30 UTC on the 14th, but it belongs to the 13th:

```json
{
  "$schema": "https://openfoodlog.org/schema/v0/meal.json",
  "ofl_version": "0.2",
  "eaten_at": "2026-11-13T23:30:00-06:00",
  "date": "2026-11-13",
  "timezone": "America/Chicago",
  "meal_type": "snack",
  "source": { "kind": "api", "client": "my-app", "external_id": "snack-42" },
  "items": [
    {
      "text": "a banana",
      "food": { "id": "ofc:bananas-raw", "name": "Bananas, raw", "match": "catalogue" },
      "amount": { "value": 1, "unit": "portion", "portion": "1-medium-7-to-7-7-8-long" },
      "grams": 118,
      "nutrients": { "energy_kcal": 105.02, "protein_g": 1.29 }
    },
    {
      "text": "some peanut butter",
      "food": { "id": null, "name": "some peanut butter", "match": "unmatched" }
    }
  ],
  "estimates": ["/items/0/amount"],
  "x-myapp-mood": "hungry"
}
```

It uses a real catalogue food and portion (`ofc:bananas-raw` has no default portion, so the medium
one was picked and listed in `estimates`), an unmatched item waiting for the person, and an
extension member ([section 14](#14-versions-and-extensions)). The item's `nutrients` are trimmed to
two keys for space; a real producer writes every nutrient the food has. It passes
`ofl validate`.

`logged_at` is when it was recorded. A lunch logged that evening has a `logged_at` hours after its
`eaten_at`; daily totals use `eaten_at` (or `date`), never `logged_at`.

## 9. Recipes

A recipe ([`recipe.json`](../schema/v0/recipe.json)) is a food made of other foods. Its `lines` are
shaped like meal items, raw as they went in, and it says how many `servings` it makes. A meal item
names it by `id` like any food, with `match: "custom"`.
[`examples/overnight-oats.json`](../examples/overnight-oats.json):

| Line | Grams | `energy_kcal` |
| --- | --- | --- |
| 1 cup oats (`ofc:oats-dry`, 379 per 100 g) | 81 | 306.99 |
| 1 cup whole milk (`ofc:milk-whole`, 61 per 100 g) | 244 | 148.84 |
| a banana (`ofc:banana-raw`, 89 per 100 g) | 118 | 105.02 |
| **Sum** | **443** | **560.85** |

with `servings: 2` and no `yield_g`.

- **Nutrients** are the lines' nutrients summed. A nutrient one line lacks is left out of the recipe
  altogether (oats and milk have no `trans_fat_g`, so the recipe has none), since the sum would be a
  lower bound.
- **Yield** (what the whole weighs when made) is `yield_g`, else the lines' grams × `cooking_yield_factor`
  (made weight over raw weight), else the lines' grams: here 443 g.
- **Per 100 g** = sum × 100 / yield = 560.85 × 100 / 443 = **126.6027 kcal**.
- **A serving** weighs yield / servings = 221.5 g and holds 560.85 / 2 = **280.425 kcal**. That's the
  "bowl of overnight oats" item in the dinner example: 221.5 g, 280.43 kcal.

A recipe item's amount is weighed (`g`, `kg`, `oz`, `lb`) or counted in `serving`s. A producer may
state `nutrients_per_100g` and `nutrients_per_serving`; a reader may recompute them. The
[recipe vectors](../schema/v0/recipe-vectors.json) hold these cases and the ones with a stated yield
and a cooking yield factor.

## 10. Saved meals and logging again

New in v0.2. A saved meal ([`saved-meal.json`](../schema/v0/saved-meal.json)) is a meal a person keeps
to log again: a `name`, `items` shaped like meal items, and optional `meal_type`, `label`, `notes`,
`source`, `updated_at` and `estimates`. See
[`examples/usual-breakfast.json`](../examples/usual-breakfast.json) (ID `saved:usual-breakfast`).

The difference from a recipe is what lands in the log: a recipe logs as one item, a saved meal as its
items, each counted on its own.

- **Logging a saved meal makes a new meal.** Each item keeps its `text`, food and amount; its grams
  and nutrients are counted again from the food as it is now ([section 6](#6-amounts-grams-and-nutrients)).
  An item naming no food (`match: none`) keeps its nutrients as given.
- **An amount listed in the saved meal's `estimates`** is logged with its amount, grams and nutrients
  marked estimated in the new meal.
- **`grams` and `nutrients` on a saved meal's items** are a preview of what they counted when it was
  saved, never a log.
- **A sentence naming a saved meal** ("my usual breakfast") may log its items instead of being matched
  as one food.
- **Logging a meal again** ("the same as yesterday's lunch") works the same way, from a logged meal.

## 11. Targets

A targets document ([`targets.json`](../schema/v0/targets.json)) holds the targets a person keeps.
They are the person's own: set by them (`person`), by a `professional` they work with, or an
`app_suggestion` they accepted. OFL never prescribes one.

Each entry has `nutrient`, `per` (`day`, the person's local day, or `meal`, optionally only a
`meal_type`), at least one of `min`, `max` and `target`, `effective_from`, and optionally
`effective_to`, `weekdays` (ISO: 1 is Monday, 7 Sunday), `tolerance_percent`, `id` and `note`.
From [`examples/targets.json`](../examples/targets.json):

```json
{ "nutrient": "energy_kcal", "per": "day", "target": 2600, "effective_from": "2026-11-01", "set_by": "professional", "id": "tg_03" },
{ "nutrient": "energy_kcal", "per": "day", "target": 3000, "weekdays": [1, 3, 5],
  "effective_from": "2026-11-01", "set_by": "professional", "note": "Training days.", "id": "tg_04" },
{ "nutrient": "sodium_mg", "per": "day", "max": 2300, "effective_from": "2026-11-01", "set_by": "app_suggestion", "id": "tg_05" }
```

**Which applies.** Of the entries for the nutrient, `per` and meal type that cover the day, take the
most specific (one with `weekdays` over one without, then one with a `meal_type` over one without),
then the latest `effective_from`, then the later in the list. So on Monday 2026-11-09, `tg_04`
(training days) applies; on Tuesday 2026-11-10, `tg_03`. A newer every-day target doesn't override a
weekday one: end the weekday one with `effective_to` to retire it.

**Status** is one of `met`, `under`, `above` or `unknown`:

- With `min` or `max`, those are the bounds, and `target` is only the point to aim for.
- With `target` alone, the bounds are `target` ± `tolerance_percent` (10 by default). `tg_04` on a
  Monday is 2700 to 3300; a day of 2900 kcal is `met`.
- Above the upper bound is `above`; below the lower, `under`; otherwise `met`.
- **An incomplete total is a lower bound** (an item lacks the nutrient, or the day may not be fully
  logged). It can show a minimum met or a maximum passed, never the reverse: 1800 mg of sodium on a
  day not yet over is `unknown`, not `met`, because what's missing could take it past 2300.

Use these words. There is no "over" or "over budget": a target is met, under or above. An app with a
calm mode hides the status as it hides the numbers. The
[target vectors](../schema/v0/target-vectors.json) hold 21 cases, each naming the entry that applies
(its index in their `targets` list, or `null`) and the status.

## 12. The log

A log ([`log.json`](../schema/v0/log.json)) is a whole food log as one file: what an export holds and
an import reads.

| Member | What it holds |
| --- | --- |
| `exported_at` | When it was written. Required. |
| `meals` | Every meal, newest first. Required. |
| `foods` | At least the `usr:` foods the meals, recipes and saved meals name. Catalogue and `gtin:` foods may be included. |
| `recipes` | At least the recipes the meals and saved meals name, and the foods their lines name go in `foods`. |
| `saved_meals` | The person's saved meals (v0.2). |
| `targets` | A targets document, current and past entries. |
| `deleted_meals` | Meals the person deleted, newest first, each as `{ "deleted_at", "meal" }`. |

A server may add more of the person's record beside these (a profile, their Open Workout workouts);
readers ignore keys they don't know. See [`examples/log.json`](../examples/log.json).

## 13. IDs and the Open Food Catalogue

An ID is `namespace:slug`.

| Namespace | Means | Resolves |
| --- | --- | --- |
| `ofc:` | The Open Food Catalogue. Reserved. | In [`catalogue/foods.jsonl`](../catalogue/foods.jsonl) |
| `gtin:` | A packaged product by barcode, 14 digits zero-padded on the left (`gtin:00012345678905`). | In any product database, without a shared copy |
| `usr:` | One person's own food. | Only in the system that made it; an export includes the ones its meals name |
| `recipe:` | One person's recipe. | The same |
| `saved:` | One person's saved meal (v0.2). | The same |
| your own | `acme:protein-bar` | Wherever you publish it |

The examples also use `example:` for illustrative products that don't exist.

### Using the catalogue

The Open Food Catalogue holds 12,442 generic foods built from USDA FoodData Central (Foundation
Foods, FNDDS survey foods and SR Legacy), one OFL food per line. Branded products aren't in it: use
`gtin:`. Each record carries its FoodData Central ID in `identifiers.fdc_id`, its sources in
`provenance.sources`, and up to 52 nutrients, portions with FoodData Central's weights and wording,
a density for most liquids, a FoodData Central category (`basis: inferred`) and aliases.

```bash
grep '"id":"ofc:bananas-raw"' catalogue/foods.jsonl
```

The ID is the FoodData Central name in lower case with runs of other characters made `-`. Things to
know before you depend on it:

- **It's a v0 draft.** IDs may change until v1. From v1, a published ID never changes meaning and is
  never reused; a consumer holding an ID it can't find looks in other records' `previous_ids`.
- **No record has been reviewed by a person** (`provenance.review` is `unreviewed`). The numbers are
  USDA's; the IDs, portions, densities and aliases are the generator's.
- **Only survey foods mark a default portion.** 5,341 of the 5,410 FNDDS survey foods have one
  ("Quantity not specified"); the SR Legacy and Foundation Foods records have none. Without one, ask
  the person, or pick a portion and list the amount in `estimates`.
- **An alias may belong to more than one food** ("milk"). Ask which, don't guess.
- **The examples' `ofc:` IDs are not all catalogue IDs.** The examples and vectors were written
  before the catalogue was built: `ofc:egg-whole-raw` and `ofc:milk-whole` exist in it, but
  `ofc:oats-dry` and `ofc:banana-raw` don't (the catalogue's banana is `ofc:bananas-raw`), and the
  catalogue's milk has different portion IDs and a density of 1.0313. Look IDs up in
  `foods.jsonl`, not the examples.

**Licence.** The catalogue is dedicated to the public domain under
[CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/), like FoodData Central. Use it for
anything without asking. A credit to "Open Food Catalogue, from USDA FoodData Central" is welcome,
not required. Don't merge records from a database whose licence requires derived databases to share
its terms (Open Food Facts is ODbL) into a CC0 copy; a `gtin:` reference avoids copying. How each
value is made is in [catalogue/README.md](../catalogue/README.md), and what was kept and left out in
[catalogue/REPORT.md](../catalogue/REPORT.md).

## 14. Versions and extensions

**`ofl_version`** names the version a document was written to, as `MAJOR.MINOR` (`"0.2"`). A
document without one is `0.0`, the first draft. Schema `$id`s stay `…/schema/v0/` for every 0.x.

- **A minor version only adds:** optional members, nutrient keys, enum values, documents. It never
  removes or renames a member, changes a unit or meaning, or makes an optional member required.
- **A reader meeting something newer keeps the document.** Ignore members you don't know. Treat an
  enum value you don't know as unknown: an amount unit you can't use means the item can't be weighed,
  not that it's invalid. Never drop an item you can't fully read.
- Changes are listed in [`schema/CHANGELOG.md`](../schema/CHANGELOG.md).

**Extensions.** A food (and its `label` and `identifiers`), a meal, an item, a recipe, a saved meal,
a targets document and a target accept members starting `x-`; every object rejects other members it
doesn't define. The small objects inside them (`source`, an item's `food`, `amount`, a portion) take
no extra members at all. Name yours `x-<vendor>-<name>`
(`x-myapp-mood` above) and nutrient keys `x-<vendor>-<name>_<unit>`, whose values must be
non-negative numbers. Readers ignore `x-` members they don't understand. Register your vendor
prefix in the Extensions table of [`schema/README.md`](../schema/README.md#extensions) by pull
request; [CONTRIBUTING.md](../CONTRIBUTING.md#registering-an-extension-prefix) says how. Two names
from before v0.1 without a vendor, `x-nutrients_per_100ml` and `x-serving_ml`, are replaced by
`nutrients_per_100ml` and `label.serving.ml`.

## 15. Validating documents

`ofl validate` validates each `.json` file under the paths you give (`examples/` by default) against
the schema its `$schema` names. In a checkout it reads `schema/v0/` from disk; installed elsewhere, it
uses the schemas built into it.

```bash
ofl validate snack.json
```

```
1 document(s) valid
```

A document with mistakes lists every one, and the command exits 1:

```json
{
  "$schema": "https://openfoodlog.org/schema/v0/meal.json",
  "source": { "kind": "app" },
  "items": [
    {
      "text": "a banana",
      "amount": { "value": 1, "unit": "cup" },
      "nutrients": { "energy": 105 }
    }
  ]
}
```

```
bad.json: jsonschema validation failed with 'https://openfoodlog.org/schema/v0/meal.json#'
- at '/items/0': validation failed
  - at '/items/0/amount/unit': value must be one of 'g', 'kg', 'oz', 'lb', 'ml', 'l', 'fl_oz', 'portion', 'serving'
  - at '/items/0/nutrients': additional properties 'energy' not allowed
- at '': 'anyOf' failed
  - at '': missing property 'eaten_at'
  - at '': missing property 'date'
1 problem(s) in 1 document(s)
```

A cup is a portion, not a unit; `energy` has no unit (use `energy_kcal`); and a meal needs a time or a
day. A member without the `x-` prefix fails too (`additional properties 'mood' not allowed`).

In Go:

```go
v, err := openfoodlog.NewValidator() // the embedded schemas
if err != nil {
	return err
}
for _, p := range v.Validate(data) { // by the document's $schema
	fmt.Println(p)
}
problems := v.ValidateAs("meal", data) // or by kind, whatever $schema says
```

`ValidateAs` takes `food`, `meal`, `recipe`, `saved-meal`, `targets` or `log`. Both return a list of
problems, empty when the document is valid.

The schemas check shape. They don't check that an item's grams and nutrients follow from its food
and amount, that pointers in `estimates` point at something, or that `date` is `eaten_at`'s local
day. Check those with your own tests, using the vectors.

## 16. Checking your amount calculation

`ofl vectors` runs every amount vector through your implementation. It starts your command once per
case, writes the case to its standard input, and reads the result from its standard output:

```json
{"name": "a share of a dish", "food": { …the whole food document… },
 "amount": {"value": 6, "unit": "portion", "portion": "large"}, "share": 0.5}
```

`share` is present only when the case has one. Print the grams and millilitres where known (`null`
or absent where not) and the nutrients, or the error code:

```json
{"grams": 150, "ml": null, "nutrients": {"energy_kcal": 214.5, "protein_g": 18.84}}
{"error": "portion_unknown"}
```

The runner can be in any language. Here is one in Go, wrapping the reference calculation:

```go
// A runner for ofl vectors: one case on stdin, the result on stdout.
package main

import (
	"encoding/json"
	"os"

	openfoodlog "github.com/openexercise/open-food-log"
)

func main() {
	var in struct {
		Name   string             `json:"name"`
		Food   openfoodlog.Food   `json:"food"`
		Amount openfoodlog.Amount `json:"amount"`
		Share  float64            `json:"share"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&in); err != nil {
		os.Exit(1)
	}
	out := map[string]any{}
	m, err := openfoodlog.Measure(in.Food, in.Amount, in.Share)
	if err != nil {
		out["error"] = err.Error() // density_unknown, portion_unknown, ...
	} else {
		out["grams"], out["ml"], out["nutrients"] = m.Grams, m.ML, m.Nutrients
	}
	json.NewEncoder(os.Stdout).Encode(out)
}
```

Build it and run the vectors through it:

```bash
go build -o runner .
ofl vectors -- ./runner
```

```
PASS grams as given
PASS a named portion
…
PASS a share of a dish
PASS a share of a drink
19 cases: 19 passed, 0 failed, 0 skipped
```

It exits non-zero when a case disagrees. Answers within 0.01 of the vector agree. `--json` prints the
report as JSON. A Go server can skip the process and call `conformance.CheckVectors(ctx, compute)`
from its tests, where `compute` receives the same input as `json.RawMessage` and returns a
`conformance.VectorResult`.

The recipe and target vectors have no runner yet; read them in your own tests, as `calc_test.go`
does here.

## 17. Serving a food log: the API core, MCP and A2A

A server that keeps people's food logs offers them to other apps in up to three ways. Each is
published here and generated from the reference implementation,
[Lifterclub](https://lifterclub.com/developers).

### The REST API core

[`api/core.yaml`](../api/core.yaml) (OpenAPI 3.1) is the routes every OFL server serves:

| Routes | What |
| --- | --- |
| `GET`, `POST /v1/foods`; `GET /v1/foods/{id}` | Search foods; create a person's own; read one as an OFL food. |
| `GET`, `POST /v1/meals`; `GET`, `PATCH`, `DELETE /v1/meals/{id}`; `POST /v1/meals/{id}/restore` | Meals by day range, log, read, change, delete, restore. |
| `GET /v1/meals?updated_since=…&include_deleted=true` | The change feed: every change since a time, oldest first, with a keyset `cursor`, deleted meals as tombstones. |
| `POST /v1/meals/{id}/items`; `PATCH`, `DELETE /v1/meal-items/{id}` | Add, change (including confirming an item's food) and remove items. |
| `POST /v1/meals/{id}/copy` | Log a meal again (v0.2). |
| `/v1/recipes`, `/v1/recipes/{id}` | Recipes (v0.2). |
| `/v1/saved-meals`, `/v1/saved-meals/{id}`, `POST /v1/saved-meals/{id}/log` | Saved meals, and logging one (v0.2). |
| `GET`, `PUT /v1/targets`; `GET /v1/nutrition/summary` | Targets, and daily totals with each target's status. |
| `GET /v1/export` | The whole log as an OFL log document. |
| `GET`, `POST /v1/webhooks`; `DELETE /v1/webhooks/{id}` | Subscriptions to `meal.created`, `meal.updated` and `meal.deleted`. |

What every server keeps to:

- **Discovery and auth.** A request without a token gets a 401 whose `WWW-Authenticate` names the
  OAuth protected resource metadata (RFC 9728), which names the authorization server (RFC 8414).
  Clients use OAuth 2.1 with PKCE and the scopes `meals:read` and `meals:write`.
- **Idempotency.** A create takes an `Idempotency-Key` header; a retry with the same key returns the
  first result with `Idempotent-Replayed: true`.
- **External IDs.** A meal's `source.external_id` replays for the app that sent it, and one the
  person deleted stays deleted (`not_found`), so a sync never brings it back.
- **Errors** are RFC 9457 problem details whose `code` is one of `invalid_input`, `consent_required`,
  `not_found`, `undo_unavailable`, `rate_limited`, `idempotency_conflict`, `unauthenticated` and
  `internal_error`. A client treats a code it doesn't know as the closest of these.
- **Webhooks** are signed in the `OFL-Signature` header: `t=<unix seconds>,v1=<hex HMAC-SHA256>`, the
  HMAC keyed with the subscription's secret over `<t>.<body>`. A receiver checks it with
  `conformance.VerifySignature(secret, header, body, time.Now(), 5*time.Minute)`, which also refuses
  a delivery signed too long ago.
- **Calm mode.** A server may leave totals out when the person asked not to see numbers; meals and
  items stay.

A server may add routes, fields and limits of its own; clients ignore what they don't know.

### The MCP tool profile

[`mcp/tools.json`](../mcp/tools.json) is the set of MCP tools an assistant uses over a food log, with
their input schemas, annotations, instructions and error codes; [`mcp/README.md`](../mcp/README.md)
explains it. Each tool's `_meta` names its profile under `dev.openfoodlog/profile`: `core` tools
(no LLM needed: `log_meal`, `get_meals`, `update_meal`, `delete_meal`, `search_foods`, `undo`, and
since v0.2 the targets, summary, recipe and saved-meal tools) are what a compatible server offers;
`lifterclub` tools are the reference server's extensions. A compatible server sends the profile's
`instructions` and keeps its rules: no invented numbers, honest matches with `needs_confirmation`, an
`idempotency_key` on every adding write, calm mode, and stored text treated as data.

### The A2A agent profile

[`a2a/README.md`](../a2a/README.md) and [`a2a/agent-card.json`](../a2a/agent-card.json) describe a
nutrition agent over A2A 1.0 (JSON-RPC), for another agent handing it food-log work as tasks. The
card names the extension `https://openfoodlog.org/a2a/v0` with its core skills: `log-meal` (an OFL
meal as a data part), `food-lookup`, `get-meals`, `nutrition-summary` and `targets`. Unsure foods put
the task in `input-required` until the caller confirms them; a message's `messageId` is its
idempotency key.

## 18. Checking a server with `ofl conformance`

`ofl conformance` runs a fixed script against your server as one person and reports each check. Use
a test account: the scripts log meals. The REST and MCP scripts delete what they made, also when a
check fails part way; A2A has no delete, so its meal stays. Pass tokens in the environment to keep
them out of your shell history.

```bash
OFL_TOKEN=… ofl conformance https://api.example.com
OFL_TOKEN=… OFL_READ_TOKEN=… ofl conformance mcp https://mcp.example.com
OFL_TOKEN=… OFL_READ_TOKEN=… ofl conformance a2a https://agent.example.com
```

| Script | Token | Checks, in order |
| --- | --- | --- |
| REST, against the API's origin (routes under `/v1`) | `OFL_TOKEN` (or `--token`): `meals:read` and `meals:write` | `discovery`, `create`, `read`, `replay`, `external-id`, `update`, `copy`, `saved-meal`, `recipe`, `change-feed`, `export`, `delete`, `tombstone`, `stays-deleted`, `webhooks` |
| MCP, against the Streamable HTTP endpoint | `OFL_TOKEN` able to write, and `OFL_READ_TOKEN` (or `--read-token`), read-only, for the same person | `discovery`, `tools`, `log`, `replay`, `needs-confirmation`, `read`, `update`, `again`, `errors`, `read-only`, `delete`, `undo` |
| A2A, against the URL serving `/.well-known/agent-card.json` | `OFL_TOKEN` with both scopes, and `OFL_READ_TOKEN` with `meals:read` | `card`, `discovery`, `log`, `replay`, `needs-confirmation`, `answer`, `cancel`, `get-task`, `read`, `lookup`, `summary`, `targets`, `errors`, `read-only` |

Without a read-only token, the MCP and A2A scripts skip `read-only`, and a server can't pass. Each
command also takes `--json` (the report as JSON) and `--timeout` (2 minutes by default). Each check
prints `PASS`, `FAIL` with what was wrong, or `SKIP` when it needs one that failed. Pointed at nothing,
the REST script reports:

```
FAIL discovery: GET /v1/meals: Get "http://127.0.0.1:9/v1/meals": dial tcp 127.0.0.1:9: connect: connection refused
FAIL create: POST /v1/meals: Post "http://127.0.0.1:9/v1/meals": dial tcp 127.0.0.1:9: connect: connection refused
SKIP read: needs a meal, which create didn't store
…
15 checks: 0 passed, 4 failed, 11 skipped
not conformant
```

The command exits non-zero unless every check passes. A server that passes the REST and MCP scripts
may say it is conformant; "conformant" is self-reported.

### In your own tests

A Go server can run the same scripts in its tests, as Lifterclub does, with `conformance.Run`,
`RunMCP` and `RunA2A`:

```go
func TestOpenFoodLogConformance(t *testing.T) {
	srv := httptest.NewServer(newAPI()) // your server's handler
	defer srv.Close()
	report, err := conformance.Run(context.Background(), conformance.Config{
		BaseURL: srv.URL,
		Token:   testToken, // a test person's token with meals:read and meals:write
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range report.Results {
		if r.Status != conformance.Pass {
			t.Errorf("%s %s: %s", r.Status, r.Name, r.Detail)
		}
	}
}
```

`MCPConfig` takes `URL`, `Token` and `ReadOnlyToken`; `A2AConfig` the same, with `URL` where the
card is served. Each has an optional `HTTPClient`. `report.Passed()` is true when every check passed.
Pin the module version in `go.mod`: a new release can add checks.

## 19. The Go client

[`clients/go`](../clients/go) is a client for the API core on the standard library alone (a module of
its own, package `openfoodlog`). It sends the token on every request and an `Idempotency-Key` on
every POST, turns problem details into errors, and pages the change feed.

```go
import openfoodlog "github.com/openexercise/open-food-log/clients/go"

c := openfoodlog.New(baseURL, token)

written, err := c.LogMeal(ctx, openfoodlog.MealInput{
	MealType:   "snack",
	ExternalID: "snack-42", // sending it again replays the stored meal
	Items: []openfoodlog.ItemInput{
		{Text: "a banana", FoodID: "ofc:bananas-raw"},
		{Text: "flat white"}, // no food: the server matches the words
	},
})
if err != nil {
	log.Fatal(err)
}
fmt.Println("logged", written.Meal.ID, "replayed:", written.Replayed)
for _, ask := range written.NeedsConfirmation {
	fmt.Printf("which food is %q? %d candidates\n", ask.Text, len(ask.Candidates))
}
```

An item without an `Amount` gets the food's default portion, listed in `estimates`. Confirm an item
with `c.UpdateItem(ctx, itemID, openfoodlog.ItemPatch{FoodID: &foodID})`. Mirror a log with
`c.Changes(ctx, openfoodlog.ChangesOptions{UpdatedSince: lastSeen, IncludeDeleted: true})`, which
yields each change oldest first. It also covers foods, recipes, saved meals and logging again,
webhooks and the export; [`clients/go/README.md`](../clients/go/README.md) has more, and `c.Do` reaches
routes it has no method for. Getting the token (OAuth 2.1 with PKCE) is the app's job; set
`c.TokenSource` to hand it a refreshed one.

The client has no release tag of its own yet, so `go get github.com/openexercise/open-food-log/clients/go@main`
gives a pseudo-version of the latest commit. A TypeScript client is published from Open Workout as
`@openexercise/open-workout-client/openfoodlog`.

## 20. Mapping to other systems

SPEC section 12 maps OFL to FHIR R5 `NutritionIntake`, Health Connect's `NutritionRecord` and
HealthKit. In short: a meal is one `NutritionIntake` (or one per item, to keep per-item nutrients), an
item is one Health Connect record, and each nutrient becomes a HealthKit sample, using the field names
in `nutrients.json`. Health Connect's `mealType` numbers breakfast 1, lunch 2, dinner 3, snack 4.

None of those systems can say which numbers were guessed, which matches are unconfirmed, or which app
wrote them. Exporting to them loses `estimates`, `match`, `confidence`, `share`, `recorded_by` and
`source`; keep the OFL document if you need them back. Water drunk goes to Health Connect's
`HydrationRecord` and HealthKit's `dietaryWater`, by volume; `water_g` in foods maps to neither.

Not in v0: body metrics, glucose readings and fasting windows (they describe the person, not what
they ate; use Health Connect, HealthKit or FHIR `Observation` and link by time), meal plans, and the
photos themselves.
