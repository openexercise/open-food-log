---
name: food-aliases
description: Ask Claude Haiku what people call the Open Food Catalogue's foods, a family of related records at a time, so a dish's plain name ("meatloaf", "omelette") finds one food for sure. Writes sources/aliases.jsonl and refreshes catalogue/foods.jsonl. Use when improving catalogue search or matching with aliases. Do NOT use for nutrient numbers, curated (person-chosen) aliases, or Lifterclub's matcher code.
metadata:
  author: mightydotrun
  version: 1.0.0
---

# Food aliases

Gives catalogue foods the names people log them by. Go writes the questions and checks every
answer (`cmd/aliases batches` and `merge`); Haiku subagents only answer, one batch each. Aliases
land in `sources/aliases.jsonl` with the model named, and the build marks each one in the record's
`provenance.generated`. No person reviews them, so `sources/alias-checks.jsonl` and
`TestAliasChecks` decide whether a run can ship.

## Usage

```
/food-aliases                 # every food sources/aliases.jsonl doesn't hold yet (~350 batches)
/food-aliases "meat loaf"     # only families whose plain name contains the text
/food-aliases "" 200          # the first families, about 200 foods
```

Run from the root of `open-food-log`, on a branch.

## Steps

1. **Batches.** Make a run directory in the scratchpad (`$RUN`), then:

   ```bash
   go run ./cmd/aliases batches -out "$RUN" [-match "<text>"] [-limit N]
   ```

   Add `-redo` to ask again about foods already answered (a later line replaces an earlier one).
   Stop if it reports 0 batches.

2. **Answers.** For each `batch-NNN.json`, spawn an `Agent` with `model: "haiku"` and
   `subagent_type: "general-purpose"`, up to 8 at once in one message, then the next 8. The prompt
   for each is:

   > Read `<repo>/.claude/skills/food-aliases/prompt.md` and follow it. Your batch is
   > `$RUN/batch-NNN.json`. Write your answer, the JSON object only, to `$RUN/answer-NNN.json` with
   > the Write tool. Don't read or write anything else. Reply with one line: done, and how many
   > foods you answered.

   Don't read the batches or answers yourself: merge checks them.

3. **Merge.** Check first, then write:

   ```bash
   go run ./cmd/aliases merge -in "$RUN" -model claude-haiku-5-5 -dry-run
   ```

   For each `refused batch-NNN`, delete its answer and spawn one fresh Haiku agent for it (step 2),
   once. Then run merge again without `-dry-run`. Batches still refused stay unanswered; a later run
   picks them up.

4. **Build and check.**

   ```bash
   go run ./cmd/ofl catalogue refresh
   go run ./cmd/ofl validate
   go test ./...
   ```

   If `TestAliasChecks` fails, the run gave a checked phrase to the wrong food or to several.
   Find the lines in `sources/aliases.jsonl` for the foods it names, delete them, and run steps 1
   to 4 again with `-match` for that family. Don't edit `alias-checks.jsonl` to make a run pass;
   change it only when the check itself was wrong, and say so.

5. **Report**, in a few lines: foods answered, aliases kept and dropped, batches refused, the
   change in `catalogue/REPORT.md`'s alias counts, and the check results. List any alias the
   catalogue now gives to more than one food:

   ```bash
   jq -r '.id as $id | .aliases[]?.name | [., $id] | @tsv' catalogue/foods.jsonl | sort | awk -F'\t' '{c[$1]++; ids[$1]=ids[$1]" "$2} END {for (a in c) if (c[a]>1) print a":"ids[a]}' | head -40
   ```

   Add an entry under `## Unreleased` in `CHANGELOG.md` with the counts. Don't commit unless asked.

## Rules

- Names only go to the model, and names only come back: never nutrients or portions.
- Never write to `sources/curated-aliases.jsonl`: that file is for aliases a person chose.
- Add a line to `sources/alias-checks.jsonl` before changing `prompt.md`, so a change that helps
  one dish and hurts another shows up.
