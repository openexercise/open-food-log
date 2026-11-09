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

// aliasSource is where cmd/aliases keeps the model's answers.
const aliasSource = "sources/aliases.jsonl"

var (
	phraseBreaks = regexp.MustCompile(`[^\p{L}\p{N}%]+`)
	// simpleModifier is a second part a rule can put in front of the food: one or two plain words
	// ("raw", "baked", "dried"), not a clause ("with skin", "from frozen").
	simpleModifier = regexp.MustCompile(`^[a-z]+( [a-z]+)?$`)
	notAModifier   = map[string]bool{"nfs": true, "ns": true, "with": true, "without": true, "from": true, "and": true, "or": true, "made": true, "type": true}
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
	f, err := os.Open(filepath.Join(root, aliasSource))
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]modelAliases{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]modelAliases{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for line := 1; scanner.Scan(); line++ {
		var m modelAliases
		if err := json.Unmarshal(scanner.Bytes(), &m); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", aliasSource, line, err)
		}
		out[m.ID] = m
	}
	return out, scanner.Err()
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

// attachAliases gives a food its rule aliases, then the model's, leaving out repeats and the
// name itself, and records the model's in provenance.generated.
func attachAliases(food *catalogueFood, model map[string]modelAliases) (rules, generated int) {
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
			rules++
		}
	}
	m, ok := model[food.ID]
	if !ok {
		return rules, 0
	}
	for _, a := range m.Aliases {
		if acceptableAlias(a, food.Name) && add(a) {
			food.Provenance.Generated = append(food.Provenance.Generated, fmt.Sprintf("/aliases/%d", len(food.Aliases)-1))
			generated++
		}
	}
	if generated > 0 {
		// The model is a source of these values, so it's named among the sources.
		food.Provenance.Sources = append(food.Provenance.Sources, sourceDoc{Name: "Aliases suggested by " + m.Model, RetrievedAt: m.At})
	}
	return rules, generated
}
