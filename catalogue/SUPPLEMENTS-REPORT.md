# Supplements report

What `ofl catalogue supplements-measure` found in the NIH Office of Dietary Supplements' Dietary Supplement Label Database (DSLD), the bulk download of every label as JSON. It writes no records: this is phase 0 of a supplements plan, the same measuring USDA's branded foods had first ([BRANDED-REPORT.md](BRANDED-REPORT.md)), and nothing here is in `foods.jsonl` or `branded-foods.jsonl.gz`.

| | |
| --- | --- |
| File | `DSLD-full-database-JSON.zip` |
| Size | 584344994 bytes |
| SHA-256 | `cac90002da672b891086780c0237f412eb7a402f0b730d178a0e53b1e9a2e9f1` |
| From | https://api.ods.od.nih.gov/dsld/s3/data/DSLD-full-database-JSON.zip |
| Retrieved | 2026-11-21 |
| DSLD's own date of download (its `ReadMe.csv`) | 2026-01-10T18:08:23-05:00 |
| DSLD release (its `ReadMe.csv`) | Version 9.4.0 \| January 2026 |
| Cite as | *US Department of Health and Human Services, National Institutes of Health, Office of Dietary Supplements. Dietary Supplement Label Database (DSLD). [Internet]. Available from: https://dsld.od.nih.gov/dsld/.* |
| Licence | CC0 1.0. NIH's API guide: "DSLD data are in the public domain and they are not copyrighted. They are published under CC0 1.0 Universal (CC0 1.0)." |

Written by `ofl catalogue supplements-measure --retrieved 2026-11-21 --branded-gtins <a file of the branded catalogue's GTINs, one a line> DSLD-full-database-JSON.zip`, which gives the same report from the same file.

The zip holds one JSON file per label, named by its DSLD ID; the command reads them from the zip in ID order and never unpacks it. **The candidate set** below is every label that is on the market (`offMarket` 0) and whose `upcSku` reads as a valid GTIN: what a first build could be made from.

## 1. Labels

| | Labels |
| --- | --- |
| Label files read | 214780 |
| Didn't parse | 0 |
| On the market (`offMarket` 0) | 121959 |
| Off the market (`offMarket` 1) | 92821 |

By the year of `entryDate`, when DSLD took the label in (not when the product was sold):

| Year | Labels | On the market | Off the market | Candidate set |
| --- | --- | --- | --- | --- |
| 2011 | 3422 | 1162 | 2260 | 972 |
| 2012 | 11938 | 3647 | 8291 | 2616 |
| 2013 | 11946 | 5180 | 6766 | 3314 |
| 2014 | 11938 | 3500 | 8438 | 2945 |
| 2015 | 11990 | 3188 | 8802 | 2763 |
| 2016 | 11980 | 3688 | 8292 | 3235 |
| 2017 | 12129 | 5273 | 6856 | 4560 |
| 2018 | 10985 | 4401 | 6584 | 3827 |
| 2019 | 14147 | 5787 | 8360 | 4724 |
| 2020 | 23435 | 13152 | 10283 | 10731 |
| 2021 | 18035 | 11658 | 6377 | 7198 |
| 2022 | 19201 | 14619 | 4582 | 10257 |
| 2023 | 22120 | 17769 | 4351 | 14385 |
| 2024 | 18000 | 15914 | 2086 | 11101 |
| 2025 | 13514 | 13021 | 493 | 9488 |

0 labels have no `brandName` and 0 no `fullName` (0 and 0 in the candidate set).

## 2. Barcodes

`upcSku` is the barcode as the label prints it, as free text ("3 0005-4451-76 7"). Spaces and dashes are dropped; what's left counts as a GTIN when it is 8, 12, 13 or 14 digits (EAN-8, UPC-A, EAN-13, GTIN-14) with a valid check digit, held as 14 digits the way branded foods' are (`normalizeGTIN`). Unlike branded foods, shorter runs of digits aren't zero-padded: the field also holds product codes, and one in ten of those would pass a check digit by chance.

| | Labels | Share of labels |
| --- | --- | --- |
| A non-empty `upcSku` | 186718 | 86.9% |
| A valid GTIN | 169864 | 79.1% |
| On the market with a valid GTIN (**the candidate set**) | 92116 | 42.9% |

