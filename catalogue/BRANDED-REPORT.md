# Branded foods report

What `ofl catalogue branded-report` found in USDA FoodData Central's Global Branded Food Products Database, the JSON download of 2026-10-29 (`FoodData_Central_branded_food_json_2026-10-29.json`), measured against the filtering rules planned for a branded catalogue. It writes no records: this is phase 0 of the plan, and nothing here is in `foods.jsonl`.

## Each rule, in order

| Step | Records | Dropped |
| --- | --- | --- |
| Read | 434001 | |
| 1. US market only (`United States` or `US`) | 432894 | 1107 |
| 2. A valid GTIN (8, 12, 13 or 14 digits, check digit) | 420988 | 11906 |
| 3. One per GTIN, newest kept | 420987 | 1 |
| 4. Not discontinued | 418977 | 2010 |
| 5. Plausible numbers | 395815 | 23162 |
| **Kept** | **395815** | |

### 1. Markets

| Market | Records |
| --- | --- |
| United States | 421218 |
| US | 11676 |
| New Zealand | 1107 |

US records by `dataSource` (GDSN is GS1's network; LI is Label Insight):

| Source | Records |
| --- | --- |
| LI | 404553 |
| GDSN | 25946 |
| Euromonitor | 2395 |

### 2. GTINs

Why a US record's `gtinUpc` isn't a GTIN:

| Reason | Records |
| --- | --- |
| wrong length | 9858 |
| check digit | 2032 |
| not digits | 16 |

Of the wrong lengths, 1162 would pass the check digit once zero-padded to 14 digits (by length: 11 digits 1129; 10 digits 29; 6 digits 2; 7 digits 1; 9 digits 1; an 11-digit code is most likely a UPC-A that lost its leading zero). The planned rule drops them; padding them would keep them.

Length of `gtinUpc` as given, US records:

| Digits | Records |
| --- | --- |
| 12 | 362452 |
| 14 | 28813 |
| 13 | 28727 |
| 11 | 9632 |
| 8 | 3036 |
| 10 | 189 |
| 7 | 20 |
| 6 | 8 |
| 9 | 8 |
| 15 | 4 |
| 17 | 2 |
| 16 | 1 |
| 18 | 1 |
| 5 | 1 |

### 3. Repeated GTINs

GTINs (as 14 digits) that more than one `fdcId` carries: 1, with 1 records beyond the first of each and at most 2 of one GTIN. Earlier versions of a product sit in each record's `foodUpdateLog`, not as records of their own. Where a GTIN repeats, the newest is kept, by `availableDate`, then `modifiedDate`, then the highest `fdcId`.

### 4. Discontinued

2010 of the records left after step 3 have a `discontinuedDate`.

### 5. Plausibility

A record counts under the first rule it breaks.

| Rule | Records |
| --- | --- |
| a negative value, or a serving of 0 | 65 |
| no energy in kcal | 4882 |
| protein + fat + carbohydrate over 100 g per 100 | 5740 |
| energy over 900 kcal per 100 | 431 |
| energy more than 20% and 40 kcal from 4·protein + 4·carbohydrate + 9·fat (+ 7·alcohol) | 12044 |

3 records with no energy in kcal give kilojoules (268) instead. 5872 records with energy lack protein, fat or carbohydrate, so the Atwater check can't be made on them; they're kept.

The Atwater check at other tolerances, over every remaining record with energy and the three macros, before the other plausibility rules (a record is off when it's past both bounds):

| Tolerance | Off |
| --- | --- |
| 10% and 20 kcal | 36054 |
| 20% and 40 kcal (planned) | 13237 |
| 30% and 50 kcal | 9273 |
| 50% and 80 kcal | 6399 |

## What's kept

### Age

Year of each kept record's `availableDate` (else `modifiedDate`, else `publicationDate`):

| Year | Records |
| --- | --- |
| 2014 | 5 |
| 2016 | 201 |
| 2017 | 59389 |
| 2018 | 70688 |
| 2019 | 26017 |
| 2020 | 54940 |
| 2021 | 47529 |
| 2022 | 32866 |
| 2023 | 89072 |
| 2024 | 1550 |
| 2025 | 3901 |
| 2026 | 9657 |

255023 kept records (64.4%) are over 5 years old (dated before 2021-10-29). 0 have no readable date.

Kept records by `dataSource`:

| Source | Records |
| --- | --- |
| LI | 371069 |
| GDSN | 22890 |
| Euromonitor | 1856 |

### Basis

By `servingSizeUnit`: g, GRM or GM is per 100 g, ml or MLT per 100 ml. Any other unit (MG, IU, MC) says nothing about the basis, so those records are counted under (none) and need a rule in phase A.

| Nutrients per | Records |
| --- | --- |
| 100g | 338764 |
| 100ml | 51970 |
| (none) | 5081 |

Serving units as given, US records:

| `servingSizeUnit` | Records |
| --- | --- |
| g | 266236 |
| GRM | 105312 |
| ml | 40337 |
| MLT | 15412 |
| MG | 3549 |
| IU | 1845 |
| GM | 177 |
| MC | 26 |

### Ingredients

393625 kept records (99.4%) have `ingredients`: 113.4 MB of text in all, 288 bytes on average, a median of 205 and a longest of 5844.

### Size

Each kept record encoded as a draft OFL food (`kind: branded`, `gtin:` ID, sentence-cased name, brand, nutrients per 100 g or 100 ml with the worked-out keys, the label's serving and per-serving values, category, `fdc_id`, provenance) and gzipped at the default level. Phase A's records will differ a little.

| | JSONL | Gzipped |
| --- | --- | --- |
| With ingredients | 591 MB | 84 MB |
| Without ingredients | 469 MB | 53 MB |

## Field coverage

Top-level fields of every record read, and how many give a non-empty value:

| Field | Records | Share |
| --- | --- | --- |
| `dataSource` | 434001 | 100.0% |
| `dataType` | 434001 | 100.0% |
| `description` | 434001 | 100.0% |
| `fdcId` | 434001 | 100.0% |
| `foodClass` | 434001 | 100.0% |
| `foodUpdateLog` | 434001 | 100.0% |
| `gtinUpc` | 434001 | 100.0% |
| `marketCountry` | 434001 | 100.0% |
| `publicationDate` | 434001 | 100.0% |
| `servingSize` | 434001 | 100.0% |
| `servingSizeUnit` | 434001 | 100.0% |
| `tradeChannels` | 434001 | 100.0% |
| `foodNutrients` | 434000 | 100.0% |
| `brandedFoodCategory` | 433877 | 100.0% |
| `labelNutrients` | 433713 | 99.9% |
| `ingredients` | 431653 | 99.5% |
| `availableDate` | 431606 | 99.4% |
| `modifiedDate` | 431606 | 99.4% |
| `brandOwner` | 427458 | 98.5% |
| `householdServingFullText` | 421299 | 97.1% |
| `brandName` | 414688 | 95.6% |
| `packageWeight` | 387543 | 89.3% |
| `subbrandName` | 42160 | 9.7% |
| `gpcClassCode` | 21622 | 5.0% |
| `preparationStateCode` | 19633 | 4.5% |
| `shortDescription` | 19625 | 4.5% |
| `foodAttributes` | 18808 | 4.3% |
| `caffeineStatement` | 2833 | 0.7% |
| `discontinuedDate` | 2012 | 0.5% |
| `materialCode` | 50 | 0.0% |
| `footnote` | 5 | 0.0% |
| `microbes` | 2 | 0.0% |

`labelNutrients` keys, US records:

| Key | Records |
| --- | --- |
| `protein` | 428828 |
| `fat` | 428799 |
| `calories` | 427624 |
| `sodium` | 427423 |
| `carbohydrates` | 426587 |
| `sugars` | 406214 |
| `saturatedFat` | 374320 |
| `cholesterol` | 366230 |
| `fiber` | 362899 |
| `transFat` | 358390 |
| `iron` | 357573 |
| `calcium` | 356832 |
| `potassium` | 211228 |
| `addedSugar` | 140936 |
| `vitaminD` | 67050 |

Vocabulary keys of the kept records, read by `nutrients.json`'s FoodData Central numbers as generic foods are (with `energy_kj`, `salt_g` and `carbohydrate_available_g` worked out by section 4 of the spec):

| Key | Records | Share |
| --- | --- | --- |
| `energy_kcal` | 395815 | 100.0% |
| `energy_kj` | 395815 | 100.0% |
| `protein_g` | 393777 | 99.5% |
| `fat_g` | 394086 | 99.6% |
| `saturated_fat_g` | 346257 | 87.5% |
| `monounsaturated_fat_g` | 46110 | 11.6% |
| `polyunsaturated_fat_g` | 46023 | 11.6% |
| `trans_fat_g` | 331217 | 83.7% |
| `ala_g` | 4 | 0.0% |
| `cholesterol_mg` | 339891 | 85.9% |
| `carbohydrate_g` | 392708 | 99.2% |
| `carbohydrate_available_g` | 332414 | 84.0% |
| `fiber_g` | 335829 | 84.8% |
| `sugars_g` | 374638 | 94.6% |
| `added_sugars_g` | 131655 | 33.3% |
| `starch_g` | 1 | 0.0% |
| `polyols_g` | 3172 | 0.8% |
| `sodium_mg` | 393578 | 99.4% |
| `salt_g` | 393578 | 99.4% |
| `potassium_mg` | 195473 | 49.4% |
| `calcium_mg` | 331305 | 83.7% |
| `iron_mg` | 331926 | 83.9% |
| `magnesium_mg` | 11450 | 2.9% |
| `phosphorus_mg` | 11093 | 2.8% |
| `zinc_mg` | 9579 | 2.4% |
| `copper_mg` | 4238 | 1.1% |
| `manganese_mg` | 4283 | 1.1% |
| `selenium_ug` | 2648 | 0.7% |
| `iodine_ug` | 1663 | 0.4% |
| `chromium_ug` | 785 | 0.2% |
| `molybdenum_ug` | 783 | 0.2% |
| `chloride_mg` | 700 | 0.2% |
| `vitamin_a_rae_ug` | 24449 | 6.2% |
| `vitamin_c_mg` | 180188 | 45.5% |
| `vitamin_d_ug` | 63412 | 16.0% |
| `vitamin_e_mg` | 288 | 0.1% |
| `vitamin_k_ug` | 2584 | 0.7% |
| `thiamin_mg` | 21020 | 5.3% |
| `riboflavin_mg` | 19917 | 5.0% |
| `niacin_mg` | 22599 | 5.7% |
| `pantothenic_acid_mg` | 5418 | 1.4% |
| `vitamin_b6_mg` | 14206 | 3.6% |
| `biotin_ug` | 1166 | 0.3% |
| `folate_dfe_ug` | 446 | 0.1% |
| `folic_acid_ug` | 8892 | 2.2% |
| `vitamin_b12_ug` | 10776 | 2.7% |
| `choline_mg` | 241 | 0.1% |
| `caffeine_mg` | 96 | 0.0% |
| `alcohol_g` | 3 | 0.0% |
| `water_g` | 871 | 0.2% |

No kept record gives `epa_g`, `dha_g`.
