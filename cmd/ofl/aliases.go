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
	"strings"
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
			if !acceptableAlias(a, name) {
				return fmt.Errorf("%s: %q for %s: an alias is short, lowercase, and has no digit the name lacks", curatedSource, a, id)
			}
		}
	}
	return nil
}

// acceptableAlias holds a model's alias to what the catalogue keeps: a short lowercase phrase,
// with a digit only where the food's own name has it ("2% milk" for "Milk, reduced fat (2%)").
func acceptableAlias(alias, name string) bool {
	if alias != strings.ToLower(alias) || len(alias) < 2 || len(alias) > 60 || strings.TrimSpace(alias) != alias {
		return false
	}
	for _, r := range alias {
		if r >= '0' && r <= '9' && !strings.ContainsRune(name, r) {
			return false
		}
	}
	return true
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
			if acceptableAlias(a, food.Name) && add(a) {
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
		if acceptableAlias(a, food.Name) && add(a) {
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
