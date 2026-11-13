# Changelog

## Unreleased

## v0.2 draft, 2026-11-13

Saved meals, and the API core and MCP profile for recipes, saved meals and logging a meal again.
Only additions: every v0.1 document stays valid.

- **v0.2: saved meals.** `saved-meal.json` is a meal a person keeps to log again: a `name` and
  items shaped like meal items, with an optional `meal_type`, `label` and `notes`. Logging one makes
  a new meal whose items are counted again from their foods (section 8); an estimated amount stays
  estimated. A log holds them in `saved_meals`. Logging a meal again works the same way.
- The API core has recipes (`/v1/recipes`), saved meals (`/v1/saved-meals`, `.../log`) and
  `POST /v1/meals/{id}/copy`; the MCP profile's core adds `save_recipe`, `get_recipes`,
  `delete_recipe`, `save_meal`, `get_saved_meals`, `delete_saved_meal` and `log_meal_again`.
- The API core holds Open Food Log alone: the exercise candidates on a problem, the workout and
  sentence-parser error codes and their warnings, and the reference server's limits and extras in
  the prose are gone. A problem's `code` is one of `invalid_input`, `consent_required`,
  `not_found`, `undo_unavailable`, `rate_limited`, `idempotency_conflict`, `unauthenticated` and
  `internal_error`; a server may send more, which a client treats as the closest of these.
- The API core (`api/core.yaml`): the REST routes any Open Food Log server serves over a person's
  food log (foods, meals and their change feed, targets and the nutrition summary, the export and webhooks, whose meal events are
  signed in `OFL-Signature`), generated from the reference server. A meal's `external_id` replays
  for the app that sent it, and one the person deleted stays deleted.
- `ofl conformance <base-url>` and `ofl conformance mcp <mcp-url>` check a server against the core
  and the MCP profile as one test person; `ofl vectors` checks an amount calculation against the
  amount vectors (package `conformance`). The reference server runs all three in its tests.
- A Go client for the core (`clients/go`); the TypeScript one is `@openexercise/open-workout-client/openfoodlog`.
- Catalogue: 1,283 liquids gain `density_g_per_ml`, so an amount in ml, l or fl oz can be weighed
  (whole milk is 1.0313 g/ml). Each comes from FoodData Central: a measured specific gravity, or the
  weight of one of the food's own volume portions over its volume, named in `provenance.sources`.
  Liquids with no volume portion, or one outside any liquid's density, have none; `catalogue/REPORT.md`
  lists them. The schemas are unchanged.

## Catalogue v0.6.0, 2026-11-12

The catalogue fills OFL v0.1. Every food keeps its `ofc:` ID, name, aliases, portions and density,
and every value it had is unchanged; nothing is removed.

- **Nutrients:** every one of the 52 keys that FoodData Central gives for a food, read by its number in
  `nutrients.json`: the B vitamins (folate as DFE, folic acid apart), vitamins E and K, phosphorus,
  copper, manganese, selenium, iodine, molybdenum, choline, ALA, EPA, DHA and starch. Folate DFE is
  on 11,298 foods, vitamin K on 9,859, choline on 9,502. A key FoodData Central doesn't give stays
  absent: added sugars, polyols, chromium and chloride aren't in these downloads at all.
- **Worked out by section 4 only:** `energy_kj` (kcal × 4.184) where FoodData Central gives no kJ,
  `salt_g` (sodium × 2.5) and `carbohydrate_available_g` (carbohydrate − fibre), each named in the
  record's `provenance.sources`.
- **Category:** FoodData Central's food category, or a survey food's WWEIA category, on all 12,442
  foods, scheme `fdc`, basis `inferred`. No allergens, diet flags or glycemic values: no CC0 source in
  hand states them.
- `catalogue/REPORT.md` counts every key per source and says why each missing one is missing. The
  FoodData Central releases are the same as v0.3.0's, retrieved again on 2026-11-12. The schemas are
  unchanged.

