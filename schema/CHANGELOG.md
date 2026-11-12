# Changelog

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
- **The API core** (`api/core.yaml`): the REST routes any Open Food Log server serves over a person's
  food log (foods, meals and their change feed, targets and the nutrition summary, the export and
  webhooks, whose meal events are signed in `OFL-Signature`), generated from the reference server. A
  meal's `external_id` replays for the app that sent it, and one the person deleted stays deleted.
- **Conformance:** `ofl conformance <base-url>` and `ofl conformance mcp <mcp-url>` check a server
  against the core and the MCP profile as one test person; `ofl vectors` checks an amount calculation
  against the amount vectors, passing each case's `share` and comparing `ml` and grams left
  unknown (package `conformance`). The reference server runs all three in its tests.
- **Clients:** a Go client for the core (`clients/go`); the TypeScript one is
  `@openexercise/open-workout-client/openfoodlog`.

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
