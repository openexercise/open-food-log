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

- **Review the model aliases:** 6,524 of them, none checked by a person. 432 names were given to
  several foods and kept on one by rule; those are the likeliest to be wrong. "Spaghetti and
  meatballs" finds a canned version (the catalogue has no everyday one). Correct them in
  `sources/curated-aliases.jsonl` and add a line to `sources/alias-checks.jsonl`.
- **Person review:** no catalogue record has been checked by a person (`catalogue/README.md`).
- **A place for reviews** that the generator merges in (aliases have one now:
  `sources/curated-aliases.jsonl`).
- **69 liquids have no density** (`catalogue/REPORT.md`).
- **Nutrients with no CC0 source in hand:** added sugars, polyols, chromium and chloride, and no
  allergens, diet flags or glycemic values (`catalogue/REPORT.md`, v0.6.0).
- **v1:** freezes when the first application outside Lifterclub writes the format in production
  ([README.md](README.md)).

## Unreleased

## v0.12.0 — 2026-11-17

USDA's Branded Foods, 396,916 US products by barcode, are published as `branded-foods.jsonl.gz`,
an asset of this release (SHA-256 in `branded-foods.jsonl.gz.sha256`). Every v0.2 document stays
valid.

- **Added:** `ofl catalogue branded <download>` builds the branded catalogue,
  `branded-foods.jsonl.gz`, from USDA FoodData Central's Branded Foods by the rules
  `branded-report` measures: one OFL food per US product (`kind: branded`, `gtin:` ID, `brand`, the
  US `label` with its serving and per-serving values, nutrients per 100 g or per 100 ml as stated,
  `ingredients_text`, FoodData Central's category, and provenance naming the release, the FDC
  record, the data provider and its dates), ordered by ID and validated against the food schema,
  with a `.sha256` beside it. It's a release asset, never committed; `catalogue/README.md` covers
  its licence (CC0, cite USDA), how apps should use it and how to release it. From the 2026-10-29
  download: 396,916 records, 69.0 MB (648 MB unzipped), SHA-256
  `926a9b6b52adce40f3d7511fcdd46fbd643d1293267e88a2548f2f45211e1555`.
