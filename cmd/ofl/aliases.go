package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/openexercise/open-food-log/internal/alias"
)

// aliasDoc is an alias as a food record carries it.
type aliasDoc struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

// modelAliases is one line of sources/aliases.jsonl: what the model answered for a food, kept so
// building the catalogue never calls it.
type modelAliases struct {
	ID      string   `json:"id"`
	Aliases []string `json:"aliases"`
	Model   string   `json:"model"`
	At      string   `json:"at"`
}

// curatedAliases is one line of sources/curated-aliases.jsonl: what a person decided a food is
// called, and why that food rather than its neighbours ("egg" is the survey's cooked egg, not
// "Egg, creamed", which only sorts first).
type curatedAliases struct {
	ID      string   `json:"id"`
	Aliases []string `json:"aliases"`
	Why     string   `json:"why"`
}

const (
	// aliasSource is where cmd/aliases keeps the model's answers.
	aliasSource = "sources/aliases.jsonl"
	// curatedSource is where people keep the aliases they chose, merged into every build.
	curatedSource = "sources/curated-aliases.jsonl"
	// withheldSource lists names a person decided no single food should hold, so a model's alias by
	// that name is kept off every food and a matcher asks.
	withheldSource = "sources/withheld-aliases.jsonl"
	// curatedSourceName names curated aliases among a record's sources.
	curatedSourceName = "Aliases chosen by a person (sources/curated-aliases.jsonl)"
	// modelSourcePrefix starts the name of a model's aliases among a record's sources.
	modelSourcePrefix = "Aliases suggested by "
)

var (
	phraseBreaks = regexp.MustCompile(`[^\p{L}\p{N}%]+`)
	// simpleModifier is a second part a rule can put in front of the food: one or two plain words
	// ("raw", "baked", "dried"), not a clause ("with skin", "from frozen").
	simpleModifier = regexp.MustCompile(`^[a-z]+( [a-z]+)?$`)
	notAModifier   = map[string]bool{"nfs": true, "ns": true, "with": true, "without": true, "from": true, "and": true, "or": true, "made": true, "type": true, "no": true, "not": true, "school": true, "home": true}
)

// normalize is the comparison rule for names: lowercase, every run of other characters one space.
// It's the Open Food Log's version of Lifterclub's normalize_phrase.
func normalize(s string) string {
	return strings.TrimSpace(phraseBreaks.ReplaceAllString(strings.ToLower(s), " "))
}

// ruleAliases are what a name's own shape gives, with no model: "Apple, raw" is also "raw apple",
// and "Oatmeal, NFS" ("not further specified") is "oatmeal".
func ruleAliases(name string) []string {
	parts := strings.Split(name, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	if len(parts) != 2 || parts[0] == "" {
		return nil
	}
	head, mod := strings.ToLower(parts[0]), strings.ToLower(parts[1])
	if mod == "nfs" || mod == "ns" {
		return []string{head}
	}
	if !simpleModifier.MatchString(mod) || notAModifier[strings.Fields(mod)[0]] {
		return nil
	}
	return []string{mod + " " + head}
}

// readModelAliases reads sources/aliases.jsonl, if there is one, by food ID. A later line for a
// food replaces an earlier one.
func readModelAliases(root string) (map[string]modelAliases, error) {
	return readByID(root, aliasSource, func(m modelAliases) string { return m.ID })
}

// readCuratedAliases reads sources/curated-aliases.jsonl, if there is one, by food ID. A later
// line for a food replaces an earlier one.
func readCuratedAliases(root string) (map[string]curatedAliases, error) {
	return readByID(root, curatedSource, func(c curatedAliases) string { return c.ID })
}

// withheldAlias is one line of sources/withheld-aliases.jsonl: a name too vague for one food, and
// why ("baby peaches": strained or diced, a person should say which).
type withheldAlias struct {
	Alias string `json:"alias"`
	Why   string `json:"why"`
}

// readWithheld reads sources/withheld-aliases.jsonl, if there is one, as a set of normalized names.
func readWithheld(root string) (map[string]bool, error) {
	byAlias, err := readByID(root, withheldSource, func(w withheldAlias) string { return normalize(w.Alias) })
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for k, w := range byAlias {
		if k == "" || strings.TrimSpace(w.Why) == "" {
			return nil, fmt.Errorf("%s: %q needs an alias and a why", withheldSource, w.Alias)
		}
		out[k] = true
	}
	return out, nil
}

// checkWithheld refuses a name both withheld and given to a food by a person: the two disagree.
func checkWithheld(withheld map[string]bool, curated map[string]curatedAliases) error {
	for id, c := range curated {
		for _, a := range c.Aliases {
			if withheld[normalize(a)] {
				return fmt.Errorf("%s: %q is withheld but %s gives it to %s", withheldSource, a, curatedSource, id)
			}
		}
	}
	return nil
}

// readByID reads a JSON Lines file under root, keyed by each line's ID; a missing file is empty.
func readByID[T any](root, name string, id func(T) string) (map[string]T, error) {
	f, err := os.Open(filepath.Join(root, name))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]T{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]T{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for line := 1; scanner.Scan(); line++ {
		if len(strings.TrimSpace(scanner.Text())) == 0 {
			continue
		}
		var v T
		if err := json.Unmarshal(scanner.Bytes(), &v); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", name, line, err)
		}
		out[id(v)] = v
	}
	return out, scanner.Err()
}

