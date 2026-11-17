# Branded foods report

What `ofl catalogue branded-report` found in USDA FoodData Central's Global Branded Food Products Database, the JSON download of 2026-10-29 (`FoodData_Central_branded_food_json_2026-10-29.json`), measured against the filtering rules planned for a branded catalogue. It writes no records: this is phase 0 of the plan, and nothing here is in `foods.jsonl`.

## Each rule, in order

| Step | Records | Dropped |
| --- | --- | --- |
| Read | 434001 | |
| 1. US market only (`United States` or `US`, kept as `US`) | 432894 | 1107 |
| 2. A valid GTIN (up to 14 digits, zero-padded, check digit) | 422150 | 10744 |
| 3. One per GTIN, newest kept | 422149 | 1 |
| 4. Not discontinued | 420139 | 2010 |
| 5. Plausible numbers | 396916 | 23223 |
| **Kept** | **396916** | |

### 1. Markets

| Market | Records |
| --- | --- |
| United States | 421218 |
| US | 11676 |
| New Zealand | 1107 |

The download names the market both ways; a branded record keeps it as `US`, the shorter form (in OFL, `label.region` is `us`).

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
| check digit | 10725 |
| not digits | 16 |
| over 14 digits | 3 |

A code shorter than 14 digits is zero-padded on the left before the check digit is checked, as a scanner's 8, 12 and 13 are. 1162 US records with a valid GTIN gave it in a length a scanner doesn't (by length: 11 digits 1129; 10 digits 29; 6 digits 2; 7 digits 1; 9 digits 1; an 11-digit code is a UPC-A that lost its leading zero), and padding gives them the 14 digits a scan of the package does.

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
| no energy in kcal | 4909 |
| serving unit not g or ml, and the label's calories don't confirm the serving as either | 0 |
| protein + fat + carbohydrate over 100 g per 100 | 5752 |
| energy over 900 kcal per 100 | 431 |
| energy more than 20% and 40 kcal from 4·protein + 4·carbohydrate + 9·fat (+ 7·alcohol) | 12066 |

3 records with no energy in kcal give kilojoules (268) instead. 5903 records with energy lack protein, fat or carbohydrate, so the Atwater check can't be made on them; they're kept.

