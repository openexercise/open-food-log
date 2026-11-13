# Changelog

The running record of this repository: the Open Food Log format, the Open Food Catalogue, the `ofl`
command, the conformance suite and the Go client. Releases are git tags `vX.Y.Z` (semantic
versioning; there is no version file). Tags number the package and the catalogue; the schema's own
version is `ofl_version` (`"0.2"` today). Until v1 the format may still change, and a release says
so when it does.

[`schema/CHANGELOG.md`](schema/CHANGELOG.md) is the format's own changelog, published from
Lifterclub with the specification; this file covers the whole repository. `git log` has the detail.

## Still to do

Known gaps and deferred work, removed when it ships (and recorded under Unreleased).

- **Model-suggested aliases:** `cmd/aliases` is written but has never been run, so all 1,680 aliases
  come from names' shape (`catalogue/REPORT.md`). `catalogue/README.md` says most come from a model;
  correct it or run the pass.
- **Person review:** no catalogue record has been checked by a person (`catalogue/README.md`).
- **A place for aliases and reviews** that the generator merges in, "still to come"
  ([CONTRIBUTING.md](CONTRIBUTING.md), The catalogue).
- **69 liquids have no density** (`catalogue/REPORT.md`).
- **Nutrients with no CC0 source in hand:** added sugars, polyols, chromium and chloride, and no
  allergens, diet flags or glycemic values (`catalogue/REPORT.md`, v0.6.0).
- **v1:** freezes when the first application outside Lifterclub writes the format in production
  ([README.md](README.md)).

## Unreleased

## v0.7.0 — 2026-11-13

Open Food Log v0.2: saved meals, and recipes, saved meals and logging again in the API core, the MCP
profile and the conformance suite. Additive: every v0.1 document stays valid.

- **Added:** `schema/v0/saved-meal.json`, a meal a person keeps to log again (spec section 8), with
  `saved_meals` in the log and an example; `ofl validate` and the site know it.
- **Added:** the API core's `/v1/recipes`, `/v1/saved-meals` (and `.../log`) and
  `POST /v1/meals/{id}/copy`, and the MCP profile's `save_recipe`, `get_recipes`, `delete_recipe`,
  `save_meal`, `get_saved_meals`, `delete_saved_meal` and `log_meal_again` in the core.
- **Added:** conformance checks `copy`, `saved-meal` and `recipe` (`ofl conformance`) and `again`
  (`ofl conformance mcp`). A server that passed v0.6.0's suite must now serve these too to pass.
- **Added:** the Go client's recipe, saved meal and log-again calls.
- **Changed:** the A2A profile documents the reference agent's `log-meal-photo` extension and how to
  check an agent; the API core's wording catches up with the reference server.

## v0.6.0 — 2026-11-12

The catalogue fills Open Food Log v0.1 (#7). Schemas unchanged; every food keeps its ID, name,
aliases, portions, density and existing values.

- **Added:** every v0.1 nutrient key FoodData Central gives (B vitamins with folate as DFE, vitamins
  E and K, trace minerals, choline, ALA, EPA, DHA, starch).
- **Added:** `energy_kj`, `salt_g` and `carbohydrate_available_g` worked out by spec section 4 only,
  each named in `provenance.sources`.
- **Added:** a FoodData Central (or WWEIA) category on all 12,442 foods, basis `inferred`.
- **Fixed:** units compare case-insensitively, so FoodData Central's `kJ` matches.
- `catalogue/REPORT.md` counts every key per source and says why each missing one is missing.

## v0.5.0 — 2026-11-12

The A2A profile and its conformance check (#6). Additive; the schema stays at `ofl_version: "0.1"`.

- **Added:** `a2a/agent-card.json` and `a2a/README.md`, what a compatible nutrition agent serves over
  A2A, embedded as `openfoodlog.A2AProfile`.
- **Added:** `ofl conformance a2a <agent-url>` (`conformance.RunA2A`): card, discovery, logging and
  replay, input-required, cancel, reads, refusals and read-only tokens over A2A 1.0 JSON-RPC.

## v0.4.0 — 2026-11-12

Open Food Log schema v0.1, the API core, a conformance suite and a Go client (#4, #5). Everything is
additive: every v0 document stays valid. The catalogue data is unchanged.

- **Added:** schema v0.1: 52 nutrient keys (up from 24), food labels and attributes, supplements,
  the `serving` unit and `share`, meal `timezone`/`logged_at`/`media`/`confidence`, `recipe.json`
  and `targets.json` with vectors, `ofl_version` and an extension registry.
- **Fixed:** `water_g` no longer maps to HealthKit's `dietaryWater`.
- **Added:** `api/core.yaml`, the REST routes any OFL server serves, and the MCP profile in `mcp/`.
- **Added:** `ofl conformance <base-url>`, `ofl conformance mcp <mcp-url>` and `ofl vectors`, with
  `VerifySignature` for `OFL-Signature` webhooks.
- **Added:** `clients/go`, a standard-library Go client for the core (tested in CI).
- **Added:** `Measure`, the reference section 6 amount calculation.

## v0.3.0 — 2026-11-10

- **Added:** `density_g_per_ml` on 1,283 of 1,352 liquids, from FoodData Central's specific gravity
  or a volume portion's weight (#3), so an amount in ml or fl oz can be weighed.

## v0.2.0 — 2026-11-09

- **Added:** 1,680 catalogue aliases from names' own shape ("Apple, raw" is also "raw apple") (#2).
- **Added:** `cmd/aliases`, a Gemini pass that suggests aliases; included but not yet run.

## v0.1.0 — 2026-11-09

First release of the Open Food Log v0 draft and the Open Food Catalogue (#1).

- The specification, food, meal and log schemas, the nutrient vocabulary (24 keys mapped to INFOODS,
  FoodData Central, Health Connect and HealthKit) and amount vectors.
- The Open Food Catalogue: 12,442 generic foods with `ofc:` IDs from USDA FoodData Central, CC0.
- Package `openfoodlog` (validator, `Grams`, `Nutrients`) and the `ofl` command (`validate`,
  `catalogue`, `site`).