// checkCurated refuses curated aliases a build would drop: a food the catalogue doesn't hold, or
// an alias the catalogue wouldn't keep from a model either.
func checkCurated(curated map[string]curatedAliases, names map[string]string) error {
	for id, c := range curated {
		name, ok := names[id]
		if !ok {
			return fmt.Errorf("%s: %s isn't in the catalogue", curatedSource, id)
		}
		for _, a := range c.Aliases {
			if !alias.Acceptable(a, name) {
				return fmt.Errorf("%s: %q for %s: an alias is short, lowercase, and has no digit the name lacks", curatedSource, a, id)
			}
		}
	}
	return nil
}

// aliasCounts are how many aliases a build attached, by where they came from.
type aliasCounts struct{ rules, curated, generated int }

func (c *aliasCounts) add(o aliasCounts) {
	c.rules += o.rules
	c.curated += o.curated
	c.generated += o.generated
}

func (c aliasCounts) total() int { return c.rules + c.curated + c.generated }

// attachAliases gives a food its rule aliases, then those a person chose, then the model's,
// leaving out repeats and the name itself, and records the model's in provenance.generated.
func attachAliases(food *catalogueFood, curated map[string]curatedAliases, model map[string]modelAliases) (n aliasCounts) {
	seen := map[string]bool{normalize(food.Name): true}
	add := func(alias string) bool {
		n := normalize(alias)
		if n == "" || seen[n] {
			return false
		}
		seen[n] = true
		food.Aliases = append(food.Aliases, aliasDoc{Name: alias, Kind: "common"})
		return true
	}
	for _, a := range ruleAliases(food.Name) {
		if add(a) {
			n.rules++
		}
	}
	if c, ok := curated[food.ID]; ok {
		for _, a := range c.Aliases {
			if alias.Acceptable(a, food.Name) && add(a) {
				n.curated++
			}
		}
		if n.curated > 0 {
			food.Provenance.Sources = append(food.Provenance.Sources, sourceDoc{Name: curatedSourceName})
		}
	}
	m, ok := model[food.ID]
	if !ok {
		return n
	}
	for _, a := range m.Aliases {
		if alias.Acceptable(a, food.Name) && add(a) {
			food.Provenance.Generated = append(food.Provenance.Generated, fmt.Sprintf("/aliases/%d", len(food.Aliases)-1))
			n.generated++
		}
	}
	if n.generated > 0 {
		// The model is a source of these values, so it's named among the sources.
		food.Provenance.Sources = append(food.Provenance.Sources, sourceDoc{Name: modelSourcePrefix + m.Model, RetrievedAt: m.At})
	}
	return n
}