| | GTINs |
| --- | --- |
| Distinct GTINs | 97789 |
| Distinct GTINs with a label on the market (the candidate set's) | 85398 |

Why a non-empty `upcSku` isn't a GTIN:

| Reason | Labels |
| --- | --- |
| not digits | 10995 |
| not 8, 12, 13 or 14 digits | 4839 |
| check digit | 1020 |

The shapes of those (a digit shown as 9, a letter as a), the 25 most common:

| Reason | Shape | Labels | Examples |
| --- | --- | --- | --- |
| not 8, 12, 13 or 14 digits | `9 999999 99` | 1689 | `0 840253 60`, `0 840253 60`, `0 840253 60` |
| not digits | `a999aaaaaa` | 1386 | `X000HBFWSN`, `X000HBFNCX`, `X000CQHDSP` |
| not 8, 12, 13 or 14 digits | `999999` | 785 | `800017`, `304796`, `000726` |
| not 8, 12, 13 or 14 digits | `9 99999 99999` | 780 | `7 37190 00170`, `8 52113 00214`, `7 18122 65798` |
| not digits | `a999a9aaaa` | 594 | `X000H3MGGH`, `X000G7VQQZ`, `X000L4SXKZ` |
| not digits | `a999aaaaa9` | 576 | `X000LZEWV3`, `X000TPQQC3`, `X000XJZIX3` |
| not digits | `a999aaa9aa` | 565 | `X000GOU6XR`, `X000GOU5EH`, `X000HAD9GL` |
| check digit | `9 99999 99999 9` | 555 | `0 48107 06002 9`, `8 11184 00040 8`, `0 35046 06353 9` |
| not digits | `a999aaaa9a` | 504 | `X000HBFQ7F`, `X000NHFQ7D`, `X000NHFQ7D` |
| not digits | `a999aa9aaa` | 498 | `X000GP5IED`, `X000LU3IUJ`, `X000LU3IVX` |
| not 8, 12, 13 or 14 digits | `999999 99999` | 492 | `883196 12681`, `883196 14390`, `883196 13680` |
| not 8, 12, 13 or 14 digits | `99-9` | 475 | `39-2`, `39-3`, `39-4` |
| not digits | `a9999aaaaa` | 458 | `X0015SWTTR`, `X0015YJTKD`, `X0015SVAWT` |
| not digits | `a999` | 442 | `M086`, `M099`, `M003` |
| not digits | `9999a` | 311 | `3100s`, `3340s`, `3341s` |
| check digit | `9999999999999` | 228 | `5113339163991`, `5113339163981`, `5113339163981` |
| not digits | `9999aa` | 221 | `1565US`, `3007US`, `0055US` |
| not digits | `a999a9aa9a` | 209 | `X000H2QP8X`, `X000L4SZ8Z`, `X000L4PJ7Z` |
| not digits | `a999aaaa99` | 205 | `X000KMHT35`, `X001MFKL43`, `X000MGYT91` |
| not digits | `a999a9aaa9` | 204 | `X000H9LED7`, `X000G4CBK3`, `X000G4CBK3` |
| not digits | `a999a99aaa` | 202 | `X000S78JGD`, `X000T60FPL`, `X001B52QGZ` |
| not digits | `a999aa9aa9` | 198 | `X000LU3IR7`, `X000LU3IU9`, `X000LO5OH5` |
| not digits | `a999aaa9a9` | 195 | `X000HXT9X5`, `X000HZJ6F9`, `X000HXT9X5` |
| not digits | `a9999aaaa9` | 194 | `X0014CRGG5`, `X0016HSVU3`, `X0017OSSN5` |
| not digits | `a9999aaa9a` | 189 | `X0000UNF1R`, `X0013CLR6V`, `X0016YRM3X` |

Digits left once spaces and dashes go, where only digits are left:

| Digits | Labels |
| --- | --- |
| 12 | 162615 |
| 13 | 8053 |
| 9 | 1773 |
| 11 | 1380 |
| 6 | 854 |
| 3 | 494 |
| 8 | 201 |
| 4 | 130 |
| 10 | 122 |
| 5 | 73 |
| 14 | 15 |
| 2 | 7 |
| 7 | 4 |
| 16 | 2 |

139 of the 11-digit codes are a valid UPC-A once a leading zero is put back (a tenth would be by chance); the measure doesn't count them as GTINs.

Labels per GTIN:

| Labels carrying the GTIN | GTINs |
| --- | --- |
| 1 | 63174 |
| 2 | 17103 |
| 3–5 | 14873 |
| 6 or more | 2639 |

The most labels on one GTIN is 115. Of the 34615 GTINs with several labels:

| | GTINs | Share |
| --- | --- | --- |
| Exactly one label on the market | 24844 | 71.8% |
| Several on the market | 5739 | 16.6% |
| … whose on-market labels have different `fullName`s | 2443 | 7.1% |
| None on the market | 4032 | 11.6% |
| Labels with different `fullName`s | 12728 | 36.8% |
| Labels with different `brandName`s | 5601 | 16.2% |
| Labels with different `productType`s | 6487 | 18.7% |
| A label DSLD relates to another as "Flavor difference" | 3155 | 9.1% |
| … as "Package size difference, same product" | 14180 | 41.0% |
| … as "Image difference, same product" | 31588 | 91.3% |

The relation rows say a label of that GTIN has such a relation to some label, not that the related label shares the GTIN.

160 GTINs carry 10 labels or more. The eight with the most:

| GTIN | Labels | On the market | The first label | A label named otherwise |
| --- | --- | --- | --- | --- |
| 00054986008031 | 115 | 22 | Ashwagandha Tablets (Banyan Botanicals) | Healthy Hair (Banyan Botanicals) |
| 00631257560247 | 18 | 3 | Rapid Cleanse 1 (Renew Life) | RAPID Cleanse 2 (Renew Life) |
| 00693749006534 | 18 | 2 | A.M. (Thorne Research) | P.M. (Thorne Research) |
| 00763948084500 | 18 | 3 | Detox Activation Cleansing Formula (Enzymatic Therapy) | Fiber Fusion Plus Daily Cleansing Fiber (Enzymatic Therapy) |
| 00871149002169 | 18 | 2 | N.O.max ER (XYMOGEN EP) | ALAmax CR (XYMOGEN EP) |
| 00693749003663 | 16 | 2 | Amino Complex Lemon Flavor (Thorne Research) | Amino Complex Lemon Flavored (Thorne) |
| 00755929010523 | 16 | 5 | Fat Metabolism Factors (Michael's Naturopathic Programs) | Fiber & More (Michael's Naturopathic Programs) |
| 00763948072064 | 16 | 2 | AM Formula (Enzymatic Therapy) | PM Formula (Enzymatic Therapy) |

9087 labels (4.9% of those with an `upcSku`) give a code shaped like the ones Amazon prints on stock it holds for a seller (`X00` and seven letters or digits) where a barcode would be.

The candidate set's GTINs by GS1 prefix (the three digits after a GTIN-14's first, which name the GS1 organisation that issued the company's prefix, not where the product is made or sold):

| Prefix | GTINs |
| --- | --- |
| GS1 US (000–019, 030–039, 060–139) | 80586 |
| another GS1 organisation | 4651 |
| restricted, for use inside a company or shop (020–029, 040–049, 200–299) | 79 |
| an EAN-8 | 59 |
| coupons (050–059) | 23 |

## 3. Overlap with branded foods

Against the 396916 GTINs of USDA's branded foods as the catalogue publishes them (`branded-gtins.txt`):

| | GTINs | Also a branded food | Share |
| --- | --- | --- | --- |
| Every valid GTIN | 97789 | 450 | 0.5% |
| The candidate set's GTINs | 85398 | 384 | 0.4% |

Candidate labels whose GTIN is also a branded food, by product type:

| Product type | Labels |
| --- | --- |
| A1305 Amino acid/Protein | 184 |
| A1325 Other Combinations | 131 |
| A1306 Botanical | 28 |
| A1315 Multi-Vitamin and Mineral (MVM) | 15 |
| A1317 Botanical with Nutrients | 11 |
| A1326 Fiber and Other Nutrients | 8 |
| A1309 Non-Nutrient/Non-Botanical | 7 |
| A1310 Fat/Fatty Acid | 7 |
| A1302 Vitamin | 4 |
| A1299 Mineral | 1 |
| A1316 Single Vitamin and Mineral | 1 |

## 4. Products

By `productType` (a LanguaL code and its description):

| Product type | All labels | Candidate set |
| --- | --- | --- |
| A1325 Other Combinations | 76497 | 30524 |
| A1306 Botanical | 55666 | 27700 |
| A1309 Non-Nutrient/Non-Botanical | 25229 | 10171 |
| A1302 Vitamin | 12358 | 4918 |
| A1317 Botanical with Nutrients | 11783 | 4770 |
| A1305 Amino acid/Protein | 9363 | 4270 |
| A1310 Fat/Fatty Acid | 8544 | 3355 |
| A1299 Mineral | 8191 | 3214 |
| A1315 Multi-Vitamin and Mineral (MVM) | 3365 | 1502 |
| A1316 Single Vitamin and Mineral | 2160 | 995 |
| A1326 Fiber and Other Nutrients | 1624 | 697 |

By `physicalState`:

| Physical state | All labels | Candidate set |
| --- | --- | --- |
| E0159 Capsule | 79822 | 32340 |
| E0162 Powder | 36485 | 17017 |
| E0155 Tablet or Pill | 33717 | 12883 |
| E0165 Liquid | 32532 | 16757 |
| E0161 Softgel Capsule | 19924 | 7705 |
| E0172 Other (e.g. tea bag) | 6171 | 2433 |
| E0176 Gummy or Jelly | 4677 | 2438 |
| E0174 Lozenge | 994 | 406 |
| E0177 Unknown | 399 | 110 |
| E0164 Bar | 59 | 27 |

Labels DSLD relates to another label (`labelRelationships`), by the kind of relation:

| Relation | All labels | Candidate set |
| --- | --- | --- |
| Image difference, same product | 126541 | 37880 |
| Package size difference, same product | 65354 | 23596 |
| Flavor difference | 16820 | 8731 |
| Bundle set | 2811 | 752 |
| Alternate version | 831 | 299 |

The groups of people the labels give daily values for (`userGroups`), all labels:

| Group | Labels |
| --- | --- |
| Adults and children 4 or more years of age | 213192 |
| Children less than 4 years of age | 2509 |
| Pregnant and Lactating | 1359 |
| Infants | 289 |

## 5. Servings

| | All labels | Candidate set |
| --- | --- | --- |
| More than one serving size | 2212 | 1028 |
| A serving size given as a range (`minQuantity` below `maxQuantity`) | 32475 | 14803 |
| No serving size | 0 | 0 |
| `servingsPerContainer` present | 175778 | 79268 |

Serving units as written, one count per serving size: 638 different spellings in all, the 40 most common here.

| `servingSizes[].unit` | All labels | Candidate set |
| --- | --- | --- |
| Capsule(s) | 64297 | 25840 |
| Tablet(s) | 30822 | 11921 |
| Gram(s) | 26737 | 13518 |
| mL | 20547 | 10763 |
| Softgel(s) | 18134 | 6849 |
| Drop(s) | 6521 | 3749 |
| Vegetarian Capsule(s) | 6123 | 2370 |
| mg | 4648 | 259 |
| Teaspoon(s) | 3904 | 3283 |
| Gummy(ies) | 3466 | 1859 |
| Caplet(s) | 2847 | 1230 |
| tsp | 2591 | 877 |
| Veg Capsule(s) | 2407 | 841 |
| Veggie Capsule(s) | 2226 | 941 |
| Vegetable Capsule(s) | 2051 | 805 |
| Tbsp | 1555 | 468 |
| fl. Oz. | 1126 | 506 |
| Gummie(s) | 1021 | 456 |
| Chewable Tablet(s) | 982 | 415 |
| Lozenge(s) | 880 | 362 |
| Packet(s) | 820 | 239 |
| VegCap(s) | 696 | 436 |
| Vegan Capsule(s) | 570 | 321 |
| Tea Bag(s) | 517 | 94 |
| Scoop(s) | 499 | 162 |
| Softgel Capsule(s) | 494 | 236 |
| Milligram(s) | 481 | 9 |
| Pill(s) | 442 | 173 |
| Wafer(s) | 432 | 153 |
| Vegetarian Tablet(s) | 396 | 172 |
| Not Present | 282 | 100 |
| Spray(s) | 277 | 147 |
| Gummy Vitamin(s) | 257 | 136 |
| Chew(s) | 250 | 135 |
| Liquid Softgel(s) | 238 | 125 |
| Tablespoon(s) | 222 | 113 |
| Ounce(s) | 204 | 110 |
| Veggie Cap(s) | 193 | 165 |
| Soft Gel(s) | 191 | 82 |
| Dropper(s) | 176 | 78 |

## 6. Ingredient rows

The rows of the Supplement Facts panel (`ingredientRows`), and the rows nested under them (`nestedRows`, to any depth: the parts of a blend, the folic acid in a folate row, added sugars under total sugars). A row's amount and unit are read from its first `quantity`, the one for the label's first serving size.

| | All labels: top-level | nested | Candidate set: top-level | nested |
| --- | --- | --- | --- | --- |
| Rows | 1213813 | 725748 | 476446 | 287830 |
| … with an amount | 1148611 | 196908 | 449049 | 81705 |
| … with a stated zero | 17573 | 24895 | 7910 | 11589 |
| … with not present on the label (unit NP) | 47564 | 503933 | 19455 | 194528 |
| … with no quantity at all | 65 | 12 | 32 | 8 |

DSLD writes a row the label gives no amount for (an ingredient of a proprietary blend) with the unit `NP` and a quantity of 0.

Rows by `category`:

| Category | All labels: top-level | nested | Candidate set: top-level | nested |
| --- | --- | --- | --- | --- |
| botanical | 159557 | 301350 | 64189 | 112587 |
| vitamin | 325293 | 24316 | 119576 | 9053 |
| mineral | 270469 | 14298 | 105093 | 6433 |
| non-nutrient/non-botanical | 110137 | 72327 | 40914 | 30674 |
| blend | 88241 | 17196 | 36186 | 7492 |
| other | 63107 | 35223 | 26994 | 12318 |
| fat | 57070 | 35558 | 23934 | 15094 |
| sugar | 42672 | 34026 | 19803 | 17183 |
| amino acid | 29011 | 37409 | 12040 | 18083 |
| enzyme | 14079 | 42937 | 5209 | 15426 |
| bacteria | 6127 | 43218 | 2231 | 16178 |
| fatty acid | 8108 | 30637 | 2775 | 11231 |
| fiber | 7122 | 23099 | 2690 | 9710 |
| protein | 22152 | 7101 | 10320 | 3340 |
| animal part or source | 5642 | 3925 | 2306 | 1463 |
| hormone | 3805 | 412 | 1630 | 176 |
| complex carbohydrate | 718 | 2197 | 333 | 1075 |
| TBD | 503 | 519 | 223 | 314 |

The 60 most common `ingredientGroup` values, of 4986:

| Ingredient group | All labels: top-level | nested | Candidate set: top-level | nested |
| --- | --- | --- | --- | --- |
| Calories | 50311 | 12476 | 21512 | 4482 |
| Calcium | 38039 | 2429 | 15772 | 1014 |
| Carbohydrate | 37239 | 386 | 17050 | 138 |
| Vitamin C | 35939 | 958 | 13973 | 375 |
| Sugar | 2172 | 30287 | 947 | 15036 |
| Magnesium | 29045 | 2408 | 10943 | 1158 |
| Vitamin D | 26785 | 609 | 10464 | 274 |
| Zinc | 26271 | 872 | 9810 | 356 |
| Vitamin B12 | 26154 | 970 | 9894 | 303 |
| Sodium | 25945 | 779 | 12517 | 444 |
| Fat (unspecified) | 26106 | 101 | 11333 | 52 |
| Vitamin B6 | 24599 | 1262 | 9608 | 498 |
| Vitamin E | 22491 | 2746 | 7952 | 763 |
| Folate | 19138 | 5673 | 7314 | 2688 |
| Potassium | 22401 | 1255 | 9656 | 686 |
| Niacin | 22293 | 1241 | 8116 | 382 |
| Vitamin A | 18932 | 2092 | 7239 | 587 |
| Blend | 16250 | 4040 | 7558 | 1854 |
| Thiamin | 18615 | 447 | 6581 | 173 |
| Biotin | 18454 | 352 | 6598 | 126 |
| Riboflavin | 17786 | 521 | 6466 | 179 |
| Selenium | 16521 | 563 | 5541 | 199 |
| Chromium | 16353 | 591 | 5782 | 260 |
| Fiber (unspecified) | 1397 | 15473 | 549 | 6919 |
| Iron | 16096 | 484 | 6910 | 193 |
| Proprietary Blend | 14759 | 1537 | 7900 | 906 |
| Manganese | 15649 | 447 | 5267 | 182 |
| Blend (Combination) | 12803 | 2607 | 4955 | 1178 |
| Iodine | 13898 | 598 | 4926 | 297 |
| Copper | 13832 | 447 | 4782 | 180 |
| Protein (unspecified) | 13914 | 353 | 5770 | 179 |
| Saturated fat | 723 | 13435 | 263 | 6170 |
| Proprietary Blend (Combination) | 12707 | 1216 | 4869 | 535 |
| Cholesterol | 13688 | 218 | 6164 | 116 |
| Pantothenic Acid (Vitamin B5) | 12051 | 114 | 4468 | 60 |
| Choline | 9833 | 1824 | 3337 | 726 |
| Ginger | 3395 | 7103 | 1164 | 2589 |
| Molybdenum | 10048 | 343 | 3251 | 143 |
| Proprietary Blend (Herb/Botanical) | 9798 | 573 | 3350 | 266 |
| Inositol | 8723 | 1502 | 2957 | 461 |
| Grape | 3998 | 5794 | 1375 | 1939 |
| Turmeric | 4755 | 5002 | 1634 | 1653 |
| Green Tea | 4718 | 4750 | 1627 | 1965 |
| Boron | 7980 | 1061 | 2543 | 386 |
| DHA (Docosahexaenoic Acid) | 1265 | 7506 | 424 | 2714 |
| Proteolytic Enzymes (Proteases) | 1359 | 7146 | 529 | 2622 |
| Caffeine | 3103 | 5272 | 1530 | 2805 |
| Blend (Herb/Botanical) | 6620 | 1405 | 2137 | 507 |
| Blend (non-nutrient/non-botanical) | 5741 | 2228 | 1875 | 690 |
| Omega-3 | 2450 | 5430 | 865 | 1916 |
| Phosphorus | 7643 | 225 | 3330 | 109 |
| EPA (Eicosapentaenoic Acid) | 757 | 6702 | 214 | 2425 |
| Trans fat | 336 | 6881 | 125 | 2867 |
| Coenzyme Q-10 | 5809 | 1178 | 2149 | 382 |
| Header | 5402 | 1305 | 2778 | 538 |
| Lutein | 5146 | 1560 | 1588 | 464 |
| Lactobacillus acidophilus | 1109 | 5571 | 409 | 2062 |
| Bromelain | 3095 | 3339 | 1123 | 1152 |
| Taurine | 3401 | 2970 | 1385 | 1418 |
| Flavonoid (mixture) | 4834 | 1449 | 1431 | 538 |

Units of the rows that state an amount, read into one spelling each ("Gram(s)", "g" and "grams" are g; "Calorie(s)" and "{Calories}" are kcal; "UI" is IU): the 40 most common of 227.

| Unit | All labels: top-level | nested | Candidate set: top-level | nested |
| --- | --- | --- | --- | --- |
| mg | 761934 | 102597 | 295035 | 41620 |
| g | 118189 | 84765 | 54118 | 40244 |
| mcg | 167125 | 14378 | 60373 | 5443 |
| kcal | 52696 | 12640 | 22999 | 4587 |
| IU | 41521 | 2004 | 12118 | 587 |
| mcg DFE | 8615 | 147 | 4053 | 64 |
| mL | 5998 | 17 | 4117 | 2 |
| mcg RAE | 2687 | 73 | 1323 | 10 |
| mg NE | 2264 | 42 | 1147 | 7 |
| CFU | 668 | 369 | 175 | 42 |
| {Unit(s)} | 207 | 525 | 27 | 56 |
| HUT | 216 | 507 | 53 | 84 |
| CU | 156 | 366 | 34 | 37 |
| DU | 127 | 386 | 23 | 55 |
| % | 248 | 148 | 168 | 47 |
| mg alpha-TE | 377 | 2 | 157 | 2 |
| Teaspoon(s) | 310 | 6 | 178 | 0 |
| USP Unit(s) | 46 | 224 | 11 | 18 |
| FIP | 60 | 203 | 20 | 27 |
| ALU | 84 | 165 | 14 | 45 |
| kJ | 219 | 4 | 207 | 4 |
| LU | 53 | 166 | 15 | 10 |
| AGU | 71 | 111 | 8 | 18 |
| SU | 53 | 125 | 3 | 15 |
| Organism(s) | 111 | 64 | 10 | 0 |
| Unit(s) | 92 | 59 | 14 | 2 |
| SAPU | 31 | 113 | 12 | 35 |
| PU | 33 | 105 | 7 | 13 |
| mg DFE | 127 | 7 | 49 | 4 |
| DP | 34 | 98 | 6 | 12 |
| ng | 114 | 16 | 35 | 6 |
| GDU | 83 | 45 | 20 | 10 |
| FCCPU | 41 | 84 | 5 | 14 |
| mcg RE | 124 | 0 | 54 | 0 |
| HCU | 36 | 84 | 3 | 13 |
| XU | 48 | 67 | 3 | 8 |
| USP | 34 | 70 | 7 | 9 |
| {microorganisms} | 38 | 52 | 3 | 4 |
| GALU | 42 | 46 | 15 | 5 |
| BGU | 28 | 49 | 2 | 6 |

Rows in IU, by ingredient group (the 15 most common):

| Ingredient group | All labels: top-level | nested | Candidate set: top-level | nested |
| --- | --- | --- | --- | --- |
| Vitamin D | 13935 | 315 | 4179 | 91 |
| Vitamin E | 12997 | 410 | 3791 | 125 |
| Vitamin A | 9641 | 932 | 2949 | 254 |
| Vitamin A (unspecified) | 2459 | 103 | 581 | 34 |
| Vitamin D (cholecalciferol) | 635 | 16 | 93 | 10 |
| Vitamin E (unspecified) | 560 | 12 | 142 | 2 |
| Vitamin D (unspecified) | 194 | 4 | 56 | 2 |
| Vitamin A (mixed) | 173 | 0 | 42 | 0 |
| Vitamin A (vitamin A Palmitate) | 154 | 9 | 37 | 5 |
| (none) | 115 | 2 | 43 | 0 |
| Vitamin E (alpha tocopherol) | 73 | 8 | 21 | 8 |
| Vitamin E (Mixed Tocopherols and Tocotrienols) | 13 | 58 | 1 | 8 |
| Vitamin A (vitamin A acetate) | 68 | 0 | 28 | 0 |
| Superoxide Dismutase | 46 | 20 | 12 | 10 |
| Vitamin E (mixed tocopherols) | 55 | 0 | 12 | 0 |

## 7. Mapping to Open Food Log nutrient keys

The table the measure maps by, from DSLD's `ingredientGroup` to a key of `schema/v0/nutrients.json` (52 keys). A group is compared by the text before its parenthesis ("Vitamin E (mixed tocopherols)" is "vitamin e"). A name rule keeps a compound out of its element's key: DSLD puts "Sodium Alginate" in the group Sodium and "Choline Bitartrate" in Choline, and the amount on such a row is the compound's weight.

| Key | Unit | DSLD ingredient groups | The row's name must | must not (name or the group's parenthesis) |
| --- | --- | --- | --- | --- |
| `energy_kcal` | kcal | calories, header | match `^(total )?(calories\|energy)$` |  |
| `protein_g` | g | protein | match `^(total \|complete )?proteins?$` |  |
| `fat_g` | g | fat | match `^(total )?fats?( total)?$` |  |
| `saturated_fat_g` | g | saturated fat |  |  |
| `monounsaturated_fat_g` | g | monounsaturated fat |  |  |
| `polyunsaturated_fat_g` | g | polyunsaturated fat |  |  |
| `trans_fat_g` | g | trans fat |  |  |
| `ala_g` | g | alpha-linolenic acid |  |  |
| `epa_g` | g | epa |  |  |
| `dha_g` | g | dha |  |  |
| `cholesterol_mg` | mg | cholesterol | match `^cholesterol$` |  |
| `added_sugars_g` | g | sugar, carbohydrate | match `^added sugars?$` |  |
| `sugars_g` | g | sugar, carbohydrate | match `^(total )?sugars?$` |  |
| `carbohydrate_g` | g | carbohydrate | match `^(total )?carbohydrates?$` |  |
| `fiber_g` | g | fiber | match `^(total )?(dietary )?fib(er\|re)s?$` |  |
| `starch_g` | g | starch | match `^starch$` |  |
| `polyols_g` | g | sugar alcohol |  |  |
| `sodium_mg` | mg | sodium | match `^sodium$` |  |
| `potassium_mg` | mg | potassium | match `^(elemental \|total )?potassium$` |  |
| `calcium_mg` | mg | calcium | match `^(elemental \|total )?calcium$` |  |
| `iron_mg` | mg | iron | match `^(elemental \|total )?iron$` |  |
| `magnesium_mg` | mg | magnesium | match `^(elemental \|total )?magnesium$` |  |
| `phosphorus_mg` | mg | phosphorus | match `^(elemental \|total )?phosphoro?us$` |  |
| `zinc_mg` | mg | zinc | match `^(elemental \|total )?zinc$` |  |
| `copper_mg` | mg | copper | match `^(elemental \|total )?copper$` |  |
| `manganese_mg` | mg | manganese | match `^(elemental \|total )?manganese$` |  |
| `selenium_ug` | µg | selenium | match `^(elemental \|total )?selenium$` |  |
| `iodine_ug` | µg | iodine | match `^(elemental \|total )?iodine$` |  |
| `chromium_ug` | µg | chromium | match `^(elemental \|total )?chromium$` |  |
| `molybdenum_ug` | µg | molybdenum | match `^(elemental \|total )?molybdenum$` |  |
| `chloride_mg` | mg | chloride | match `^(elemental \|total )?chloride$` |  |
| `vitamin_a_rae_ug` | µg | vitamin a |  |  |
| `vitamin_c_mg` | mg | vitamin c |  |  |
| `vitamin_d_ug` | µg | vitamin d, vitamin d3 |  |  |
| `vitamin_e_mg` | mg | vitamin e |  | match `gamma\|delta\|beta\|tocotrienol\|mixed\|tocopherols\|toco(ph\|f)ersolan` |
| `vitamin_k_ug` | µg | vitamin k |  | match `k-?2\|mena\|mk-?[0-9]\|mixed` |
| `thiamin_mg` | mg | thiamin, vitamin b1 |  |  |
| `riboflavin_mg` | mg | riboflavin, vitamin b2 |  |  |
| `niacin_mg` | mg | niacin, vitamin b3 |  | match `riboside` |
| `pantothenic_acid_mg` | mg | pantothenic acid, vitamin b5 |  | match `pantethine` |
| `vitamin_b6_mg` | mg | vitamin b6, pyridoxine |  |  |
| `biotin_ug` | µg | biotin, vitamin b7 |  |  |
| `folic_acid_ug` | µg | folate, vitamin b9, folic acid | match `^(coenzyme )?folic acid` |  |
| `folate_dfe_ug` | µg | folate, vitamin b9, folic acid |  |  |
| `vitamin_b12_ug` | µg | vitamin b12 |  |  |
| `choline_mg` | mg | choline | match `^(elemental \|total )?choline$` |  |
| `caffeine_mg` | mg | caffeine | match `caffeine\|trimethylxanthine` | match `malate\|citrate\|pterostilbene\|extract` |
| `alcohol_g` | g | alcohol | match `^(alcohol\|ethanol)$` |  |