## v0.1 draft, 2026-11-12

A complete schema: everything here is optional and additive, so every v0 document stays valid.

- **Versions and extensions:** `ofl_version` on every document; minor versions only add (section 13);
  extensions are named `x-<vendor>-…`, with a registry in the README. Nutrient `x-` keys may now
  contain hyphens, for the vendor prefix.
- **Nutrients:** 24 keys to 52: `energy_kj`, ALA, EPA and DHA, `carbohydrate_available_g` (the EU
  basis, without fibre), starch, polyols, `salt_g`, phosphorus, copper, manganese, selenium, iodine,
  chromium, molybdenum, chloride, vitamins E and K, the B vitamins (folate as DFE, folic acid apart),
  choline. Mappings checked against each source again: added sugars gain INFOODS `SUGAD` and FDC
  539; vitamin A gains `VITA_RAE`; alcohol stays unmapped in Health Connect and HealthKit, which
  have no grams of ethanol (now said in its notes). **Correction:** `water_g` no longer maps to
  HealthKit's `dietaryWater`, which is water drunk by volume. Net carbs and IU conversions are
  defined in section 4, never stored.
- **Foods:** `nutrients_basis` (`100g`, `100ml`, `serving`) with `nutrients_per_100ml`; `label`
  (region, declared serving, servings per container, per-serving values, %DV); `category`,
  `allergens` (the EU 14 and US 9, `contains` or `may_contain`), `diet_flags`, `ingredients_text`,
  `glycemic_index` and `glycemic_load`, each `declared` or `inferred`; `kind: supplement` and
  `identifiers.dsld_id`.
- **Amounts:** the `serving` unit, an item's `share`, and amounts counted by volume or serving when a
  food can't be weighed; new codes `serving_unknown` and `serving_weight_unknown` (section 6). Eight
  new amount vectors.
- **Meals:** `timezone` (IANA), `logged_at`, a free `label`, `recorded_by`, `media` references, and
  per-item `share` and `confidence`. Hydration is `water_g`; plain water is an item.
- **Recipes** (`recipe.json`, section 8): lines shaped like meal items, `servings`, `yield_g` or a
  `cooking_yield_factor`; `recipe:` IDs; `recipe-vectors.json`.
- **Targets** (`targets.json`, section 9): `min`, `max` or `target` per day or meal, by weekday, from
  `effective_from`, `set_by`; met, under, above or unknown; `target-vectors.json`.
- **Log:** `recipes` and `targets`.
- **Not in v0:** body metrics, glucose and fasting windows are out of scope, pointing to Health
  Connect, HealthKit and FHIR.
- Examples: a packaged food with its label and attributes, a supplement, a dinner with a shared dish,
  a drink and a recipe, the recipe, targets, and the log holding them.

## Catalogue v0.3.0, 2026-11-10

- 1,283 liquids gain `density_g_per_ml`, so an amount in ml, l or fl oz can be weighed (whole milk is
  1.0313 g/ml). Each comes from FoodData Central: a measured specific gravity, or the weight of one of
  the food's own volume portions over its volume, named in `provenance.sources`. Liquids with no
  volume portion, or one outside any liquid's density, have none; `catalogue/REPORT.md` lists them.
  The schemas are unchanged.

## v0 draft, MCP tool profile, 2026-11-09

The MCP tool profile (`mcp/tools.json`), generated from the reference server: the core `log_meal`,
`get_meals`, `update_meal`, `delete_meal`, `search_foods` and `undo`, marked
`dev.openfoodlog/profile`, with the reference server's `log_meal_text` and `lookup_barcode` as
extensions. Section 9 points at it.

## v0 draft, 2026-11-09

First draft: the food, meal and log schemas, the nutrient vocabulary (24 keys mapped to INFOODS,
FoodData Central, Health Connect and HealthKit), amount vectors, examples, and the specification.
