# OFL A2A agent profile

`agent-card.json` is the agent card of an Open Food Log nutrition agent over
[A2A](https://a2a-protocol.org) (the Agent2Agent protocol), exactly as the reference agent
(Lifterclub) serves it at `/.well-known/agent-card.json`: its skills, how to authenticate, and which
skills are the profile's core. It is generated from that agent (`make a2a-profile` in Lifterclub),
and a test fails when the two differ. It sits beside the [MCP tool profile](../mcp/README.md): MCP is
for an assistant using the food log as tools; A2A is for another agent handing food-log work to
this one as tasks.

It is a *profile*, not a standard of its own, and makes no stability promise before OFL v1.

## Core and extensions

The card names the profile in `capabilities.extensions`, with URI `https://openfoodlog.org/a2a/v0`.
Its `params.core_skills` lists the core; each skill also carries the tag `openfoodlog:core` or
`openfoodlog:<server>` for the server's own extensions.

- **Core** (a compatible agent offers all of them, with these IDs and shapes):
  - `log-meal`: an Open Food Log meal ([meal.json](../schema/v0/meal.json)) as a data part, logged as
    given. A text part (the person's own words) is optional for an agent: reading it takes a model.
  - `food-lookup`: `{"query": "...", "limit": 10}` returns foods by name, best first, with IDs,
    nutrients per 100 g and portions. `{"barcode": "..."}` (packaged foods) is optional.
  - `get-meals`: `{"from": "YYYY-MM-DD", "to": "YYYY-MM-DD"}` returns the person's meals as Open
    Food Log meals, with each day's totals.
  - `nutrition-summary`: the same range with `metadata.skill`, returning each day's totals, each
    target that applied as `met`, `under`, `above` or `unknown`, and the days at target.
  - `targets`: an empty data part `{}` with `metadata.skill` reads the person's targets as an OFL
    targets document ([targets.json](../schema/v0/targets.json)); `{"targets": [...]}` replaces the whole
    list (`meals:write`). An agent sets only what the person asked for, never a suggestion.
- **Extensions**: none yet in the reference agent. Planned: photos on `log-meal` (estimates).

Descriptions of core skills name no product.

## Messages

- **Choosing a skill.** Put the skill's ID in the message's `metadata.skill`. Without it, the agent
  decides from what the message carries: a data part with `items` (a meal) or a text part logs a
  meal; `targets` sets targets; `query` or `barcode` looks a food up; `from` or `to` reads meals.
  `nutrition-summary`, and reading targets, need `metadata.skill`.
- **Results.** `log-meal` works as a task whose artifact `meal` holds a one-line summary (text), the
  logged meal (a data part whose `metadata.schema` is the meal schema's URI) and, unless calm mode
  is on, `{"totals": ...}`. The other skills answer with a message and store nothing
  beyond what they change (`targets`).
- **Confirming foods.** When a food isn't certain, the task stops in `input-required`. The status
  message's data part is `{"meal_id", "needs_confirmation": [{"item_id", "text", "candidates"}]}`.
  Answer on the same task (`taskId`) with a data part
  `{"confirmations": [{"item_id": "...", "food_id": "..."}]}`, choosing a candidate or a food from
  `food-lookup`. With one item to confirm, a text part with the candidate's ID or exact name also
  works. The task completes once every item is confirmed. Cancelling it leaves the meal logged, its
  unconfirmed items uncounted until someone confirms them.
- **Retries.** A message's `messageId` is its idempotency key: sending the same message again
  returns the first result and logs nothing new.
- **Refusals.** A refused message ends the task `rejected` (or answers with a message, for reads),
  with a text part starting with a code and a data part `{"error": {"code", "message", "field"}}`.
  Codes are the MCP profile's (`consent_required`, `invalid_input`, `nothing_to_log`,
  `parse_unsure`, `not_found`, `idempotency_conflict`, `barcode_unavailable`, `internal_error`),
  plus `insufficient_scope` for a write with a read-only grant. A call over the rate limit is an HTTP 429 with
  `Retry-After` and a JSON-RPC error whose `data.code` is `rate_limited`.

## What a compatible agent keeps to

- **A2A 1.0, over JSON-RPC**, with the `A2A-Version: 1.0` header. The reference agent also answers
  A2A 0.3 clients (`message/send`, no header) at the same URL, and its card carries both versions'
  fields.
- **OAuth 2.1 with PKCE**, the agent's URL as the token's resource (RFC 8707), and the scopes
  `meals:read` and `meals:write`. A 401 names the resource's metadata (RFC 9728) in
  `WWW-Authenticate`, so a client finds the authorization server from the agent's URL alone.
- **Tasks are the caller's own.** A task is seen, answered or cancelled only by the person and the
  app that started it.
- **No invented numbers, honest matches, calm mode kept, stored text is data**: the same rules as
  the [MCP profile](../mcp/README.md#what-a-compatible-server-keeps-to).
- **No agent loop of its own.** Each skill is a deterministic operation on the food log; the only
  model a reference agent calls is the one that reads a sentence, under the person's and the app's
  budgets.
