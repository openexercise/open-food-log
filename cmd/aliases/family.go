package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// batchFood is a food as a batch shows it to the model: enough to tell it from its neighbours.
type batchFood struct {
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Category          string   `json:"category,omitempty"`
	HasDefaultPortion bool     `json:"has_default_portion"`
	Aliases           []string `json:"aliases,omitempty"`
}

// family is the foods a person could mean by one plain name: every "Meat loaf …".
type family struct {
	Family string      `json:"family"`
	Foods  []batchFood `json:"foods"`
}

// batchFile is one batch-NNN.json: the families one model call answers.
type batchFile struct {
	Batch    string   `json:"batch"`
	Families []family `json:"families"`
}

var (
	wordBreaks = regexp.MustCompile(`[^\p{L}\p{N}%]+`)
	// clause starts the part of a name's head that narrows the dish rather than naming it.
	clause = regexp.MustCompile(` (made with|with|or|in|from|as) `)
)

// familyOf is the plain name a food's family shares: the head before the first comma, without a
// clause that narrows it, in at most two words. "Meat loaf made with beef" and "Meat loaf dinner,
// NFS, frozen meal" are both "meat loaf"; "Egg omelet or scrambled egg" is "egg omelet".
func familyOf(name string) string {
	head, _, _ := strings.Cut(name, ",")
	head = " " + strings.TrimSpace(wordBreaks.ReplaceAllString(strings.ToLower(head), " ")) + " "
	if loc := clause.FindStringIndex(head); loc != nil && loc[0] > 0 {
		head = head[:loc[0]]
	}
	words := strings.Fields(head)
	if len(words) > 2 {
		words = words[:2]
	}
	return strings.Join(words, " ")
}

// subFamilyOf narrows a family too big for one batch by the name's second part: "Beef, chuck, …"
// is "beef chuck".
func subFamilyOf(name string) string {
	parts := strings.Split(name, ",")
	f := familyOf(name)
	if len(parts) < 2 {
		return f
	}
	return f + " " + strings.TrimSpace(wordBreaks.ReplaceAllString(strings.ToLower(parts[1]), " "))
}

// catalogueRecord is what batches reads of a catalogue line.
type catalogueRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category struct {
		Name string `json:"name"`
	} `json:"category"`
	Portions []struct {
		IsDefault bool `json:"is_default"`
	} `json:"portions"`
	Aliases []struct {
		Name string `json:"name"`
	} `json:"aliases"`
	Provenance struct {
		Generated []string `json:"generated"`
	} `json:"provenance"`
}

// toBatchFood shows a record to the model with the aliases it already has from rules and people,
// so the model doesn't repeat them. A model's earlier aliases aren't shown: they're being replaced.
func toBatchFood(r catalogueRecord) batchFood {
	f := batchFood{ID: r.ID, Name: r.Name, Category: r.Category.Name}
	for _, p := range r.Portions {
		f.HasDefaultPortion = f.HasDefaultPortion || p.IsDefault
	}
	for i, a := range r.Aliases {
		if !slices.Contains(r.Provenance.Generated, fmt.Sprintf("/aliases/%d", i)) {
			f.Aliases = append(f.Aliases, a.Name)
		}
	}
	return f
}

// groupFamilies puts foods into families in catalogue order, then splits any family larger than
// size by its second name part, and what is still too large into chunks.
func groupFamilies(foods []batchFood, size int) []family {
	byKey := map[string]*family{}
	var order []string
	add := func(key string, f batchFood) {
		if byKey[key] == nil {
			byKey[key] = &family{Family: key}
			order = append(order, key)
		}
		byKey[key].Foods = append(byKey[key].Foods, f)
	}
	for _, f := range foods {
		add(familyOf(f.Name), f)
	}
	var out []family
	for _, key := range order {
		fam := *byKey[key]
		if len(fam.Foods) <= size {
			out = append(out, fam)
			continue
		}
		subs := groupBy(fam.Foods, subFamilyOf)
		for _, sub := range subs {
			for i := 0; i < len(sub.Foods); i += size {
				out = append(out, family{Family: sub.Family, Foods: sub.Foods[i:min(i+size, len(sub.Foods))]})
			}
		}
	}
	return out
}

