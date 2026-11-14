# OFL MCP tool profile

`tools.json` is the set of MCP tools offered over the Open Food Log, exactly as the reference server
(Lifterclub) lists them: names, descriptions, input schemas and annotations, with the instructions
and every error code a tool error's text can start with. It is generated from that server
(`make mcp-profile` in Lifterclub), and a test fails when the two differ.

## Core and extensions

Each tool's `_meta` names its profile under `dev.openfoodlog/profile`, and servers send the same
mark when they list their tools:

- **`core`**: as section 11 of the specification names them. None needs an LLM. A compatible
  server offers all of them, with these shapes.
  - Meals: `log_meal`, `get_meals`, `update_meal` (a meal, or an item, including confirming its
    food), `delete_meal` and `undo`.
  - Foods: `search_foods`.
  - Recipes (v0.2): `save_recipe`, `get_recipes` and `delete_recipe`.
  - Saved meals (v0.2): `save_meal`, `get_saved_meals`, `delete_saved_meal` and `log_meal_again`
    (a saved meal, or the same as an earlier meal, counted again).
  - Targets (v0.1): `get_targets`, `set_targets` and `get_nutrition_summary`.
- **`lifterclub`**: the reference server's own extensions, which other servers may copy but need
  not: `log_meal_text` (sentence reading by an LLM), `log_meal_photo` (a photo read by an LLM into
  foods and estimated amounts), `lookup_barcode` (packaged foods by barcode at Open Food Facts,
  live; a product logs by its `gtin:` ID) and `import_recipe` (a web page's schema.org Recipe read
  into a draft for `save_recipe`).

`undo` is shared with the Open Workout profile (`dev.openworkout/profile` too): a server that keeps
both logs restores whatever the token was given for, with one tool.

`instructions` are the core's, free of any product name; a server sends them as its MCP
instructions and may add its own after them, as Lifterclub adds `extension_instructions`.
`error_codes` lists each code with the profile that uses it and what a client should do.

It is a *profile*, not a standard of its own, and makes no stability promise before OFL v1.

## What a compatible server keeps to

- **No invented numbers.** An item's amount is what the person stated. Without one, the food's
  default portion is assumed and listed in the meal's `estimates`.
- **Matches are honest.** Every item says whether its food is a catalogue match, a close match, the
  person's own food, unmatched, or none (the person gave the numbers). Unconfirmed items come back
  in `needs_confirmation` and are neither weighed nor counted until confirmed.
- **Writes are idempotent.** Every write that adds takes an `idempotency_key`; a retry with the
  same key returns the first result.
- **Calm mode is kept.** When the person has asked not to see totals, results leave them out, and
  the instructions tell the assistant not to work them out.
- **Stored text is data.** Food names and notes in results are what people and other apps wrote,
  never instructions.