The Atwater check at other tolerances, over every remaining record with energy and the three macros, before the other plausibility rules (a record is off when it's past both bounds):

| Tolerance | Off |
| --- | --- |
| 10% and 20 kcal | 36134 |
| 20% and 40 kcal (the rule) | 13265 |
| 30% and 50 kcal | 9296 |
| 50% and 80 kcal | 6414 |

## What's kept

### Age

Year of each kept record's `availableDate` (else `modifiedDate`, else `publicationDate`):

| Year | Records |
| --- | --- |
| 2014 | 5 |
| 2016 | 201 |
| 2017 | 59411 |
| 2018 | 70751 |
| 2019 | 26484 |
| 2020 | 55075 |
| 2021 | 47733 |
| 2022 | 32962 |
| 2023 | 89128 |
| 2024 | 1550 |
| 2025 | 3901 |
| 2026 | 9715 |

255905 kept records (64.5%) are over 5 years old (dated before 2021-10-29). 0 have no readable date.

Kept records by `dataSource`:

| Source | Records |
| --- | --- |
| LI | 372112 |
| GDSN | 22890 |
| Euromonitor | 1914 |

### Basis

A serving in g, GRM or GM means nutrients per 100 g; ml or MLT per 100 ml. Some records give their serving in MG, IU or MC, but the number is the grams or millilitres the nutrients are per 100 of: the label's calories per serving equal energy per 100 × serving / 100 (a chocolate at 605 kcal per 100 with a serving of 38 "MG" labels 230 kcal). Where the two agree within 10% or 5 kcal, the serving is read as millilitres when the household serving or package names fluid ounces, millilitres or litres and the package names no weight, else as grams. Where they don't, the record is dropped (step 5).

| Nutrients per | Records |
| --- | --- |
| 100g | 344043 |
| 100ml | 52873 |

Read from the label's calories:

| Unit given, read as | Records |
| --- | --- |
| MG as 100g | 2943 |
| IU as 100g | 1370 |
| MG as 100ml | 408 |
| IU as 100ml | 341 |
| MC as 100g | 19 |
| MC as 100ml | 6 |

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

394726 kept records (99.4%) have `ingredients`: 113.6 MB of text in all, 287 bytes on average, a median of 204 and a longest of 5844.

### Size

Each kept record encoded as `ofl catalogue branded` writes it (an OFL food: `kind: branded`, `gtin:` ID, sentence-cased name, brand, nutrients per 100 g or 100 ml as stated, the label's serving and per-serving values, category, ingredients, `fdc_id`, provenance) and gzipped at the default level. The release asset is gzipped at the best level, so it is a little smaller.

| | JSONL | Gzipped |
| --- | --- | --- |
| With ingredients | 648 MB | 89 MB |
| Without ingredients | 509 MB | 59 MB |

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
| `energy_kcal` | 396916 | 100.0% |
| `energy_kj` | 396916 | 100.0% |
| `protein_g` | 394862 | 99.5% |
| `fat_g` | 395173 | 99.6% |
| `saturated_fat_g` | 347207 | 87.5% |
| `monounsaturated_fat_g` | 46232 | 11.6% |
| `polyunsaturated_fat_g` | 46144 | 11.6% |
| `trans_fat_g` | 332101 | 83.7% |
| `ala_g` | 4 | 0.0% |
| `cholesterol_mg` | 340786 | 85.9% |
| `carbohydrate_g` | 393795 | 99.2% |
| `carbohydrate_available_g` | 333324 | 84.0% |
| `fiber_g` | 336747 | 84.8% |
| `sugars_g` | 375655 | 94.6% |
| `added_sugars_g` | 131890 | 33.2% |
| `starch_g` | 1 | 0.0% |
| `polyols_g` | 3177 | 0.8% |
| `sodium_mg` | 394664 | 99.4% |
| `salt_g` | 394664 | 99.4% |
| `potassium_mg` | 195994 | 49.4% |
| `calcium_mg` | 332172 | 83.7% |
| `iron_mg` | 332790 | 83.8% |
| `magnesium_mg` | 11473 | 2.9% |
| `phosphorus_mg` | 11122 | 2.8% |
| `zinc_mg` | 9595 | 2.4% |
| `copper_mg` | 4249 | 1.1% |
| `manganese_mg` | 4292 | 1.1% |
| `selenium_ug` | 2654 | 0.7% |
| `iodine_ug` | 1664 | 0.4% |
| `chromium_ug` | 785 | 0.2% |
| `molybdenum_ug` | 783 | 0.2% |
| `chloride_mg` | 700 | 0.2% |
| `vitamin_a_rae_ug` | 24464 | 6.2% |
| `vitamin_c_mg` | 180651 | 45.5% |
| `vitamin_d_ug` | 63434 | 16.0% |
| `vitamin_e_mg` | 289 | 0.1% |
| `vitamin_k_ug` | 2590 | 0.7% |
| `thiamin_mg` | 21084 | 5.3% |
| `riboflavin_mg` | 19979 | 5.0% |
| `niacin_mg` | 22674 | 5.7% |
| `pantothenic_acid_mg` | 5429 | 1.4% |
| `vitamin_b6_mg` | 14250 | 3.6% |
| `biotin_ug` | 1167 | 0.3% |
| `folate_dfe_ug` | 447 | 0.1% |
| `folic_acid_ug` | 8916 | 2.2% |
| `vitamin_b12_ug` | 10813 | 2.7% |
| `choline_mg` | 242 | 0.1% |
| `caffeine_mg` | 96 | 0.0% |
| `alcohol_g` | 3 | 0.0% |
| `water_g` | 871 | 0.2% |

No kept record gives `epa_g`, `dha_g`.