- **Changed:** `BRANDED-REPORT.md` measures the file's sizes from the records `ofl catalogue
  branded` writes, not a draft shape.
- **Added:** `ofl catalogue branded-report <file>` measures USDA FoodData Central's Branded Foods
  download against the rules for a branded catalogue and writes
  [`catalogue/BRANDED-REPORT.md`](catalogue/BRANDED-REPORT.md), writing no records. The rules: the
  US market (written `United States` or `US`, kept as `US`); a GTIN zero-padded to 14 digits with a
  valid check digit (an 11-digit code is a UPC-A that lost its leading zero); the newest record per
  GTIN; not discontinued; and plausible numbers (energy within 20% or 40 kcal of 4·protein +
  4·carbohydrate + 9·fat). A serving given in MG, IU or MC is read as grams or millilitres when the
  label's calories confirm it. Of the 2026-10-29 release's 434,001 records, 396,916 are kept: about
  85 MB gzipped with ingredients, 53 MB without.

## v0.11.0 — 2026-11-17

Restaurant and fast-food dishes, and the chain items FoodData Central names ("big mac",
"whopper"), are found by the names people log them by. Every v0.2 document stays valid.

- **Added:** restaurant and fast-food dishes have the names people log them by. A `/food-aliases`
  run over the 354 foods whose name says "fast food" or "restaurant" gave them 504 aliases ("fast
  food cheeseburger", "drive-through fries", "takeout orange chicken", "restaurant mashed
  potatoes"); 39 of them still have none, down from 186. Plain dish names stay with the everyday
  record ("fries", "cheeseburger"), except restaurant pizza, which keeps "cheese pizza" and
  "pepperoni pizza". "Pizza" and two "baked chicken thigh" names were dropped from the run's
  answers as too plain for a restaurant's version.
- **Added:** the 14 chain items FoodData Central names outright have their plain names, chosen by a
  person in `sources/curated-aliases.jsonl`: "big mac", "mcdouble", "quarter pounder", "quarter
  pounder with cheese", "whopper", "whopper jr" and the rest, plus "mcdonalds …" and "burger king
  …" forms of the hamburgers and cheeseburgers. SR Legacy's 2018 chain records stay out.
- **Added:** 50 more phrases in `sources/alias-checks.jsonl` (160 in all): every chain alias, the
  new restaurant names, and the plain names that must stay with the everyday food.
- **Added:** `cmd/aliases batches -name` picks foods by their own name, across families; the
  skill's prompt says how to name a restaurant's version.

## v0.10.1 — 2026-11-17

Everyday words find the food a person means for sure in four more cases, and 110 phrases now
guard the catalogue's names. Every v0.2 document stays valid.

- **Fixed:** "scrambled eggs" finds the survey's egg omelet or scrambled egg (a model had given it
  to the fast-food one), "mac and cheese" and "macaroni and cheese" find "Macaroni or noodles with
  cheese" (not the SR Legacy box mix), "banana" and "bananas" find "Banana, raw" (no food held the
  plain word), and "almond" and "almonds" both find "Almonds, NFS" (the singular was another
  record's, so neither was sure). Each is in `sources/curated-aliases.jsonl` with its reason.
- **Added:** 90 phrases in `sources/alias-checks.jsonl` (110 in all): coffee, rice, chicken
  breast, yogurt, steak, sushi and other everyday foods, each held by the one food it names.

## v0.10.0 — 2026-11-16

The catalogue gets the names people log foods by: 6,961 foods have aliases (1,807 before), most
suggested by Claude Haiku through the new `/food-aliases` skill, each on one food only. Every v0.2
document stays valid.

- **Added:** model-suggested aliases for the whole catalogue, from `/food-aliases` (Claude Haiku,
  `claude-haiku-5-5`) over the 12,140 foods the trial didn't cover. 6,961 foods now have aliases
  (1,807 before), 6,524 of them suggested by the model ("french fries", "hash browns", "salmon",
  "pot roast"), each listed in its record's `provenance.generated`. None is shared by two foods.
- **Changed:** a model's alias stays on one food. When a model gives one alias to several foods
  (asked a family at a time, it gave "meatloaf" to a beef record as well as the meat loaf), the
  build keeps it on the record USDA leaves unspecified, else one with a default portion, else the
  shortest name, and never on a food whose name or rule or person-chosen alias another food
  already has. `cmd/aliases merge` no longer drops a name two batches gave to one family; the
  build decides.

- **Added:** the `/food-aliases` Claude Code skill (`.claude/skills/food-aliases`), which asks
  Claude Haiku what people call the catalogue's foods, a family of related records at a time, so
  a dish's plain name goes to one food alone ("meatloaf" to the survey's unspecified meat loaf,
  "turkey meatloaf" to the turkey one). `cmd/aliases batches` writes its questions and
  `cmd/aliases merge` checks the answers (every food answered, no alias given to two foods of a
  family or taken from a rule's or a person's) before appending them to `sources/aliases.jsonl`.
- **Added:** `sources/alias-checks.jsonl` and `TestAliasChecks`: phrases that must name one food
  alone, the one listed, so a model run that moves or doubles a plain name fails `go test`.
- **Added:** the first model-suggested aliases, from a trial run of `/food-aliases` (Claude Haiku,
  `claude-haiku-5-5`) over 302 foods in 11 families: meat loaf, orange juice, oatmeal, pancakes,
  waffles, burritos, chili, hamburgers, French toast, grilled cheese and egg omelets. 166 aliases
  on 119 more foods ("meatloaf", "beef meatloaf", "oj", "bean burrito"), none shared by two foods;
  each is listed in its record's `provenance.generated`.

## v0.9.2 — 2026-11-16

Ingredient-only foods lose a drink's placeholder portions, and `ofl catalogue refresh` reapplies
record rules without the FoodData Central downloads. Every v0.2 document stays valid.

- **Fixed:** 37 ingredient-only survey foods ("Tomatoes as ingredient in omelet", "Rice, white,
  cooked, as ingredient", "Industrial oil as ingredient in food"…, WWEIA category 9999) no longer
  carry a drink's portions ("1 fl oz (with ice)", and a 248 g glass as the default). The survey
  download gives them that placeholder; FoodData Central's API lists no portions for them, so they
  have none, and their `provenance.sources` says why. 5,304 survey foods keep a default portion.
- **Changed:** `ofl catalogue aliases` is now `ofl catalogue refresh` (the old name still works):
  it reapplies every rule that works on records alone, portions as well as aliases, and updates
  `REPORT.md`'s "Kept without portions" from the records. `TestCatalogueIsRefreshed` replaces
  `TestCatalogueAliasesAreCurrent`.

## v0.9.1 — 2026-11-16

Cheese omelets are found by name. Aliases only; every v0.2 document stays valid.

- **Added:** cheese omelet aliases ("cheese omelet", "cheese omelette", "omelet with cheese",
  "omelette with cheese", "cheesy scrambled egg", "scrambled egg(s) with cheese") on
  `ofc:egg-omelet-or-scrambled-egg-with-cheese-made-with-butter`. FoodData Central has no cheese
  omelet with the fat not further specified; butter is nearest to its plain omelet's blend.

## v0.9.0 — 2026-11-16

Curated aliases: the catalogue now finds everyday egg foods by what people call them, and aliases
can be changed without the FoodData Central downloads. Every v0.2 document stays valid.

- **Added:** curated aliases. `sources/curated-aliases.jsonl` holds aliases a person chose, with
  why, merged into every build after the name rules and named in `provenance.sources`; `ofl
  catalogue aliases` attaches aliases again over the built catalogue without the FoodData Central
  downloads. The first 14 fix eggs: "egg" found "Egg, creamed" (the shortest of ~50 names starting
  "Egg"), "fried egg" found "Fried eggplant", and "omelet" or "omelette" found nothing. Each now
  names one survey food whose usual portion is one egg.

## v0.8.0 — 2026-11-16

The documents, examples and vectors catch up with v0.2, the API core names an unavailable barcode
lookup, and the guides are on the site. Every v0.2 document stays valid.

- **Released:** the Go client's first tag, `clients/go/v0.1.0` (commit 8bd048d), the API core for
  OFL v0.2. `go get github.com/openexercise/open-food-log/clients/go@v0.1.0` replaces a pseudo-version
  of `main`.
- **Fixed:** the README's conformance paragraph names the v0.2 checks (copy, saved meals, recipes,
  `log_meal_again`) and the vectors' `share`; `catalogue/README.md` no longer says the aliases come
  from a model (the alias pass hasn't run; all 1,680 come from names) or that every food has a
  default portion (survey foods do, 5,341 of 5,410; SR Legacy and Foundation foods don't), and
  drops its link to `sources/aliases.jsonl`, which doesn't exist yet.
- **Added:** the developer guide is on the site at [openfoodlog.org/guide](https://openfoodlog.org/guide/),
  linked from the navigation and the home page. `ofl site` rewrites the relative links in every
  Markdown page it renders (the guide, the specification, the changelog) to the site's own pages
  and files, else to the file on GitHub, gives headings GitHub's IDs so section links work, and
  fails on a link to a file the repository lacks; `TestSiteLinks` checks every built page's links.
  This also fixes the specification's links to `api/core.yaml`, `mcp/tools.json` and
  `schema/README.md`, which went nowhere on the site.
- **Fixed:** the specification, MCP profile README, examples and vectors caught up with v0.2
  (published from Lifterclub): section 2 lists `saved-meal.json`, section 3 the `saved:` and
  `example:` namespaces, section 11 and `mcp/README.md` every core and extension tool, and saved
  meals are no longer "Not in v0". The examples' and vectors' foods moved from `ofc:` IDs the
  catalogue doesn't have to the `example:` namespace (numbers unchanged), `examples/log.json` is
  v0.2 with saved and deleted meals, and the developer guide quotes the new IDs.
- **Changed:** the API core answers a barcode that can't be looked up just now with
  `barcode_unavailable` (503, `Retry-After`), where it said `internal_error`.
- **Added:** a developer guide (`docs/developer-guide.md`), walking through every document, the
  amount calculation, targets, the catalogue and checking a server, and a full contributor guide
  (`CONTRIBUTING.md`) saying which paths are published from Lifterclub and which are made here.

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
