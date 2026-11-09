# The Open Food Catalogue

Generic foods in the Open Food Log format, one record per line in [`foods.jsonl`](foods.jsonl), with `ofc:` IDs. Branded and packaged foods aren't here: name them by barcode (`gtin:`) and look them up where they're kept.

**Status: v0 draft.** IDs may change until v1. No record has been reviewed by a person; each says so in `provenance.review`.

## Licence

The catalogue is dedicated to the public domain under [CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/), like its source. Use it for anything, without asking. A credit to "Open Food Catalogue, from USDA FoodData Central" is welcome, not required. The code and schemas in this repository are Apache-2.0.

## Where the numbers come from

Every record is built from [USDA FoodData Central](https://fdc.nal.usda.gov) (public domain, CC0 1.0), retrieved 2026-11-09, by `ofl catalogue`:

| Source | Release | What it adds |
| --- | --- | --- |
| Foundation Foods | 2026-10-29 | The newest laboratory analyses of single foods |
| FNDDS survey foods | 2024-10-31 | Everyday foods and dishes as Americans report eating them, with household portions |
| SR Legacy | 2018-04 | The broad reference set, frozen since 2018 |

The rules, so anyone can check a record against its source (`identifiers.fdc_id` and `provenance.sources`):

- **One record per name.** When two sources describe a food by the same name, the first in the table above wins; the others are listed in `provenance.sources` but their numbers aren't used. [REPORT.md](REPORT.md) counts every case.
- **Nutrients** are read by the FoodData Central numbers in [`nutrients.json`](../schema/v0/nutrients.json), in the vocabulary's unit only. Where a food lacks one, a stated alternative is read instead: energy from Atwater specific (958) then general (957) factors, fat from NLEA total fat (298), carbohydrate by summation (205.2), sugars (269.3) and fibre (293) by newer methods. Nothing missing is filled in or calculated.
- **Foods with no energy value** in any of those forms are left out, rather than given one.
- **Branded records** in SR Legacy (named in capitals: `KRAFT`, `SUBWAY`) are left out. Capitalised words used for generic foods (`USDA`, `BBQ`, `NFS`, `NFSMI`, `RTE`, `UHT`) don't count.
- **Portions** keep FoodData Central's weights and wording. A survey food's "Quantity not specified" is its default portion: the amount to assume when someone names the food without one. The RACC is "1 serving (RACC)".
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