Keys with no rule: `energy_kj`, `carbohydrate_available_g`, `salt_g`, `water_g`. Energy in kJ is read through `energy_kcal`'s rule, by its unit; salt and water are ingredients on these labels, not declared nutrients; "net carbohydrates" isn't available carbohydrate as the vocabulary defines it.

A nested row whose key a row above it already carries is **a part of a mapped row** (the beta-carotene inside vitamin A, the vitamin K1 inside vitamin K): mapping it would count the amount twice, so it is counted apart and left out of the shares.

Every row, top-level and nested:

| | All labels | Share | Candidate set | Share |
| --- | --- | --- | --- | --- |
| Mapped to a key | 840357 | 43.5% | 337308 | 44.2% |
| In a group with a key, but named otherwise (a compound, or a form the key doesn't mean) | 43650 | 2.3% | 16779 | 2.2% |
| In a group with no key | 1049690 | 54.3% | 408294 | 53.6% |
| **Rows counted** | **1933697** | | **762381** | |
| Parts of a mapped row, left out | 5864 | | 1895 | |

Rows that state an amount (a stated zero included):

| | All labels | Share | Candidate set | Share |
| --- | --- | --- | --- | --- |
| Mapped to a key | 814541 | 58.9% | 326296 | 59.5% |
| In a group with a key, but named otherwise (a compound, or a form the key doesn't mean) | 34170 | 2.5% | 12665 | 2.3% |
| In a group with no key | 533674 | 38.6% | 209468 | 38.2% |
| **Rows counted** | **1382385** | | **548429** | |
| Parts of a mapped row, left out | 5602 | | 1824 | |

By product type, candidate set (parts of a mapped row left out):

| Product type | Rows | Mapped | Share | Rows with an amount | Mapped | Share |
| --- | --- | --- | --- | --- | --- | --- |
| A1325 Other Combinations | 495009 | 212250 | 42.9% | 348915 | 206791 | 59.3% |
| A1306 Botanical | 75187 | 7455 | 9.9% | 41661 | 7207 | 17.3% |
| A1317 Botanical with Nutrients | 50582 | 24355 | 48.1% | 37955 | 24011 | 63.3% |
| A1305 Amino acid/Protein | 37114 | 28924 | 77.9% | 32134 | 25740 | 80.1% |
| A1309 Non-Nutrient/Non-Botanical | 30089 | 4706 | 15.6% | 17364 | 4603 | 26.5% |
| A1315 Multi-Vitamin and Mineral (MVM) | 25927 | 24769 | 95.5% | 24884 | 23851 | 95.8% |
| A1310 Fat/Fatty Acid | 24549 | 14900 | 60.7% | 22730 | 14381 | 63.3% |
| A1302 Vitamin | 10186 | 9026 | 88.6% | 9921 | 8988 | 90.6% |
| A1299 Mineral | 7886 | 6216 | 78.8% | 7210 | 6043 | 83.8% |
| A1326 Fiber and Other Nutrients | 3135 | 2095 | 66.8% | 2948 | 2074 | 70.4% |
| A1316 Single Vitamin and Mineral | 2717 | 2612 | 96.1% | 2707 | 2607 | 96.3% |

Labels by how much of what they state the vocabulary holds (rows with an amount, parts of a mapped row left out):

| | All labels | Share | Candidate set | Share |
| --- | --- | --- | --- | --- |
| Every row with an amount maps | 26503 | 12.3% | 11396 | 12.4% |
| Some rows with an amount map | 91930 | 42.8% | 36694 | 39.8% |
| No row with an amount maps | 90688 | 42.2% | 41862 | 45.4% |
| No row states an amount | 5659 | 2.6% | 2164 | 2.3% |

The 40 ingredient groups with no key that the most candidate labels carry (a label counts once per group; rows count every row, top-level or nested):

| Ingredient group | Candidate labels | Share of them | Rows | Rows with an amount |
| --- | --- | --- | --- | --- |
| Proprietary Blend | 7669 | 8.3% | 8806 | 7633 |
| Blend | 6223 | 6.8% | 9412 | 6050 |
| Proprietary Blend (Combination) | 4627 | 5.0% | 5404 | 4763 |
| Blend (Combination) | 3964 | 4.3% | 6133 | 4921 |
| Ginger | 3628 | 3.9% | 3753 | 1252 |
| Green Tea | 3413 | 3.7% | 3592 | 1807 |
| Inositol | 3388 | 3.7% | 3418 | 3035 |
| Proprietary Blend (Herb/Botanical) | 3375 | 3.7% | 3616 | 3366 |
| Turmeric | 2969 | 3.2% | 3287 | 1779 |
| Boron | 2907 | 3.2% | 2929 | 2678 |
| Taurine | 2770 | 3.0% | 2803 | 1780 |
| Grape | 2729 | 3.0% | 3314 | 1514 |
| Leucine | 2721 | 3.0% | 2894 | 1892 |
| Coenzyme Q-10 | 2509 | 2.7% | 2531 | 2203 |
| Lactobacillus acidophilus | 2361 | 2.6% | 2471 | 280 |
| Proteolytic Enzymes (Proteases) | 2343 | 2.5% | 3151 | 541 |
| Black Pepper | 2277 | 2.5% | 2334 | 1489 |
| Glutamine | 2273 | 2.5% | 2367 | 1453 |
| Valine | 2249 | 2.4% | 2308 | 1398 |
| Omega-3 | 2247 | 2.4% | 2781 | 2568 |
| Blend (non-nutrient/non-botanical) | 2233 | 2.4% | 2565 | 1812 |
| Bromelain | 2231 | 2.4% | 2275 | 1141 |
| Isoleucine | 2222 | 2.4% | 2272 | 1376 |
| Lipase | 2182 | 2.4% | 2270 | 386 |
| Beta-Alanine | 2151 | 2.3% | 2209 | 1578 |
| Amylase | 2116 | 2.3% | 2221 | 383 |
| Blend (Herb/Botanical) | 2075 | 2.3% | 2644 | 2165 |
| Ashwagandha | 2046 | 2.2% | 2141 | 1195 |
| Lutein | 2006 | 2.2% | 2052 | 1794 |
| Creatine | 2005 | 2.2% | 2599 | 1768 |
| Tyrosine | 2004 | 2.2% | 2051 | 1233 |
| Alpha Lipoic Acid | 1934 | 2.1% | 1959 | 1550 |
| Flavonoid (mixture) | 1857 | 2.0% | 1969 | 1521 |
| Licorice | 1842 | 2.0% | 1874 | 628 |
| cranberry | 1822 | 2.0% | 1907 | 747 |
| Arginine | 1769 | 1.9% | 1878 | 982 |
| Blueberry | 1758 | 1.9% | 1962 | 346 |
| Fish Oil | 1715 | 1.9% | 1734 | 1620 |
| Beet | 1712 | 1.9% | 1772 | 611 |
| Broccoli | 1688 | 1.8% | 2023 | 494 |

The 30 most common rows a name rule kept out of their group's key, candidate set:

| Group: name | Rows |
| --- | --- |
| Calories: Calories from Fat | 5162 |
| Choline: Choline Bitartrate | 949 |
| Vitamin K (menaquinone): Vitamin K2 | 534 |
| Fiber (unspecified): Soluble Fiber | 445 |
| Vitamin K: Vitamin K2 | 441 |
| Vitamin E: Mixed Tocopherols | 206 |
| Caffeine: Caffeine Citrate | 175 |
| Magnesium: Magnesium Oxide | 167 |
| Header: Calories from Fat | 159 |
| Calcium: Calcium Carbonate | 157 |
| Fiber (unspecified): Insoluble Fiber | 150 |
| Magnesium: Magnesium Citrate | 149 |
| Caffeine: Infinergy | 138 |
| Magnesium: Magnesium Complex | 122 |
| Potassium: Dipotassium Phosphate | 106 |
| Calcium: Calcium Citrate | 88 |
| Carbohydrate: Other Carbohydrates | 88 |
| Caffeine: Dicaffeine Malate | 85 |
| Vitamin E: Gamma-Tocopherol | 75 |
| Zinc: Zinc Gluconate | 73 |
| Vitamin E (mixed tocopherols): natural Mixed Tocopherols | 70 |
| Vitamin E (mixed tocotrienols): Tocotrienols | 70 |
| Vitamin E: D-Gamma-Tocopherol | 70 |
| Vitamin K (menaquinone): Vitamin K-2 | 69 |
| Potassium: Potassium Chloride | 68 |
| Calcium: Calcium D-Glucarate | 67 |
| Caffeine: Di-Caffeine Malate | 66 |
| Calcium: Calcium Beta-Hydroxybutyrate | 66 |
| Sodium: Sodium Beta-Hydroxybutyrate | 65 |
| Sodium: Sodium Citrate | 65 |

Mapped rows with an amount, by key, candidate set, and how the amount reaches the key's unit: directly (grams, milligrams and micrograms into each other, kcal and kJ, and a unit naming the key's own equivalent: mcg RAE, mcg DFE, mg NE, mg alpha-TE); through an IU conversion (vitamins A, D and E only); folate in plain mcg or mg, which a label from before the 2016 rule meant as micrograms of folate and a later one means as DFE with the letters dropped; or not at all (IU for another nutrient, mcg RE, a percentage, a unit that isn't a mass).

| Key | Labels | Rows | Directly | IU | Folate, not stated as DFE | Can't be converted |
| --- | --- | --- | --- | --- | --- | --- |
| `energy_kcal` | 22208 | 22490 | 22407 | 0 | 0 | 83 |
| `protein_g` | 8220 | 8452 | 8450 | 0 | 0 | 2 |
| `fat_g` | 11212 | 11289 | 11280 | 0 | 0 | 9 |
| `saturated_fat_g` | 6325 | 6390 | 6386 | 0 | 0 | 4 |
| `monounsaturated_fat_g` | 933 | 933 | 932 | 0 | 0 | 1 |
| `polyunsaturated_fat_g` | 1415 | 1415 | 1415 | 0 | 0 | 0 |
| `trans_fat_g` | 3181 | 3194 | 3194 | 0 | 0 | 0 |
| `ala_g` | 508 | 510 | 507 | 0 | 0 | 3 |
| `epa_g` | 2338 | 2357 | 2357 | 0 | 0 | 0 |
| `dha_g` | 2708 | 2789 | 2788 | 0 | 0 | 1 |
| `cholesterol_mg` | 6173 | 6221 | 6220 | 0 | 0 | 1 |
| `carbohydrate_g` | 18209 | 18353 | 18344 | 0 | 0 | 9 |
| `fiber_g` | 6797 | 6842 | 6841 | 0 | 0 | 1 |
| `sugars_g` | 12050 | 12162 | 12154 | 0 | 0 | 8 |
| `added_sugars_g` | 5139 | 5232 | 5228 | 0 | 0 | 4 |
| `starch_g` | 2 | 2 | 2 | 0 | 0 | 0 |
| `polyols_g` | 597 | 599 | 599 | 0 | 0 | 0 |
| `sodium_mg` | 12356 | 12465 | 12461 | 0 | 0 | 4 |
| `potassium_mg` | 9540 | 9607 | 9604 | 0 | 0 | 3 |
| `calcium_mg` | 14570 | 14661 | 14623 | 0 | 0 | 38 |
| `iron_mg` | 5963 | 6006 | 5971 | 0 | 0 | 35 |
| `magnesium_mg` | 10397 | 10477 | 10469 | 0 | 0 | 8 |
| `phosphorus_mg` | 3026 | 3034 | 3027 | 0 | 0 | 7 |
| `zinc_mg` | 9597 | 9647 | 9640 | 0 | 0 | 7 |
| `copper_mg` | 4741 | 4754 | 4749 | 0 | 0 | 5 |
| `manganese_mg` | 5240 | 5250 | 5247 | 0 | 0 | 3 |
| `selenium_ug` | 5552 | 5561 | 5557 | 0 | 0 | 4 |
| `iodine_ug` | 4883 | 4903 | 4902 | 0 | 0 | 1 |
| `chromium_ug` | 5594 | 5618 | 5617 | 0 | 0 | 1 |
| `molybdenum_ug` | 3268 | 3273 | 3273 | 0 | 0 | 0 |
| `chloride_mg` | 1307 | 1320 | 1320 | 0 | 0 | 0 |
| `vitamin_a_rae_ug` | 7682 | 7777 | 3961 | 3712 | 0 | 104 |
| `vitamin_c_mg` | 13956 | 14089 | 14053 | 0 | 0 | 36 |
| `vitamin_d_ug` | 11129 | 11295 | 6851 | 4435 | 0 | 9 |
| `vitamin_e_mg` | 8472 | 8565 | 4464 | 4057 | 0 | 44 |
| `vitamin_k_ug` | 2529 | 2547 | 2544 | 0 | 0 | 3 |
| `thiamin_mg` | 7094 | 7126 | 7121 | 0 | 0 | 5 |
| `riboflavin_mg` | 7133 | 7166 | 7160 | 0 | 0 | 6 |
| `niacin_mg` | 8944 | 9047 | 9043 | 0 | 0 | 4 |
| `pantothenic_acid_mg` | 7277 | 7302 | 7301 | 0 | 0 | 1 |
| `vitamin_b6_mg` | 11104 | 11210 | 11206 | 0 | 0 | 4 |
| `biotin_ug` | 6967 | 6989 | 6986 | 0 | 0 | 3 |
| `folate_dfe_ug` | 5780 | 5813 | 4051 | 0 | 1761 | 1 |
| `folic_acid_ug` | 5460 | 5491 | 5413 | 0 | 0 | 78 |
| `vitamin_b12_ug` | 10409 | 10493 | 10482 | 0 | 0 | 11 |
| `choline_mg` | 2874 | 2889 | 2888 | 0 | 0 | 1 |
| `caffeine_mg` | 2456 | 2690 | 2690 | 0 | 0 | 0 |
| `alcohol_g` | 1 | 1 | 0 | 0 | 0 | 1 |
| **All** | | **326296** | **311778** (95.6%) | **12204** (3.7%) | **1761** (0.5%) | **553** (0.2%) |

The rows in IU, by the form the row states (its name, its `forms` and its `notes`), candidate set. One IU of vitamin D is 0.025 mcg whatever the form. One IU of vitamin A is 0.3 mcg RAE as retinol and, in a supplement, as beta-carotene (0.05 as the beta-carotene of a food). One IU of vitamin E is 0.67 mg as natural d-alpha-tocopherol and 0.45 mg as synthetic dl-alpha-tocopherol, so a vitamin E row that doesn't say which can't be converted.

| Key | Form stated | Rows |
| --- | --- | --- |
| `vitamin_a_rae_ug` | beta-carotene | 1433 |
| `vitamin_a_rae_ug` | not stated | 791 |
| `vitamin_a_rae_ug` | retinol | 757 |
| `vitamin_a_rae_ug` | retinol and beta-carotene | 731 |
| `vitamin_d_ug` | not needed (one factor for D2 and D3) | 4435 |
| `vitamin_e_mg` | natural (d-alpha) | 2193 |
| `vitamin_e_mg` | not stated | 1065 |
| `vitamin_e_mg` | synthetic (dl-alpha) | 764 |
| `vitamin_e_mg` | natural and synthetic | 35 |

## 8. The %DV check

For every mapped row, each amount that carries a percent daily value for "Adults and children 4 or more years of age", both stated exactly (`=`, not `<`) and above zero, in a unit that converts: the amount against the percent × FDA's daily value (21 CFR 101.9(c)(8)(iv) and (c)(9)). A label from before the 2016 rule states percents of the old daily values, so an amount **agrees** when it is the percent of the current daily value or of the old one, within 10% or one percentage point (labels round the percent). It is **off by a factor of 1000** when a thousandth or a thousand times the amount would agree (mcg written for mg, or the reverse), and **disagrees** otherwise. An amount in IU is read against the old daily value in IU, and against the current one through the IU factors above (either form's, for vitamin E).

Candidate set:

| Key | Checked | Agree | … only with the current DV | … only with the DV before 2016 | … with either | Off by 1000 | Disagree | … narrowly | Not agreeing | … leaving out the narrow ones |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `protein_g` | 4936 | 4611 | 0 | 0 | 4611 | 19 | 306 | 101 | 6.6% | 4.5% |
| `fat_g` | 9149 | 9006 | 814 | 1016 | 7176 | 11 | 132 | 77 | 1.6% | 0.7% |
| `saturated_fat_g` | 4424 | 4180 | 0 | 0 | 4180 | 5 | 239 | 200 | 5.5% | 1.0% |
| `cholesterol_mg` | 4344 | 4248 | 0 | 0 | 4248 | 16 | 80 | 62 | 2.2% | 0.8% |
| `carbohydrate_g` | 11930 | 11760 | 81 | 219 | 11460 | 23 | 147 | 69 | 1.4% | 0.8% |
| `fiber_g` | 4858 | 4612 | 915 | 1188 | 2509 | 22 | 224 | 170 | 5.1% | 1.6% |
| `added_sugars_g` | 3113 | 3033 | 3033 | 0 | 0 | 5 | 75 | 46 | 2.6% | 1.1% |
| `sodium_mg` | 9359 | 9158 | 10 | 35 | 9113 | 22 | 179 | 112 | 2.1% | 1.0% |
| `potassium_mg` | 8508 | 8305 | 1973 | 1750 | 4582 | 27 | 176 | 90 | 2.4% | 1.3% |
| `calcium_mg` | 14303 | 13913 | 5574 | 5585 | 2754 | 39 | 351 | 284 | 2.7% | 0.7% |
| `iron_mg` | 5278 | 4874 | 0 | 0 | 4874 | 21 | 383 | 313 | 7.7% | 1.7% |
| `magnesium_mg` | 10304 | 10088 | 62 | 191 | 9835 | 24 | 192 | 104 | 2.1% | 1.1% |
| `phosphorus_mg` | 3052 | 2961 | 915 | 1191 | 855 | 5 | 86 | 44 | 3.0% | 1.5% |
| `zinc_mg` | 9305 | 9136 | 5353 | 3705 | 78 | 25 | 144 | 71 | 1.8% | 1.1% |
| `copper_mg` | 4576 | 4395 | 2207 | 2174 | 14 | 14 | 167 | 122 | 4.0% | 1.3% |
| `manganese_mg` | 5170 | 5061 | 2518 | 2431 | 112 | 30 | 79 | 35 | 2.1% | 1.4% |
| `selenium_ug` | 5460 | 5368 | 2899 | 2364 | 105 | 32 | 60 | 31 | 1.7% | 1.1% |
| `iodine_ug` | 4682 | 4585 | 0 | 0 | 4585 | 44 | 53 | 11 | 2.1% | 1.8% |
| `chromium_ug` | 5633 | 5446 | 2860 | 2585 | 1 | 34 | 153 | 105 | 3.3% | 1.5% |
| `molybdenum_ug` | 3237 | 3102 | 1632 | 1469 | 1 | 27 | 108 | 82 | 4.2% | 1.6% |
| `chloride_mg` | 1247 | 1211 | 364 | 442 | 405 | 3 | 33 | 9 | 2.9% | 2.2% |
| `vitamin_a_rae_ug` | 7243 | 7034 | 3569 | 3345 | 120 | 25 | 184 | 93 | 2.9% | 1.6% |
| `vitamin_c_mg` | 13644 | 13133 | 7120 | 5865 | 148 | 43 | 468 | 378 | 3.7% | 1.0% |
| `vitamin_d_ug` | 9976 | 9666 | 5517 | 4116 | 33 | 35 | 275 | 50 | 3.1% | 2.6% |
| `vitamin_e_mg` | 8065 | 7745 | 3731 | 873 | 3141 | 35 | 285 | 174 | 4.0% | 1.8% |
| `vitamin_k_ug` | 2479 | 2409 | 1223 | 1179 | 7 | 20 | 50 | 32 | 2.8% | 1.5% |
| `thiamin_mg` | 6773 | 6637 | 3453 | 3155 | 29 | 19 | 117 | 52 | 2.0% | 1.2% |
| `riboflavin_mg` | 6824 | 6678 | 3483 | 3147 | 48 | 11 | 135 | 85 | 2.1% | 0.9% |
| `niacin_mg` | 8697 | 8594 | 4303 | 4153 | 138 | 10 | 93 | 35 | 1.2% | 0.8% |
| `pantothenic_acid_mg` | 7168 | 6895 | 3661 | 3222 | 12 | 30 | 243 | 191 | 3.8% | 1.1% |
| `vitamin_b6_mg` | 10864 | 10399 | 5279 | 4973 | 147 | 25 | 440 | 317 | 4.3% | 1.4% |
| `biotin_ug` | 6766 | 6523 | 3588 | 2932 | 3 | 33 | 210 | 43 | 3.6% | 3.0% |
| `folate_dfe_ug` | 5523 | 5387 | 0 | 0 | 5387 | 29 | 107 | 12 | 2.5% | 2.2% |
| `folic_acid_ug` | 2749 | 2544 | 5 | 2536 | 3 | 12 | 193 | 163 | 7.5% | 1.5% |
| `vitamin_b12_ug` | 10164 | 9897 | 5320 | 4577 | 0 | 79 | 188 | 64 | 2.6% | 2.0% |
| `choline_mg` | 1556 | 1535 | 1535 | 0 | 0 | 6 | 15 | 4 | 1.3% | 1.1% |
| **All** | **241359** | **234129** | **82997** | **70418** | **80714** | **860** | **6370** | **3831** | **3.0%** | **1.4%** |

All labels:

| Key | Checked | Agree | … only with the current DV | … only with the DV before 2016 | … with either | Off by 1000 | Disagree | … narrowly | Not agreeing | … leaving out the narrow ones |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `protein_g` | 10711 | 10052 | 0 | 0 | 10052 | 37 | 622 | 216 | 6.2% | 4.1% |
| `fat_g` | 20965 | 20676 | 1415 | 2631 | 16630 | 31 | 258 | 165 | 1.4% | 0.6% |
| `saturated_fat_g` | 9170 | 8642 | 0 | 0 | 8642 | 62 | 466 | 399 | 5.8% | 1.4% |
| `cholesterol_mg` | 9037 | 8798 | 0 | 0 | 8798 | 48 | 191 | 138 | 2.6% | 1.1% |
| `carbohydrate_g` | 24872 | 24466 | 135 | 492 | 23839 | 50 | 356 | 156 | 1.6% | 1.0% |
| `fiber_g` | 10389 | 9880 | 1823 | 2920 | 5137 | 32 | 477 | 375 | 4.9% | 1.3% |
| `added_sugars_g` | 5519 | 5396 | 5396 | 0 | 0 | 5 | 118 | 78 | 2.2% | 0.8% |
| `sodium_mg` | 18965 | 18525 | 48 | 72 | 18405 | 55 | 385 | 218 | 2.3% | 1.2% |
| `potassium_mg` | 18945 | 18485 | 3457 | 4301 | 10727 | 47 | 413 | 204 | 2.4% | 1.4% |
| `calcium_mg` | 34461 | 33542 | 10986 | 16326 | 6230 | 73 | 846 | 606 | 2.7% | 0.9% |
| `iron_mg` | 12393 | 11456 | 0 | 0 | 11456 | 59 | 878 | 743 | 7.6% | 1.6% |
| `magnesium_mg` | 27577 | 26961 | 188 | 436 | 26337 | 54 | 562 | 319 | 2.2% | 1.1% |
| `phosphorus_mg` | 7160 | 6926 | 1690 | 3326 | 1910 | 11 | 223 | 100 | 3.3% | 1.9% |
| `zinc_mg` | 25137 | 24752 | 11347 | 13230 | 175 | 46 | 339 | 149 | 1.5% | 0.9% |
| `copper_mg` | 13275 | 12846 | 4901 | 7921 | 24 | 46 | 383 | 278 | 3.2% | 1.1% |
| `manganese_mg` | 15209 | 14956 | 5743 | 8882 | 331 | 64 | 189 | 60 | 1.7% | 1.3% |
| `selenium_ug` | 16113 | 15823 | 6529 | 9084 | 210 | 111 | 179 | 52 | 1.8% | 1.5% |
| `iodine_ug` | 13306 | 13066 | 0 | 0 | 13066 | 85 | 155 | 43 | 1.8% | 1.5% |
| `chromium_ug` | 15799 | 15310 | 6099 | 9208 | 3 | 81 | 408 | 268 | 3.1% | 1.4% |
| `molybdenum_ug` | 9845 | 9516 | 3827 | 5680 | 9 | 63 | 266 | 210 | 3.3% | 1.2% |
| `chloride_mg` | 2475 | 2405 | 571 | 1047 | 787 | 4 | 66 | 28 | 2.8% | 1.7% |
| `vitamin_a_rae_ug` | 20086 | 19540 | 7627 | 11684 | 229 | 48 | 498 | 218 | 2.7% | 1.6% |
| `vitamin_c_mg` | 35669 | 34536 | 14690 | 19511 | 335 | 83 | 1050 | 828 | 3.2% | 0.9% |
| `vitamin_d_ug` | 26570 | 25760 | 11486 | 14214 | 60 | 90 | 720 | 201 | 3.0% | 2.3% |
| `vitamin_e_mg` | 22581 | 21817 | 8034 | 2912 | 10871 | 113 | 651 | 391 | 3.4% | 1.7% |
| `vitamin_k_ug` | 6975 | 6769 | 2663 | 4073 | 33 | 35 | 171 | 121 | 3.0% | 1.2% |
| `thiamin_mg` | 19521 | 19111 | 7768 | 11269 | 74 | 49 | 361 | 197 | 2.1% | 1.1% |
| `riboflavin_mg` | 19595 | 19179 | 7826 | 11232 | 121 | 34 | 382 | 211 | 2.1% | 1.0% |
| `niacin_mg` | 23555 | 23290 | 9413 | 13589 | 288 | 17 | 248 | 87 | 1.1% | 0.8% |
| `pantothenic_acid_mg` | 20373 | 19753 | 8089 | 11641 | 23 | 74 | 546 | 390 | 3.0% | 1.1% |
| `vitamin_b6_mg` | 28848 | 27788 | 11292 | 16217 | 279 | 59 | 1001 | 696 | 3.7% | 1.3% |
| `biotin_ug` | 18903 | 18343 | 7792 | 10548 | 3 | 89 | 471 | 92 | 3.0% | 2.5% |
| `folate_dfe_ug` | 14327 | 13993 | 0 | 0 | 13993 | 57 | 277 | 28 | 2.3% | 2.1% |
| `folic_acid_ug` | 8272 | 7793 | 17 | 7771 | 5 | 34 | 445 | 360 | 5.8% | 1.4% |
| `vitamin_b12_ug` | 26882 | 26127 | 11188 | 14936 | 3 | 194 | 561 | 190 | 2.8% | 2.1% |
| `choline_mg` | 3422 | 3382 | 3382 | 0 | 0 | 15 | 25 | 9 | 1.2% | 0.9% |
| **All** | **616902** | **599660** | **175422** | **235153** | **189085** | **2055** | **15187** | **8824** | **2.8%** | **1.4%** |

"With either" is where the two daily values are the same or within a tenth of each other (iron, iodine, folate, saturated fat, cholesterol, protein, sodium, magnesium, carbohydrate; vitamin E, whose old 30 IU is 13.5 mg as the synthetic form, within a tenth of today's 15 mg), or where the percent is small enough that both round to it. A disagreement is **narrow** when the amount is within 25% or three percentage points of the percent: labels round their amounts too (fat to the half gram, sodium to 5 or 10 mg), so a small amount's percent can sit a point or two from the one worked out here. Protein's percent is of protein corrected for its quality (21 CFR 101.9(c)(7)), so it can sit below amount ÷ 50 g without being wrong.

Labels by the daily values their percents agree with, which says which rule a label was printed under (a row counts when its percent agrees with one daily value and not the other):

| | All labels | Share | Candidate set | Share |
| --- | --- | --- | --- | --- |
| Only the current daily values | 37578 | 17.5% | 19312 | 21.0% |
| Only the daily values before 2016 | 45400 | 21.1% | 14404 | 15.6% |
| Some rows with each | 1908 | 0.9% | 823 | 0.9% |
| Can't tell (every percent agrees with either, or none agrees) | 21946 | 10.2% | 8950 | 9.7% |
| No percent to check | 107948 | 50.3% | 48627 | 52.8% |

Ten that disagree by more than narrowly, candidate set (the lowest DSLD IDs, one key at a time):

| DSLD ID | Product | Row | Key | Amount | Percent DV |
| --- | --- | --- | --- | --- | --- |
| 801 | Maximum Nutrition Vanilla Bean (GNC Mega Men) | Vitamin B12 | `vitamin_b12_ug` | 50 mcg | 8330% |
| 1060 | Sport Milk Chocolate (GNC Mega Men) | Zinc | `zinc_mg` | 25 mg | 100% |
| 1752 | V.M.T. Regular Strength (Viva Vitamins) | Copper | `copper_mg` | 75 mcg | 38% |
| 1791 | CSA Complete Spectrum Antioxidant (Viva Vitamins) | Vitamin B6 | `vitamin_b6_mg` | 50 mg | 833% |
| 2123 | Bulk-Up Weight Gainer Chocolate (Tapout Sports Nutrition) | Molybdenum | `molybdenum_ug` | 8 mcg | 2% |
| 2126 | Recover Post Workout (Tapout Sports Nutrition) | Iron | `iron_mg` | 2.5 mg | 1.4% |
| 2353 | Healthy Energy Low Cal Orange (FRS) | Total Carbohydrates | `carbohydrate_g` | 5 g | 10% |
| 2430 | Hemavol (iForce Nutrition) | Niacin | `niacin_mg` | 25 mg | 250% |
| 2489 | Green SuperFood Raw Reserve (Amazing Grass) | Vitamin K | `vitamin_k_ug` | 80 mcg | 50% |
| 2489 | Green SuperFood Raw Reserve (Amazing Grass) | Riboflavin | `riboflavin_mg` | 0.6 mg | 20% |

Five off by a factor of 1000, candidate set:

| DSLD ID | Product | Row | Key | Amount | Percent DV |
| --- | --- | --- | --- | --- | --- |
| 788 | Active (GNC Women's Ultra Mega) | Iodine | `iodine_ug` | 150 mg | 100% |
| 801 | Maximum Nutrition Vanilla Bean (GNC Mega Men) | Molybdenum | `molybdenum_ug` | 75 mg | 100% |
| 909 | Mega B-150 (Vitamin World) | Folic Acid | `folic_acid_ug` | 400 mg | 100% |
| 909 | Mega B-150 (Vitamin World) | Pantothenic Acid | `pantothenic_acid_mg` | 150 mcg | 1500% |
| 1753 | V.M.T. Extra Strength (Viva Vitamins) | Copper | `copper_mg` | 1.5 mcg | 75% |

## 9. Size estimate

One draft record for each of the candidate set's 85398 GTINs, from the first on-market label read for it (the lowest DSLD ID), encoded and thrown away: an OFL branded food (`gtin:` ID, name, brand, the label's serving as text, `label.nutrients_per_serving` for the mapped rows that convert directly, `dsld_id`, provenance), with or without the other rows as written (name, amount, unit) and the label's other ingredients as text. The shape is a guess for sizing, not a decision. Gzipped at the default level.

| | JSONL | Per record | Gzipped | Per record |
| --- | --- | --- | --- | --- |
| With every row and the other ingredients | 76.6 MB | 897 bytes | 8.1 MB | 95 bytes |
| Mapped nutrients only | 46.3 MB | 542 bytes | 3.9 MB | 45 bytes |

A record per candidate label, not per GTIN, would be about 1.08 times that (92116 labels).

## 10. Other things a build's rules need

Counts the sections above don't hold.

| | All labels | Share | Candidate set | Share |
| --- | --- | --- | --- | --- |
| A row the label gives no amount for (unit `NP`) | 62430 | 29.1% | 25986 | 28.2% |
| A blend row (`ingredientGroup` Blend or Proprietary Blend, with or without a parenthesis) | 59485 | 27.7% | 25114 | 27.3% |
| No daily values for adults and children 4 or more (only for infants, children under 4, or pregnant and lactating women) | 1588 | 0.7% | 711 | 0.8% |
| More than one serving size, each with its own amounts | 2212 | 1.0% | 1028 | 1.1% |
| A serving size given as a range | 32475 | 15.1% | 14803 | 16.1% |

Contacts (`contacts[].contactDetails.country`: free text, often empty, sometimes a postcode) of the candidate labels:

| | Candidate labels |
| --- | --- |
| A contact naming a country other than the United States | 9001 |
| Contacts naming only the United States | 25618 |
| No contact names a country | 57497 |

The 12 most named other countries, as written:

| Country | Candidate labels |
| --- | --- |
| Canada | 6673 |
| India | 622 |
| UK | 609 |
| Portugal | 409 |
| New Zealand | 186 |
| NL | 154 |
| United Kingdom | 93 |
| Ireland | 60 |
| Germany | 49 |
| Belgium | 40 |
| CANADA | 36 |
| Malta | 29 |