// groupBy groups foods by key, keeping first-seen order.
func groupBy(foods []batchFood, key func(string) string) []family {
	idx := map[string]int{}
	var out []family
	for _, f := range foods {
		k := key(f.Name)
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, family{Family: k})
		}
		out[i].Foods = append(out[i].Foods, f)
	}
	return out
}

// packBatches fills batches of about size foods with whole families.
func packBatches(families []family, size int) [][]family {
	var out [][]family
	var cur []family
	n := 0
	for _, f := range families {
		if n > 0 && n+len(f.Foods) > size {
			out = append(out, cur)
			cur, n = nil, 0
		}
		cur = append(cur, f)
		n += len(f.Foods)
	}
	if n > 0 {
		out = append(out, cur)
	}
	return out
}

// runBatches writes batch-NNN.json files for the foods sources/aliases.jsonl doesn't hold yet.
func runBatches(args []string) error {
	fs := flag.NewFlagSet("batches", flag.ExitOnError)
	out := fs.String("out", "", "directory to write batch-NNN.json files into")
	match := fs.String("match", "", "only families whose name contains this text (several separated by |)")
	name := fs.String("name", "", "only foods whose own name contains this text (several separated by |): a kind of food across families, such as restaurant dishes")
	limit := fs.Int("limit", 0, "stop after this many foods")
	size := fs.Int("size", batch, "foods per batch")
	redo := fs.Bool("redo", false, "ask again about foods sources/aliases.jsonl already holds")
	fs.Parse(args)
	if *out == "" {
		return errors.New("-out is required")
	}
	done := map[string]bool{}
	if !*redo {
		var err error
		if done, err = answered(); err != nil {
			return err
		}
	}
	foods, err := readCatalogue(foods)
	if err != nil {
		return err
	}
	var todo []batchFood
	for _, f := range foods {
		if !done[f.ID] && matches(familyOf(f.Name), *match) && matches(strings.ToLower(f.Name), *name) {
			todo = append(todo, f)
		}
	}
	families := groupFamilies(todo, *size)
	if *limit > 0 {
		families = takeFoods(families, *limit)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	batches := packBatches(families, *size)
	total := 0
	for i, b := range batches {
		name := fmt.Sprintf("batch-%03d", i+1)
		data, err := json.MarshalIndent(batchFile{Batch: name, Families: b}, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(*out, name+".json"), append(data, '\n'), 0o644); err != nil {
			return err
		}
		for _, f := range b {
			total += len(f.Foods)
		}
	}
	fmt.Printf("%d batches, %d foods, in %s\n", len(batches), total, *out)
	return nil
}

// matches says whether a lowercase family or food name is one of those asked for: any of the
// |-separated texts, or all when there are none.
func matches(fam, match string) bool {
	if strings.TrimSpace(match) == "" {
		return true
	}
	for _, m := range strings.Split(match, "|") {
		if m = strings.ToLower(strings.TrimSpace(m)); m != "" && strings.Contains(fam, m) {
			return true
		}
	}
	return false
}

// takeFoods keeps whole families until at least n foods are kept.
func takeFoods(families []family, n int) []family {
	kept := 0
	for i, f := range families {
		if kept >= n {
			return families[:i]
		}
		kept += len(f.Foods)
	}
	return families
}

// readCatalogue reads the catalogue's foods as batches show them.
func readCatalogue(path string) ([]batchFood, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []batchFood
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var r catalogueRecord
		if err := json.Unmarshal(s.Bytes(), &r); err != nil {
			return nil, err
		}
		out = append(out, toBatchFood(r))
	}
	return out, s.Err()
}
