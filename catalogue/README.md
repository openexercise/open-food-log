# The Open Food Catalogue

Generic foods in the Open Food Log format, one record per line in [`foods.jsonl`](foods.jsonl), with `ofc:` IDs. Branded and packaged foods aren't here: name them by barcode (`gtin:`). USDA's Branded Foods (CC0), about 400,000 US products, are published beside it as a release asset, [`branded-foods.jsonl.gz`](#branded-foods-branded-foodsjsonlgz), too large for git.

**Status: v0 draft.** IDs may change until v1. No record has been reviewed by a person; each says so in `provenance.review`.

## Licence

The catalogue is dedicated to the public domain under [CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/), like its source. Use it for anything, without asking. A credit to "Open Food Catalogue, from USDA FoodData Central" is welcome, not required. The code and schemas in this repository are Apache-2.0.

## Where the numbers come from

Every record is built from [USDA FoodData Central](https://fdc.nal.usda.gov) (public domain, CC0 1.0), retrieved 2026-11-12, by `ofl catalogue`:

| Source | Release | What it adds |
| --- | --- | --- |
| Foundation Foods | 2026-10-29 | The newest laboratory analyses of single foods |
| FNDDS survey foods | 2024-10-31 | Everyday foods and dishes as Americans report eating them, with household portions |
| SR Legacy | 2018-04 | The broad reference set, frozen since 2018 |

The rules, so anyone can check a record against its source (`identifiers.fdc_id` and `provenance.sources`):

- **One record per name.** When two sources describe a food by the same name, the first in the table above wins; the others are listed in `provenance.sources` but their numbers aren't used. [REPORT.md](REPORT.md) counts every case, and how many foods have each nutrient.
- **Nutrients** are every key of the v0.1 vocabulary that FoodData Central gives for the food, read by the FoodData Central numbers in [`nutrients.json`](../schema/v0/nutrients.json), in the vocabulary's unit only: the B vitamins (folate as DFE, folic acid apart), vitamins E and K, phosphorus, copper, manganese, selenium, iodine, molybdenum, choline, ALA, EPA and DHA, starch and the rest. Where a food lacks one, a stated alternative is read instead: energy from Atwater specific (958) then general (957) factors, fat from NLEA total fat (298), carbohydrate by summation (205.2, which like the total by difference includes fibre), sugars (269.3) and fibre (293) by newer methods. Nothing missing is filled in, and an amount in another unit (IU) isn't converted. A key a food doesn't have is unknown, never zero.
- **Three keys are worked out**, by the rules of [section 4 of the spec](../schema/v0/SPEC.md#4-nutrients) and no others, and each record that has one names them in `provenance.sources`:
  - `energy_kj` is `energy_kcal` × 4.184 where FoodData Central gives no kilojoules (268). SR Legacy and some Foundation Foods give them; survey foods don't.
  - `salt_g` is sodium × 2.5, as EU and UK labels declare it (`sodium_mg` × 0.0025).
  - `carbohydrate_available_g` is `carbohydrate_g` − `fiber_g`, where both are given and fibre isn't above the total.
- **Category** is FoodData Central's own: SR Legacy's and Foundation Foods' food category ("Dairy and Egg Products"), or a survey food's WWEIA category with its number ("Milk, whole", `1002`), all with `scheme` `fdc`. It's `inferred`: FoodData Central files the food under it, but no label or analysis declares it.
- **Allergens, diet flags, ingredients and glycemic index or load** are left out: FoodData Central's generic foods don't state them, and the catalogue has no other CC0 source that does. Working them out from names would be guessing.
- **Foods with no energy value** in any of those forms are left out, rather than given one.
- **Branded records** in SR Legacy (named in capitals: `KRAFT`, `SUBWAY`) are left out. Capitalised words used for generic foods (`USDA`, `BBQ`, `NFS`, `NFSMI`, `RTE`, `UHT`) don't count.
- **Portions** keep FoodData Central's weights and wording. A survey food's "Quantity not specified" is its default portion (`is_default`): the amount to assume when someone names the food without one. 5,304 of the 5,410 survey foods have one; SR Legacy and Foundation foods have none, so an app asks for an amount or uses a serving it chooses. The RACC is "1 serving (RACC)".
- **Placeholder portions are left out.** The survey download gives 37 ingredient-only foods (WWEIA category 9999, such as "Tomatoes as ingredient in omelet" and "Industrial oil as ingredient in food") a drink's portions: an ounce with ice, without ice and not further specified, and a 248 g glass when the amount isn't said. FoodData Central's own API lists no portions for them, so they have none here, and their `provenance.sources` says so. Drinks keep those measures.
- **Densities** (`density_g_per_ml`, so an amount in ml, l or fl oz can be weighed) are given to liquids only: drinks, milks and plant milks, juices, soups and broths, oils, vinegars, syrups and honey, soy and fish sauce. A food is a liquid by FoodData Central's own category (a survey food's WWEIA category, such as "Milk, whole" or "Soft drinks"; an SR Legacy or Foundation food's category, such as "Beverages", narrowed by its name where the category also holds solids), and not when its name says it's a powder or mix before it's made up. The value is FoodData Central's measured specific gravity where there is one, else the weight of the food's largest volume portion over that volume: "1 cup" of whole milk is 244 g in 236.588 ml (a US cup), so 1.0313 g/ml, rounded to four places. "1 fl oz (with ice)" doesn't count. A value outside 0.85 to 1.45 g/ml, beyond any liquid food, means the portion isn't the liquid alone (a shake's foam, a can whose label and weight disagree), so it's left out rather than corrected. Each density's portion is named in the record's `provenance.sources`; [REPORT.md](REPORT.md) lists the liquids without one, and why.
- **IDs** are the name in lower case with runs of other characters made `-`.
- **Aliases** say what people call a food, all of kind `common`:
  - From the name's own shape, with no model: a two-part name with a plain modifier is also that modifier first ("Apple, raw" is "raw apple"), and "X, NFS" (not further specified) is "x".
  - Chosen by a person in `sources/curated-aliases.jsonl`, with why that food: "egg", "fried egg", "boiled egg", "omelet", "omelette", "scrambled egg", "cheese omelette", "egg white" and a few more, each naming a survey food with one egg (or white, or yolk) as a portion. They're named in the record's `provenance.sources`. `ofl catalogue refresh` attaches aliases again without the FoodData Central downloads.
  - Suggested by a model, which is sent names only and asked for names only: Claude Haiku through the `/food-aliases` skill (`.claude/skills/food-aliases`), or Gemini (`cmd/aliases` with no subcommand, not run yet). The skill asks about a **family** at a time (every "Meat loaf …" record together), so a dish's plain name ("meatloaf") goes to the one record a person means and its neighbours get names that say how they differ ("turkey meatloaf"). The answers are kept in `sources/aliases.jsonl` with the model and date, so building never calls it; each alias a model gives is listed in the record's `provenance.generated`, and the model is named in `provenance.sources`. An alias is kept only if it's short, lowercase, and has no digit the food's name lacks, and never if it's another food's name or a name a rule or a person gave another food. A model's alias stays on one food only: when a model gives one to several ("french fries" to a fast-food, a frozen and a home record), the build keeps it on the record USDA leaves unspecified ("NFS", "NS as to …"), else one with a default portion, else the shortest name. No person reviews them: [`sources/alias-checks.jsonl`](../sources/alias-checks.jsonl) lists phrases that must name one food alone, the one listed, and `go test` fails when a run breaks one.
  - A rule's or a person's alias may belong to more than one food ("milk"). A matcher should then ask which one, not guess.
  - Withheld by a person in `sources/withheld-aliases.jsonl`, with why: a name too vague for any one food ("baby peaches", strained or diced), which no model alias may hold, so a matcher asks.

## Trust it accordingly

The numbers are USDA's, but no record has been checked by a person, and the aliases are worked out from names by rule, so they're few and plain. Reviewing records and aliases is the main way to contribute.

## Ask the model again

In Claude Code, from this repository's root:

```
/food-aliases                 # every food sources/aliases.jsonl doesn't hold yet
/food-aliases "meat loaf"     # only families whose plain name contains the text
/food-aliases name:"fast food|restaurant"   # only foods whose own name contains the text
```

`go run ./cmd/aliases batches` writes the questions, Haiku subagents answer one batch each, and `go run ./cmd/aliases merge` checks every answer (each food answered, no name given to two foods of one family) before appending it. Or ask Gemini, one food at a time:

```bash
OFL_GCP_PROJECT=<project> go run ./cmd/aliases
```

Either asks only about foods `sources/aliases.jsonl` doesn't hold yet (the file is made on the first run) and appends the answers; a later line for a food replaces an earlier one. Delete a food's line, or pass `batches -redo`, to ask about it again. Add a line to `sources/alias-checks.jsonl` before changing the skill's prompt.

## Rebuild it

Download the three JSON files from [FoodData Central's downloads](https://fdc.nal.usda.gov/download-datasets) into one directory and unzip them (the survey file unpacks as `surveyDownload.json`), then:

```bash
go run ./cmd/ofl catalogue <that directory>
go test ./...
```

## Branded foods (`branded-foods.jsonl.gz`)

US packaged products from USDA FoodData Central's [Global Branded Food Products Database](https://fdc.nal.usda.gov), one OFL food per line, gzipped, ordered by ID. It's attached to the catalogue's [GitHub releases](https://github.com/openexercise/open-food-log/releases) as an asset with its checksum (`branded-foods.jsonl.gz.sha256`), never committed: at about 70 MB it would pass GitHub's file limit and grow the history by that much each release. A release that doesn't rebuild it carries no asset; use the latest release that has one.

**Licence.** CC0 1.0, like its source: USDA FoodData Central data are in the public domain. USDA asks to be named as the source, and so do we. Cite it as *U.S. Department of Agriculture, Agricultural Research Service. FoodData Central: USDA Global Branded Food Products Database, release 2026-10-29. fdc.nal.usda.gov*, and show "USDA FoodData Central" where your app shows a branded food's numbers. The data is submitted by manufacturers and retailers (through Label Insight, GS1's GDSN or Euromonitor), who USDA says are responsible for it.

**Each record:**

- `id` is `gtin:` and the product's barcode as 14 digits; `gtin` lists the same barcode. `kind` is `branded`.
- `name` is USDA's description, sentence-cased when it was written all in capitals ("Original greek yogurt"); the original is quoted in `provenance.sources`. `brand` is the brand name, else the brand owner.
- `nutrients_per_100g` holds the nutrients FoodData Central states per 100 g, read by the vocabulary's FoodData Central numbers as generic foods are, in the vocabulary's unit only. Nothing is worked out (no `energy_kj`, `salt_g` or `carbohydrate_available_g`); compute them by section 4 of the spec if you need them. A product labelled per 100 ml has `nutrients_basis: "100ml"`, its values in `nutrients_per_100ml`, and an empty `nutrients_per_100g`, since no density is known: log it by volume or by serving.
- `label` is the US label (`region: us`): the serving as printed (`serving.text`, else its size), its weight in `grams` or volume in `ml`, and `nutrients_per_serving` as declared (calories, macros, sodium, cholesterol, sugars and added sugars, fibre, calcium, iron, potassium; vitamin D is left out, since the download doesn't say whether it's IU or µg). `carbohydrate_g` is total carbohydrate, as US labels give it.
- `ingredients_text` is the label's ingredients list (`basis: declared`, `locale: en`), on one line; 99.4% of records have one.
- `category` is FoodData Central's branded category (`scheme: fdc`, `basis: inferred`).
- `identifiers.fdc_id` is USDA's record; `provenance.sources` names the release, the record's URL, the data provider, how it reached USDA, and the record's available, modified and published dates. No record has been reviewed by a person.

**The rules**, measured in [BRANDED-REPORT.md](BRANDED-REPORT.md) (`ofl catalogue branded-report`), which counts every record each one drops:

1. The US market only (written `United States` or `US`).
2. A GTIN: up to 14 digits, zero-padded on the left, with a valid check digit (an 11-digit code is a UPC-A that lost its leading zero).
3. One record per GTIN, the newest by available date, then modified date, then the highest FDC ID.
4. Not discontinued. There's no age cutoff: an old label isn't wrong, and the product may still be in someone's cupboard. About 65% of records are dated over five years before the release.
5. Plausible numbers: energy in kcal; no negative value or serving of 0; protein, fat and carbohydrate no more than 100 g per 100, energy no more than 900 kcal; energy within 20% or 40 kcal of 4·protein + 4·carbohydrate + 9·fat (+ 7·alcohol). A serving in g or ml names the basis; one given in MG, IU or MC is read as grams or millilitres when the label's calories confirm it, else the record is dropped.

**Using it.** Read it a line at a time (`gzip -dc branded-foods.jsonl.gz | head`); the whole file is about 650 MB of JSON. Check the download against its `.sha256` (`shasum -a 256 -c branded-foods.jsonl.gz.sha256`) and pin the checksum, so a later release changes your data only when you choose. Things to know:

- **Look products up by barcode.** Names repeat across brands and sizes, and outnumber the generic catalogue thirty to one: a matcher that treats a branded name as a sure match for a person's words will stop being sure about "banana". Rank branded results below generic foods, and ask before choosing one from words alone.
- **When a product leaves a release** (discontinued, or its record failed a rule), keep what you loaded and stop offering it; a meal that named it keeps its numbers.
- **Labels lag.** A manufacturer's submission can trail a reformulation; FoodData Central updates its downloads twice a year (April and October). Another product database may be newer for one product; say which source a number came from.
- A meal item logged from a branded food should name it by its `gtin:` ID and keep its own nutrients, as for any food.

**Build it** from the unzipped JSON download (`FoodData_Central_branded_food_json_YYYY-MM-DD.json`, about 3 GB unzipped, from FoodData Central's downloads), anywhere outside the repository:

```bash
go build -o ~/bin/ofl ./cmd/ofl        # in a checkout
cd <a scratch directory>
ofl catalogue branded <path>/FoodData_Central_branded_food_json_2026-10-29.json
```

It writes `branded-foods.jsonl.gz` and `branded-foods.jsonl.gz.sha256` in the current directory (or the path given after the download), validates every record against the food schema, and stops on the first that fails. The same download and Go version build the same bytes, so a checksum can be reproduced. The 2026-10-29 download took about two minutes and 1.5 GB of memory on an Apple-silicon Mac.

**Release it** by hand, after the pull request that changes the rules (if any) has merged and `CHANGELOG.md` names the release:

```bash
gh release create vX.Y.Z --title vX.Y.Z --notes-file <notes> branded-foods.jsonl.gz branded-foods.jsonl.gz.sha256
```

or `gh release upload vX.Y.Z branded-foods.jsonl.gz branded-foods.jsonl.gz.sha256` for a release that exists. Put the record count and SHA-256 in the release notes and the changelog entry. The repository is public, so the asset downloads without a token from `https://github.com/openexercise/open-food-log/releases/download/vX.Y.Z/branded-foods.jsonl.gz`.

| Built from | Records | Size | SHA-256 |
| --- | --- | --- | --- |
| 2026-10-29 (built with Go 1.26.2) | 396,916 | 68,970,860 bytes (69.0 MB; 648 MB unzipped) | `926a9b6b52adce40f3d7511fcdd46fbd643d1293267e88a2548f2f45211e1555` |
