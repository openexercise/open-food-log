# Catalogue report

What `ofl catalogue` did with the FoodData Central downloads, retrieved 2026-11-12.

| Source | Read | Kept | Same name as an earlier source | Branded, left out | No nutrients, left out | No energy, left out | Kept without portions |
| --- | --- | --- | --- | --- | --- | --- | --- |
| USDA FoodData Central, Foundation Foods (2026-10-29) | 424 | 373 | 1 | 0 | 0 | 50 | 95 |
| USDA FoodData Central, FNDDS survey foods (2024-10-31) | 5432 | 5410 | 21 | 0 | 1 | 0 | 0 |
| USDA FoodData Central, SR Legacy (2018-04) | 7793 | 6659 | 180 | 954 | 0 | 0 | 231 |

12442 foods in `foods.jsonl`, 1680 with aliases: 1680 from the names' own shape, 0 suggested by a model (listed in each record's `provenance.generated`).

## Densities

1352 foods are liquids by their FoodData Central category and name; 1283 have a `density_g_per_ml`, from a measured specific gravity or one of their own volume portions (each record's `provenance.sources` says which):

| Source | With a density |
| --- | --- |
| USDA FoodData Central, Foundation Foods (2026-10-29) | 5 |
| USDA FoodData Central, FNDDS survey foods (2024-10-31) | 710 |
| USDA FoodData Central, SR Legacy (2018-04) | 568 |

69 liquids have no density, and why:

- Almond milk, unsweetened, plain, refrigerated: no portion in ml, fl oz, cups or spoons
- Almond milk, unsweetened, plain, shelf stable: no portion in ml, fl oz, cups or spoons
- Apple juice, with added vitamin C, from concentrate, shelf stable: no portion in ml, fl oz, cups or spoons
- Babyfood, juice treats, fruit medley, toddler: no portion in ml, fl oz, cups or spoons
- Babyfood, juice, apple, with calcium: no portion in ml, fl oz, cups or spoons
- Beverages, Energy Drink, Monster, fortified with vitamins C, B2, B3, B6, B12: no portion in ml, fl oz, cups or spoons
- Beverages, Malt liquor beverage: no portion in ml, fl oz, cups or spoons
- Beverages, Meal supplement drink, canned, peanut flavor: portion "1 cup", 158 g in 236.588 ml is 0.6678 g/ml, outside 0.85 to 1.45
- Beverages, Vegetable and fruit juice drink, reduced calorie, with low-calorie sweetener, added vitamin C: no portion in ml, fl oz, cups or spoons
- Beverages, Water with added vitamins and minerals, bottles, sweetened, assorted fruit flavors: no portion in ml, fl oz, cups or spoons
- Beverages, carbonated, cola, fast-food cola: no portion in ml, fl oz, cups or spoons
- Beverages, carbonated, reduced sugar, cola, contains caffeine and sweeteners: portion "1 can (8 fl oz)", 355 g in 236.588 ml is 1.5005 g/ml, outside 0.85 to 1.45
- Beverages, chocolate malt powder, prepared with 1% milk, fortified: no portion in ml, fl oz, cups or spoons
- Beverages, chocolate malt, powder, prepared with fat free milk: no portion in ml, fl oz, cups or spoons
- Beverages, chocolate-flavor beverage mix for milk, powder, with added nutrients, prepared with whole milk: no portion in ml, fl oz, cups or spoons
- Beverages, dairy drink mix, chocolate, reduced calorie, with aspartame, powder, prepared with water and ice: no portion in ml, fl oz, cups or spoons
- Beverages, rice milk, unsweetened: no portion in ml, fl oz, cups or spoons
- Beverages, shake, fast food, strawberry: portion "1 fl oz", 23.5 g in 29.574 ml is 0.7946 g/ml, outside 0.85 to 1.45
- Beverages, tea, Oolong, brewed: no portion in ml, fl oz, cups or spoons
- Beverages, vegetable and fruit juice blend, 100% juice, with added vitamins A, C, E: no portion in ml, fl oz, cups or spoons
- Buttermilk, low fat: no portion in ml, fl oz, cups or spoons
- Cranberry juice, not fortified, from concentrate, shelf stable: no portion in ml, fl oz, cups or spoons
- Cream, half and half, lowfat: no portion in ml, fl oz, cups or spoons
- Cream, heavy: no portion in ml, fl oz, cups or spoons
- Gelatin shot, alcoholic: no portion in ml, fl oz, cups or spoons
- Grape juice, purple, with added vitamin C, from concentrate, shelf stable: no portion in ml, fl oz, cups or spoons
- Grape juice, white, with added vitamin C, from concentrate, shelf stable: no portion in ml, fl oz, cups or spoons
- Grapefruit juice, red, not fortified, not from concentrate, refrigerated: no portion in ml, fl oz, cups or spoons
- Ice cream soda, chocolate: portion "1 soda (10 fl oz)", 240 g in 295.735 ml is 0.8115 g/ml, outside 0.85 to 1.45
- Ice cream soda, flavors other than chocolate: portion "1 soda (10 fl oz)", 240 g in 295.735 ml is 0.8115 g/ml, outside 0.85 to 1.45
- Milk, chocolate, lowfat, reduced sugar: no portion in ml, fl oz, cups or spoons
- Oat milk, unsweetened, plain, refrigerated: no portion in ml, fl oz, cups or spoons
- Oil, coconut: no portion in ml, fl oz, cups or spoons
- Orange juice, no pulp, not fortified, from concentrate, refrigerated: no portion in ml, fl oz, cups or spoons
- Orange juice, no pulp, not fortified, not from concentrate, refrigerated: no portion in ml, fl oz, cups or spoons
- Raspberry juice concentrate: no portion in ml, fl oz, cups or spoons
- Shake, fast food, vanilla: portion "1 large McDonald's shake (22 fl oz)", 458 g in 650.618 ml is 0.7039 g/ml, outside 0.85 to 1.45
- Soup, bean with bacon, condensed, single brand: no portion in ml, fl oz, cups or spoons
- Soup, beef mushroom, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, broccoli cheese, canned, condensed, commercial: no portion in ml, fl oz, cups or spoons
- Soup, chicken broth, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, chicken corn chowder, chunky, ready-to-serve, single brand: no portion in ml, fl oz, cups or spoons
- Soup, chicken gumbo, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, chicken noodle, canned, prepared with equal volume water: no portion in ml, fl oz, cups or spoons
- Soup, chicken with rice, canned, prepared with equal volume water: no portion in ml, fl oz, cups or spoons
- Soup, clam chowder, manhattan, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, cream of asparagus, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, cream of chicken, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, minestrone, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, mushroom barley, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, mushroom with beef stock, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, onion, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, oyster stew, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, pea, split with ham, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, tomato bisque, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, tomato rice, canned, condensed: no portion in ml, fl oz, cups or spoons
- Soup, tomato, canned, condensed, reduced sodium: no portion in ml, fl oz, cups or spoons
- Soup, tomato, canned, condensed: portion "1 cup", 148 g in 236.588 ml is 0.6256 g/ml, outside 0.85 to 1.45
- Soup, tomato, dry, mix, prepared with water: no portion in ml, fl oz, cups or spoons
- Soup, vegetable beef, microwavable, ready-to-serve, single brand: no portion in ml, fl oz, cups or spoons
- Soupy rice with chicken, Puerto Rican style: no portion in ml, fl oz, cups or spoons
- Soy milk, sweetened, plain, refrigerated: no portion in ml, fl oz, cups or spoons
- Soy milk, unsweetened, plain, shelf stable: no portion in ml, fl oz, cups or spoons
- Syrup, Cane: no portion in ml, fl oz, cups or spoons
- Syrup, fruit flavored: no portion in ml, fl oz, cups or spoons
- Syrup, maple, Canadian: no portion in ml, fl oz, cups or spoons
- Syrups, table blends, pancake, with butter: no portion in ml, fl oz, cups or spoons
- Tomato juice, with added ingredients, from concentrate, shelf stable: no portion in ml, fl oz, cups or spoons
- Water, with corn syrup and/or sugar and low calorie sweetener, fruit flavored: no portion in ml, fl oz, cups or spoons

## Nutrients

How many foods give each key of the vocabulary, by the source whose numbers the food uses. "Worked out" counts values OFL v0 section 4 defines from others (`energy_kj` from kcal × 4.184 where FoodData Central gives no kJ, `salt_g` as sodium × 2.5, `carbohydrate_available_g` as carbohydrate − fibre); each such record names them in `provenance.sources`. A food without a key doesn't have it: never zero.

| Key | FDC number | Foods | Worked out | USDA FoodData Central, Foundation Foods (2026-10-29) | USDA FoodData Central, FNDDS survey foods (2024-10-31) | USDA FoodData Central, SR Legacy (2018-04) |
| --- | --- | --- | --- | --- | --- | --- |
| `energy_kcal` | 208 | 12442 | 0 | 373 | 5410 | 6659 |
| `energy_kj` | 268 | 12442 | 5688 | 373 | 5410 | 6659 |
| `protein_g` | 203 | 12442 | 0 | 373 | 5410 | 6659 |
| `fat_g` | 204 | 12442 | 0 | 373 | 5410 | 6659 |
| `saturated_fat_g` | 606 | 11930 | 0 | 161 | 5410 | 6359 |
| `monounsaturated_fat_g` | 645 | 11875 | 0 | 161 | 5410 | 6304 |
| `polyunsaturated_fat_g` | 646 | 11863 | 0 | 146 | 5410 | 6307 |
| `trans_fat_g` | 605 | 3532 | 0 | 128 | 0 | 3404 |
| `ala_g` | 851 | 1711 | 0 | 57 | 0 | 1654 |
| `epa_g` | 629 | 10518 | 0 | 58 | 5410 | 5050 |
| `dha_g` | 621 | 10479 | 0 | 58 | 5410 | 5011 |
| `cholesterol_mg` | 601 | 11843 | 0 | 87 | 5410 | 6346 |
| `carbohydrate_g` | 205 | 12432 | 0 | 363 | 5410 | 6659 |
| `carbohydrate_available_g` | none | 11807 | 11807 | 217 | 5410 | 6180 |
| `fiber_g` | 291 | 11807 | 0 | 217 | 5410 | 6180 |
| `sugars_g` | 269 | 10509 | 0 | 140 | 5410 | 4959 |
| `added_sugars_g` | 539 | 0 | 0 | 0 | 0 | 0 |
| `starch_g` | 209 | 992 | 0 | 112 | 0 | 880 |
| `polyols_g` | 299 | 0 | 0 | 0 | 0 | 0 |
| `sodium_mg` | 307 | 12341 | 0 | 355 | 5410 | 6576 |
| `salt_g` | 375 | 12341 | 12341 | 355 | 5410 | 6576 |
| `potassium_mg` | 306 | 12266 | 0 | 356 | 5410 | 6500 |
| `calcium_mg` | 301 | 12364 | 0 | 356 | 5410 | 6598 |
| `iron_mg` | 303 | 12372 | 0 | 356 | 5410 | 6606 |
| `magnesium_mg` | 304 | 12219 | 0 | 356 | 5410 | 6453 |
| `phosphorus_mg` | 305 | 12273 | 0 | 356 | 5410 | 6507 |
| `zinc_mg` | 309 | 12206 | 0 | 356 | 5410 | 6440 |
| `copper_mg` | 312 | 12170 | 0 | 356 | 5410 | 6404 |
| `manganese_mg` | 315 | 6115 | 0 | 356 | 0 | 5759 |
| `selenium_ug` | 317 | 11640 | 0 | 152 | 5410 | 6078 |
| `iodine_ug` | 314 | 44 | 0 | 44 | 0 | 0 |
| `chromium_ug` | 310 | 0 | 0 | 0 | 0 | 0 |
| `molybdenum_ug` | 316 | 51 | 0 | 51 | 0 | 0 |
| `chloride_mg` | 302 | 0 | 0 | 0 | 0 | 0 |
| `vitamin_a_rae_ug` | 320 | 11623 | 0 | 51 | 5410 | 6162 |
| `vitamin_c_mg` | 401 | 11858 | 0 | 103 | 5410 | 6345 |
| `vitamin_d_ug` | 328 | 10069 | 0 | 74 | 5410 | 4585 |
| `vitamin_e_mg` | 323 | 10238 | 0 | 75 | 5410 | 4753 |
| `vitamin_k_ug` | 430 | 9859 | 0 | 81 | 5410 | 4368 |
| `thiamin_mg` | 404 | 12098 | 0 | 207 | 5410 | 6481 |
| `riboflavin_mg` | 405 | 12071 | 0 | 183 | 5410 | 6478 |
| `niacin_mg` | 406 | 12100 | 0 | 213 | 5410 | 6477 |
| `pantothenic_acid_mg` | 410 | 5793 | 0 | 89 | 0 | 5704 |
| `vitamin_b6_mg` | 415 | 12013 | 0 | 241 | 5410 | 6362 |
| `biotin_ug` | 416 | 110 | 0 | 110 | 0 | 0 |
| `folate_dfe_ug` | 435 | 11298 | 0 | 0 | 5410 | 5888 |
| `folic_acid_ug` | 431 | 11317 | 0 | 0 | 5410 | 5907 |
| `vitamin_b12_ug` | 418 | 11728 | 0 | 74 | 5410 | 6244 |
| `choline_mg` | 421 | 9502 | 0 | 37 | 5410 | 4055 |
| `caffeine_mg` | 262 | 10075 | 0 | 0 | 5410 | 4665 |
| `alcohol_g` | 221 | 10268 | 0 | 0 | 5410 | 4858 |
| `water_g` | 255 | 12442 | 0 | 373 | 5410 | 6659 |

### Left out, and why

Every key is read by its FoodData Central number in the vocabulary's unit. Where a food lacks one, FoodData Central doesn't report it for that food. Beyond that:

- `ala_g`: FNDDS survey foods give only 18:3 undifferentiated (619), which counts isomers other than ALA, so survey foods have none.
- `carbohydrate_available_g`: Worked out only where both carbohydrate and fibre are given, and not where fibre exceeds the total.
- `added_sugars_g`: None of the three downloads reports added sugars (539); FoodData Central gives them for branded foods only.
- `starch_g`: Only Foundation Foods and some SR Legacy foods measure starch (209).
- `polyols_g`: None of the three downloads reports sugar alcohols (299).
- `iodine_ug`: Only Foundation Foods measure iodine (314).
- `chromium_ug`: None of the three downloads reports chromium (310).
- `molybdenum_ug`: Only Foundation Foods measure molybdenum (316).
- `chloride_mg`: None of the three downloads reports chlorine (302).
- `vitamin_a_rae_ug`: Read as RAE (320) only; SR Legacy's vitamin A in IU (318) doesn't say the form, so isn't converted.
- `vitamin_d_ug`: Read in µg (328) only; FoodData Central's IU value (324) comes with it wherever it's given, so no IU is converted.
- `biotin_ug`: Only Foundation Foods measure biotin (416).
- `folate_dfe_ug`: Foundation Foods give only total folate (417), which equals DFE only when a food has no folic acid, so they have none.
- `folic_acid_ug`: Foundation Foods don't report folic acid (431).

## Attributes

12442 foods have a `category`: FoodData Central's food category (SR Legacy, Foundation Foods) or WWEIA category with its number (survey foods), scheme `fdc`, basis `inferred`.

No food has `allergens`, `diet_flags`, `ingredients_text`, `glycemic_index` or `glycemic_load`. FoodData Central's generic foods state none of them, and the catalogue has no other CC0 source that does; working them out from names would be guessing.
