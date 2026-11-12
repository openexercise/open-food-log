# The Open Food Catalogue

Generic foods in the Open Food Log format, one record per line in [`foods.jsonl`](foods.jsonl), with `ofc:` IDs. Branded and packaged foods aren't here: name them by barcode (`gtin:`) and look them up where they're kept.

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
- **Portions** keep FoodData Central's weights and wording. A survey food's "Quantity not specified" is its default portion: the amount to assume when someone names the food without one. The RACC is "1 serving (RACC)".
- **Densities** (`density_g_per_ml`, so an amount in ml, l or fl oz can be weighed) are given to liquids only: drinks, milks and plant milks, juices, soups and broths, oils, vinegars, syrups and honey, soy and fish sauce. A food is a liquid by FoodData Central's own category (a survey food's WWEIA category, such as "Milk, whole" or "Soft drinks"; an SR Legacy or Foundation food's category, such as "Beverages", narrowed by its name where the category also holds solids), and not when its name says it's a powder or mix before it's made up. The value is FoodData Central's measured specific gravity where there is one, else the weight of the food's largest volume portion over that volume: "1 cup" of whole milk is 244 g in 236.588 ml (a US cup), so 1.0313 g/ml, rounded to four places. "1 fl oz (with ice)" doesn't count. A value outside 0.85 to 1.45 g/ml, beyond any liquid food, means the portion isn't the liquid alone (a shake's foam, a can whose label and weight disagree), so it's left out rather than corrected. Each density's portion is named in the record's `provenance.sources`; [REPORT.md](REPORT.md) lists the liquids without one, and why.
- **IDs** are the name in lower case with runs of other characters made `-`.
- **Aliases** say what people call a food, all of kind `common`:
  - From the name's own shape, with no model: a two-part name with a plain modifier is also that modifier first ("Apple, raw" is "raw apple"), and "X, NFS" (not further specified) is "x".
  - Suggested by Gemini (`cmd/aliases`), which is sent the names only and asked for names only. Its answers are kept in [`sources/aliases.jsonl`](../sources/aliases.jsonl) with the model and date, so building never calls it. Each alias it gave is listed in the record's `provenance.generated`, and the model is named in `provenance.sources`. An alias is kept only if it's short, lowercase, and has no digit the food's name lacks.
  - An alias may belong to more than one food ("milk"). A matcher should then ask which one, not guess.

## Trust it accordingly

The numbers are USDA's, but no record has been checked by a person, and most aliases come from a model. Reviewing records and aliases is the main way to contribute.

## Ask the model again

```bash
OFL_GCP_PROJECT=<project> go run ./cmd/aliases
```

This asks only about foods `sources/aliases.jsonl` doesn't hold yet, 40 at a time, and appends the answers. Delete a food's line to ask about it again.

## Rebuild it

Download the three JSON files from [FoodData Central's downloads](https://fdc.nal.usda.gov/download-datasets) into one directory and unzip them (the survey file unpacks as `surveyDownload.json`), then:

```bash
go run ./cmd/ofl catalogue <that directory>
go test ./...
```
