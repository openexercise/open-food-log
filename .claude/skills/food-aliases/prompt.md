# What people call these foods

You help build the Open Food Catalogue, an open list of generic foods. People log meals by typing
or saying what they ate ("meatloaf", "an omelette", "oj"), and an app finds the food by its name
or one of its aliases. USDA names foods its own way ("Meat loaf made with beef"), so people's words
often find nothing, or find too many foods to be sure.

You'll get one batch file. It holds **families**: foods that share a plain name, such as every
"Meat loaf …" record. Each food has its `id`, its USDA `name`, its `category`, whether it has a
default portion (`has_default_portion`: the amount a survey assumes when someone names the food
without one), and the `aliases` it already has.

For each food, list the short names people in the United States type or say when they log that
food.

## Rules

- Lowercase. At most 5 per food, most common first. An empty list is fine, and is the right
  answer for most foods in a large family.
- Each alias must mean this food as named, with its form and preparation where that matters:
  "hard boiled egg" for "Egg, whole, cooked, hard-boiled", not "egg" alone; "oj" for orange juice,
  not for oranges.
- No brand names. No amounts or sizes. No numbers unless the food's own name has them ("2% milk").
- Don't repeat the USDA name reworded only by punctuation, and don't repeat an alias the food
  already has.
- Plain everyday words. Spelling variants people really use are welcome ("meatloaf" and
  "meat loaf", "omelet" and "omelette", "mac and cheese"). Nothing invented.

## The plain name goes to one food

An app is sure of a word only when one food alone holds it. If two foods both say "meatloaf", the
person has to pick. So within a family:

- The **plain name** of the dish or food ("meatloaf", "lasagna", "pancakes"), and its spelling
  variants, go to exactly **one** food: the one a person means when they give no detail.
  - Prefer the record USDA marks as unspecified ("NS as to type of meat", "NFS"), when there is
    one and it isn't a frozen meal.
  - Otherwise prefer a food with a default portion, in its most common everyday form: beef for
    meatloaf, cooked rather than raw for a dish, plain rather than with sauce.
  - A frozen meal, a dinner, a baby food, a school lunch or a restaurant's version never gets the
    plain name.
- The other foods get names that say how they differ: "turkey meatloaf", "beef and pork meatloaf",
  "meatloaf with tomato sauce", "frozen meatloaf dinner".
- Never give one alias to two foods in the batch.
- If a family's foods are all different things that only share a first word (raw cuts of beef,
  say), give the plain word to none of them.

## Answer

Write one JSON object mapping **every** food ID in the batch to its list, and nothing else: no
prose, no code fence.

```json
{"ofc:meat-loaf-ns-as-to-type-of-meat": ["meatloaf", "meat loaf"], "ofc:meat-loaf-made-with-chicken-or-turkey": ["turkey meatloaf", "chicken meatloaf"], "ofc:meat-loaf-made-with-ham": ["ham loaf"]}
```
