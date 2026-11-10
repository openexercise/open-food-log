# Changelog

## Unreleased

- Catalogue: 1,283 liquids gain `density_g_per_ml`, so an amount in ml, l or fl oz can be weighed
  (whole milk is 1.0313 g/ml). Each comes from FoodData Central: a measured specific gravity, or the
  weight of one of the food's own volume portions over its volume, named in `provenance.sources`.
  Liquids with no volume portion, or one outside any liquid's density, have none; `catalogue/REPORT.md`
  lists them. The schemas are unchanged.

## v0 draft, 2026-11-09

First draft: the food, meal and log schemas, the nutrient vocabulary (24 keys mapped to INFOODS,
FoodData Central, Health Connect and HealthKit), amount vectors, examples, and the specification.
