# Contributing

Thank you for helping. This guide covers how the repository is put together, which parts you can
change here and which come from elsewhere, and what a pull request needs. To read or write Open
Food Log data, or build a server that speaks it, read the
[developer guide](docs/developer-guide.md) instead.

Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md). Report security problems
privately, as [SECURITY.md](SECURITY.md) describes, never in a public issue. How decisions are made
is in [GOVERNANCE.md](GOVERNANCE.md).

## Setting up

You need Go (the version `go.mod` names: 1.26) and git. Nothing else: no database, no services.

```bash
git clone https://github.com/openexercise/open-food-log
cd open-food-log
go run ./cmd/ofl validate            # every example against the schemas
go test ./...                        # schemas, vectors, catalogue, ofl, conformance
(cd clients/go && go test ./...)     # the Go client, a module of its own
```

`go run ./cmd/ofl site dist` builds the website into `dist/` (git ignores it), to see how the
specification and the changelog render.

## Where things come from

Until v1, the source of the format is the private repository of
[Lifterclub](https://lifterclub.com), the reference implementation, and its
`scripts/publish-openfoodlog.sh` copies it into this repository. Anything that script writes is
overwritten on the next publish, so an edit made to it only here is lost.

| Path | Made in | To change it |
| --- | --- | --- |
| `schema/v0/*` (SPEC, schemas, `nutrients.json`, vectors) | Lifterclub, published here | An issue or proposal here ([Changing the format](#changing-the-format)) |
| `schema/CHANGELOG.md`, `schema/README.md` (versions, extension registry) | Lifterclub, published here | The same |
| `examples/*.json` | Lifterclub, published here | The same |
| `api/core.yaml` | Generated in Lifterclub from its server's OpenAPI document | The same |
| `mcp/README.md`, `mcp/tools.json` | Generated in Lifterclub from its MCP server | The same |
| `a2a/README.md`, `a2a/agent-card.json` | Generated in Lifterclub from its A2A agent | The same |
| `clients/go` | Lifterclub, published here | The same |
| `catalogue/` | **Here**, by `ofl catalogue` | A pull request here |
| `cmd/ofl`, `cmd/aliases` | **Here** | A pull request here |
| `conformance/` | **Here** | A pull request here |
| `openfoodlog.go` and the root `*_test.go` files | **Here** | A pull request here |
| `README.md`, `CONTRIBUTING.md`, `GOVERNANCE.md`, `CHANGELOG.md`, `docs/` | **Here** | A pull request here |

A maintainer publishes by running the script from a Lifterclub checkout into a branch of this
repository and opening a pull request, so every published change is still reviewed here. At v1 the
source of truth moves to this repository and the publish step goes away.

## Changing the format

The format is the specification, the schemas, the nutrient vocabulary and the vectors. A change to
it starts here, as an issue, whoever proposes it.

1. **Open an issue** saying what real meals, foods or apps the change serves, with an example
   document. Say whether it's additive (a new optional member, nutrient key, enum value or
   document) or breaking (a renamed or removed member, a changed unit or meaning). Before v1 both
   are possible; after v1 only additive changes land without a new major version.
2. **Wait for comment.** A maintainer decides in the issue after at least a week.
3. **The change is made in Lifterclub and published here** in the next version. You're welcome to
   open a pull request here with the change written as you'd like it to land (schema, SPEC,
   example, vector): it makes the proposal concrete. A maintainer carries it to Lifterclub, and the
   publish brings it back.

A change to the format is complete when it has all of these:

- **The schema** in `schema/v0/`. New members are optional unless the proposal says why not. Objects
  keep `additionalProperties: false`, with `x-` members let through by `patternProperties`.
- **The specification**, `schema/v0/SPEC.md`, saying the same thing in the section it belongs to.
- **An example** in `examples/` that uses it and passes `ofl validate`.
- **Vectors** when it touches a calculation: an amount to grams and nutrients
  (`amount-vectors.json`), a recipe's yield and nutrients (`recipe-vectors.json`), or which target
  applies and its status (`target-vectors.json`). The reference calculation here
  (`openfoodlog.Measure`) and the tests in `calc_test.go` must agree with them.
- **The version**: `ofl_version` moves to the next minor version, and the status line at the top of
  SPEC says so.
- **An entry in `schema/CHANGELOG.md`**, marked **Breaking:** when it is.
- **The API core and the MCP and A2A profiles**, when servers must carry the new member. Those are
  generated from Lifterclub's server, so they change with its code.

### Rules for the format

- Record what a person stated or a source gave. A value a server would have to invent doesn't belong
  in the format; inferred values are listed in `estimates` (on a food, `provenance.generated`).
- Every nutrient key ends in its unit, and a nutrient that isn't known is left out, never zero.
- A new nutrient key names its INFOODS tagname, FoodData Central nutrient number, and Health Connect
  and HealthKit fields, checked against their published references, with `null` where there is no
  exact match. `TestNutrientVocabularyMatchesTheSchema` keeps `nutrients.json` and the schema's list
  the same.
- Provenance (`source`, `external_id`) travels with the record, and an item keeps the person's words
  (`text`) beside its match.
- A minor version only adds: a document valid under 0.n stays valid under 0.n+1.

### Registering an extension prefix

Producers name their own members and nutrient keys `x-<vendor>-…` (SPEC section 13). To register a
vendor, open a pull request adding a row to the Extensions table in `schema/README.md`: the prefix,
who you are and a contact. That file is published from Lifterclub, so a maintainer carries the row
over and the next publish includes it. An extension more than one producer needs is a candidate for
the core: propose it as a format change.

## Contributing to the catalogue

The Open Food Catalogue (`catalogue/foods.jsonl`, 12,442 generic foods with `ofc:` IDs) is built here
from USDA FoodData Central, so catalogue contributions happen in this repository. They are dedicated
to the public domain under [CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/), like the
catalogue and its source.

**Never edit `foods.jsonl` or `REPORT.md` by hand.** `ofl catalogue` writes them. Change the generator
or its inputs, rebuild, and commit the generator, the rebuilt files and the tests together.

### Reporting a wrong record

Open an issue with the food's `ofc:` ID, its `identifiers.fdc_id`, what is wrong, and where you
checked. Start with the FoodData Central page named in the record's `provenance.sources`. The
catalogue copies FoodData Central's numbers: when it agrees with FoodData Central, the fix is a rule
in the generator, or a report to USDA, not a different number here.

### Rebuilding from FoodData Central

1. Download the three JSON datasets from
   [FoodData Central's downloads](https://fdc.nal.usda.gov/download-datasets) into one directory and
   unzip them. The generator reads these files (`sources` in `cmd/ofl/catalogue.go`):
   - `FoodData_Central_foundation_food_json_2026-10-29.json` (Foundation Foods)
   - `surveyDownload.json` (FNDDS survey foods)
   - `FoodData_Central_sr_legacy_food_json_2018-04.json` (SR Legacy)
2. Rebuild and test:

   ```bash
   go run ./cmd/ofl catalogue <that directory>
   go test ./...
   ```

3. Read the diff of `catalogue/REPORT.md` first. It counts what each source kept and left out, which
   nutrients the records carry, and the liquids without a density. A change there you didn't expect
   is a change you didn't mean.

A new FoodData Central release means changing the file names and release labels in `sources`, then
rebuilding; say in the changelog which numbers moved.

The rules the generator follows (one record per name, which nutrient numbers are read, which keys
are worked out, what is left out, how IDs, portions and densities are made) are written in
[catalogue/README.md](catalogue/README.md). A change to a rule changes that README in the same pull
request. The code is in `cmd/ofl`: `catalogue.go` (reading and writing), `worked.go` (`energy_kj`,
`salt_g`, `carbohydrate_available_g` and categories), `density.go` (which foods are liquids, and
their density) and `aliases.go`. Each has tests beside it: add a case for the food that prompted the
change. `TestCatalogue` checks every record: valid against the food schema, IDs in order and used
once, portions with unique IDs and at most one default, and a source for every density.

### IDs

An `ofc:` ID is the food's FoodData Central name in lower case, with runs of other characters made
`-` (`Bananas, raw` is `ofc:bananas-raw`). Until v1, IDs may change. From v1, a published ID never
changes meaning and is never reused: a food that goes away gets `status: replaced` and
`replaced_by`, and the record that replaces it lists the old ID in `previous_ids`.

### Aliases

Aliases are what people call a food. All 1,680 today come from the shape of names, with no model
("Apple, raw" is also "raw apple"; "X, NFS" is also "x"). `cmd/aliases` can ask a model (Gemini on
Vertex AI) for more:

```bash
OFL_GCP_PROJECT=<project> go run ./cmd/aliases
```

It sends food names only, and appends the answers to `sources/aliases.jsonl` with the model and
date, which `ofl catalogue` reads so a build never calls the model. That pass hasn't been run, so
`sources/` doesn't exist yet. Aliases from a model are listed in each record's
`provenance.generated`.

A place for people to add or correct aliases, which the generator merges in, is still to come
(`CHANGELOG.md`, Still to do). Until then, propose aliases in an issue, with the `ofc:` IDs.

### Review

No record has been checked by a person, and each says so (`provenance.review: "unreviewed"`).
Checking records and aliases against their sources is the most useful thing to contribute. A place to
record reviews is still to come; until then, report what you checked in an issue, record by record,
so the work carries over when it exists.

### What the catalogue doesn't take

- Branded or packaged products. They are named by barcode (`gtin:`) and looked up where they're kept.
- Records from a database whose licence requires derived databases to share its terms (Open Food
  Facts is ODbL). The catalogue is CC0 and stays that way.
- Values worked out from a name (allergens, diet flags, glycemic values). No CC0 source in hand
  states them, and guessing would be inventing data.

## Changing the tools and the conformance suite

`cmd/ofl`, `openfoodlog.go` and `conformance/` are made here.

### The `ofl` command and the package

- `ofl validate` walks its paths (`examples/` by default) for `.json` files and validates each
  against the schema its `$schema` names. In a checkout it reads `schema/v0/` from disk; elsewhere it
  uses the schemas embedded in the binary.
- `openfoodlog.Measure` is the reference calculation of SPEC section 6. `TestAmountVectors` and
  `TestMeasureAgrees` hold it to the amount vectors; `TestRecipeVectors` and `TestTargetVectors` do
  the same for recipes and targets; `TestExamplesAddUp` checks that every example's grams and
  nutrients follow from its foods and amounts; `TestLabelsAgree` checks label foods against their
  per-100 g values; `TestSchemasRefuse` checks the schemas reject what they should.

### Adding or changing a conformance check

The suite has three scripts: REST (`conformance.go`, `again.go`), MCP (`mcp.go`) and A2A (`a2a.go`).
Each runs a fixed list of named checks in order (`Checks`, `MCPChecks`, `A2AChecks`); a check that
needs an earlier one's result skips when that one failed.

A new REST or MCP check needs:

- **Its name in the list**, at the point where it runs.
- **The behaviour in the in-memory fake** in the tests (`reference` in `conformance_test.go`, the
  food log in `mcp_test.go`), so `TestReferenceServerPasses` and `TestFoodLogPassesMCP` still pass.
- **A broken behaviour it catches**: a flag on the fake that turns the behaviour off, and a case in
  `TestEachCheckCatchesItsBehaviour`, `TestV02ChecksCatchTheirBehaviour` or
  `TestMCPChecksCatchTheirBehaviour` saying exactly which checks fail. A check no broken server fails
  proves nothing.
- **Cleanup.** A run writes to a real person's account, so it deletes what it made, also when a later
  check fails.
- **A published requirement.** A check tests only behaviour that `api/core.yaml`, `mcp/tools.json` or
  the A2A profile requires. Something new there is a format or profile change first.

The A2A script has no in-memory fake yet: its tests here cover the card and configuration, and the
whole script runs against Lifterclub's agent in Lifterclub's tests.

Lifterclub pins this module and runs all three scripts in its own tests, so a new check is released
in a tag and Lifterclub moves to it. Say in the changelog when a server that passed the last release
must do more to pass this one.

## Before a pull request

Run what CI runs (`.github/workflows/ci.yml`):

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
(cd clients/go && go vet ./... && go test ./...)
go run ./cmd/ofl validate
go run ./cmd/ofl site dist
```

## Commits, pull requests and the changelog

- Work on a branch and open a pull request; nothing is committed to `main` directly.
- Commit messages are conventional commits, `type(scope): description`: types such as `feat`, `fix`,
  `docs`, `refactor`, `test`, `chore` and `ci`, scopes such as `catalogue`, `conformance`, `schema`
  or `api` (see `git log`). No attribution trailers.
- Every change someone would notice adds an entry under `## Unreleased` in
  [CHANGELOG.md](CHANGELOG.md), in the same pull request: one or two lines saying what changed and
  why it matters, marked **Added**, **Changed**, **Fixed** or **Removed** as the file does. Skip pure
  refactors, tests and typos.
- Keep `## Still to do` current: add what you leave undone, remove what ships.
- `CHANGELOG.md` covers this repository. The format's own changelog, `schema/CHANGELOG.md`, is
  published from Lifterclub; don't edit it here.

## Releases

Releases are git tags `vX.Y.Z` on this repository with a GitHub release, by semantic versioning;
there is no version file. A maintainer renames `## Unreleased` in `CHANGELOG.md` to
`## vX.Y.Z — YYYY-MM-DD` with a one-line summary, starts a fresh empty `## Unreleased`, merges, and
tags the merge.

Tags number the package, the `ofl` command, the conformance suite and the catalogue. The format's
version is separate: `ofl_version` (`"0.2"` today), recorded in `schema/CHANGELOG.md`. The Go client
in `clients/go` is a module of its own and would take tags named `clients/go/vX.Y.Z`; none is
published yet.
