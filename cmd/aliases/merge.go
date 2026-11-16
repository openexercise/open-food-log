package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/openexercise/open-food-log/internal/alias"
)

const curatedSource = "sources/curated-aliases.jsonl"

// mergeResult is what checking a run's answers found.
type mergeResult struct {
	entries []entry
	// refused names each batch whose answer wasn't used, and why, so it can be asked again.
	refused map[string]string
	// dropped lists single aliases left out, and why.
	dropped []string
}

// checkAnswer holds one answer to its batch: every food answered and no other, and no plain name
// given to two foods of one family. It returns the aliases kept by food ID, and those dropped.
func checkAnswer(b batchFile, answer map[string][]string, taken map[string]string) (map[string][]string, []string, error) {
	asked := map[string]batchFood{}
	for _, fam := range b.Families {
		for _, f := range fam.Foods {
			asked[f.ID] = f
		}
	}
	for id := range answer {
		if _, ok := asked[id]; !ok {
			return nil, nil, fmt.Errorf("answers %s, which it wasn't asked about", id)
		}
	}
	kept := map[string][]string{}
	var dropped []string
	for _, fam := range b.Families {
		holder := map[string]string{}
		for _, f := range fam.Foods {
			list, ok := answer[f.ID]
			if !ok {
				return nil, nil, fmt.Errorf("leaves out %s", f.ID)
			}
			if len(list) > 5 {
				list = list[:5]
			}
			out := []string{}
			for _, a := range list {
				key := normalize(a)
				if other, ok := holder[key]; ok && other != f.ID {
					return nil, nil, fmt.Errorf("gives %q to both %s and %s, of one family", a, other, f.ID)
				}
				holder[key] = f.ID
				if !alias.Acceptable(a, f.Name) {
					dropped = append(dropped, fmt.Sprintf("%s: %q isn't short, lowercase and free of new digits", f.ID, a))
					continue
				}
				if owner, ok := taken[key]; ok && owner != f.ID {
					dropped = append(dropped, fmt.Sprintf("%s: %q already belongs to %s", f.ID, a, owner))
					continue
				}
				if !slices.Contains(out, a) {
					out = append(out, a)
				}
			}
			kept[f.ID] = out
		}
	}
	return kept, dropped, nil
}

// normalize is the catalogue's comparison rule for names (cmd/ofl's normalize).
func normalize(s string) string {
	return strings.TrimSpace(wordBreaks.ReplaceAllString(strings.ToLower(s), " "))
}

// mergeRun checks every answer in dir against its batch. A name two batches gave to different
// foods is kept on both here: building the catalogue keeps each model alias on one food
// (cmd/ofl's soleModelAliases).
func mergeRun(dir, model, at string, taken map[string]string) (mergeResult, error) {
	res := mergeResult{refused: map[string]string{}}
	names, err := filepath.Glob(filepath.Join(dir, "batch-*.json"))
	if err != nil {
		return res, err
	}
	sort.Strings(names)
	kept := map[string][]string{}
	var order []string
	for _, name := range names {
		var b batchFile
		if err := readJSON(name, &b); err != nil {
			return res, err
		}
		answerPath := filepath.Join(dir, strings.Replace(filepath.Base(name), "batch-", "answer-", 1))
		var answer map[string][]string
		if err := readJSON(answerPath, &answer); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				res.refused[b.Batch] = "no answer file"
			} else {
				res.refused[b.Batch] = err.Error()
			}
			continue
		}
		got, dropped, err := checkAnswer(b, answer, taken)
		if err != nil {
			res.refused[b.Batch] = err.Error()
			continue
		}
		res.dropped = append(res.dropped, dropped...)
		for _, fam := range b.Families {
			for _, f := range fam.Foods {
				kept[f.ID] = got[f.ID]
				order = append(order, f.ID)
			}
		}
	}
	for _, id := range order {
		res.entries = append(res.entries, entry{ID: id, Aliases: kept[id], Model: model, At: at})
	}
	return res, nil
}

// readTaken reads the aliases a model mustn't move, by normalized alias, naming the food each
// belongs to: those people chose, and those the catalogue already has from rules ("oatmeal" for
// "Oatmeal, NFS"). A model's earlier aliases aren't among them: a run may replace them.
func readTaken() (map[string]string, error) {
	taken := map[string]string{}
	f, err := os.Open(foods)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var r catalogueRecord
		if err := json.Unmarshal(s.Bytes(), &r); err != nil {
			return nil, err
		}
		for _, a := range toBatchFood(r).Aliases {
			taken[normalize(a)] = r.ID
		}
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(curatedSource)
	if errors.Is(err, os.ErrNotExist) {
		return taken, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var c struct {
			ID      string   `json:"id"`
			Aliases []string `json:"aliases"`
		}
		if err := json.Unmarshal([]byte(line), &c); err != nil {
			return nil, fmt.Errorf("%s: %w", curatedSource, err)
		}
		for _, a := range c.Aliases {
			taken[normalize(a)] = c.ID
		}
	}
	return taken, nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: not the JSON asked for: %w", filepath.Base(path), err)
	}
	return nil
}

// runMerge appends a run's checked answers to sources/aliases.jsonl and lists what it refused.
func runMerge(args []string) error {
	fs := flag.NewFlagSet("merge", flag.ExitOnError)
	in := fs.String("in", "", "directory holding batch-NNN.json and answer-NNN.json")
	model := fs.String("model", "", "the model that answered, as records will name it")
	dry := fs.Bool("dry-run", false, "check the answers without writing")
	fs.Parse(args)
	if *in == "" || *model == "" {
		return errors.New("-in and -model are required")
	}
	taken, err := readTaken()
	if err != nil {
		return err
	}
	res, err := mergeRun(*in, *model, time.Now().UTC().Format("2006-01-02"), taken)
	if err != nil {
		return err
	}
	aliases := 0
	for _, e := range res.entries {
		aliases += len(e.Aliases)
	}
	for _, d := range res.dropped {
		fmt.Printf("dropped %s\n", d)
	}
	refused := make([]string, 0, len(res.refused))
	for b := range res.refused {
		refused = append(refused, b)
	}
	sort.Strings(refused)
	for _, b := range refused {
		fmt.Printf("refused %s: %s\n", b, res.refused[b])
	}
	fmt.Printf("%d foods answered, %d aliases kept, %d dropped, %d batches refused\n", len(res.entries), aliases, len(res.dropped), len(refused))
	if *dry || len(res.entries) == 0 {
		return nil
	}
	out, err := os.OpenFile(source, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	for _, e := range res.entries {
		line, err := json.Marshal(e)
		if err != nil {
			return err
		}
		if _, err := out.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	fmt.Printf("appended to %s\n", source)
	return nil
}
