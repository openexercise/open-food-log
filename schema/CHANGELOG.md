# Changelog

## Unreleased

- The API core (`api/core.yaml`): the REST routes any Open Food Log server serves over a person's
  food log (foods, meals and their change feed, the export and webhooks, whose meal events are
  signed in `OFL-Signature`), generated from the reference server. A meal's `external_id` replays
  for the app that sent it, and one the person deleted stays deleted.
- `ofl conformance <base-url>` and `ofl conformance mcp <mcp-url>` check a server against the core
  and the MCP profile as one test person; `ofl vectors` checks an amount calculation against the
  amount vectors (package `conformance`). The reference server runs all three in its tests.
- A Go client for the core (`clients/go`); the TypeScript one is `@openexercise/open-workout-client/openfoodlog`.
- Catalogue: 1,283 liquids gain `density_g_per_ml`, so an amount in ml, l or fl oz can be weighed
  (whole milk is 1.0313 g/ml). Each comes from FoodData Central: a measured specific gravity, or the
  weight of one of the food's own volume portions over its volume, named in `provenance.sources`.
  Liquids with no volume portion, or one outside any liquid's density, have none; `catalogue/REPORT.md`
  lists them. The schemas are unchanged.

## v0 draft, MCP tool profile, 2026-11-09

The MCP tool profile (`mcp/tools.json`), generated from the reference server: the core `log_meal`,
`get_meals`, `update_meal`, `delete_meal`, `search_foods` and `undo`, marked
`dev.openfoodlog/profile`, with the reference server's `log_meal_text` and `lookup_barcode` as
extensions. Section 9 points at it.

## v0 draft, 2026-11-09

First draft: the food, meal and log schemas, the nutrient vocabulary (24 keys mapped to INFOODS,
FoodData Central, Health Connect and HealthKit), amount vectors, examples, and the specification.