// dropAliases takes off everything attachAliases gives, so aliases can be attached again.
func dropAliases(food *catalogueFood) {
	food.Aliases = nil
	var generated []string
	for _, g := range food.Provenance.Generated {
		if !strings.HasPrefix(g, "/aliases/") {
			generated = append(generated, g)
		}
	}
	food.Provenance.Generated = generated
	var sources []sourceDoc
	for _, s := range food.Provenance.Sources {
		if s.Name != curatedSourceName && !strings.HasPrefix(s.Name, modelSourcePrefix) {
			sources = append(sources, s)
		}
	}
	food.Provenance.Sources = sources
}

// aliasReport is REPORT.md's line on aliases.
func aliasReport(foods, withAliases int, n aliasCounts) string {
	return fmt.Sprintf("%d foods in `foods.jsonl`, %d with aliases: %d from the names' own shape, %d chosen by a person (`%s`), %d suggested by a model (listed in each record's `provenance.generated`).", foods, withAliases, n.rules, n.curated, curatedSource, n.generated)
}

var aliasReportLine = regexp.MustCompile("(?m)^\\d+ foods in `foods\\.jsonl`, .*$")

// unspecified marks a record USDA describes as the food with nothing further said: what a person
// means by the plain name.
var unspecified = regexp.MustCompile(`(?i)(^|, )(nfs|ns as to )`)

// preferredFirst orders the foods that could keep a plain name, best first, by the rule the model
// is asked to follow: the record USDA leaves unspecified, then one with a default portion (an
// everyday survey food), then the shortest name, then the ID.
func preferredFirst(a, b *catalogueFood) int {
	if ua, ub := unspecified.MatchString(a.Name), unspecified.MatchString(b.Name); ua != ub {
		if ua {
			return -1
		}
		return 1
	}
	if da, db := hasDefaultPortion(a), hasDefaultPortion(b); da != db {
		if da {
			return -1
		}
		return 1
	}
	if len(a.Name) != len(b.Name) {
		return len(a.Name) - len(b.Name)
	}
	return strings.Compare(a.ID, b.ID)
}

func hasDefaultPortion(f *catalogueFood) bool {
	for _, p := range f.Portions {
		if p.IsDefault {
			return true
		}
	}
	return false
}

// soleModelAliases keeps each of a model's aliases on one food only, so the plain names a model
// gives are names a matcher can be sure of. A model asked about one family at a time can give
// "meatloaf" to a meat loaf and to a beef record in another family, or "french fries" to a fast
// food and a home version. An alias that is already another food's name, or one a rule or a
// person gave another food, is left to that food; otherwise the foods asked about share it and
// preferredFirst picks one. A withheld name stays on none of them. It returns the model's answers
// with the others' copies taken out.
func soleModelAliases(foods []*catalogueFood, curated map[string]curatedAliases, model map[string]modelAliases, withheld map[string]bool) map[string]modelAliases {
	fixed := map[string]string{} // a name, rule or curated alias: whose it is
	for _, f := range foods {
		phrases := append([]string{f.Name}, ruleAliases(f.Name)...)
		phrases = append(phrases, curated[f.ID].Aliases...)
		for _, p := range phrases {
			if _, ok := fixed[normalize(p)]; !ok {
				fixed[normalize(p)] = f.ID
			}
		}
	}
	byID := map[string]*catalogueFood{}
	for _, f := range foods {
		byID[f.ID] = f
	}
	claims := map[string][]*catalogueFood{}
	for id, m := range model {
		f, ok := byID[id]
		if !ok {
			continue
		}
		for _, a := range m.Aliases {
			k := normalize(a)
			if !slices.Contains(claims[k], f) {
				claims[k] = append(claims[k], f)
			}
		}
	}
	keeper := map[string]string{}
	for k, holders := range claims {
		if withheld[k] {
			continue
		}
		if owner, ok := fixed[k]; ok {
			keeper[k] = owner
			continue
		}
		keeper[k] = slices.MinFunc(holders, preferredFirst).ID
	}
	out := make(map[string]modelAliases, len(model))
	for id, m := range model {
		kept := m
		kept.Aliases = nil
		for _, a := range m.Aliases {
			if keeper[normalize(a)] == id {
				kept.Aliases = append(kept.Aliases, a)
			}
		}
		out[id] = kept
	}
	return out
}
