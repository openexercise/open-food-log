// Command aliases asks Gemini on Vertex AI what people call each catalogue food when they log it,
// and appends each answer to sources/aliases.jsonl, so a run resumes where it stopped and `ofl
// catalogue` merges the answers without calling the model. It sends names only and asks for names
// only: no numbers come from the model.
//
//	OFL_GCP_PROJECT=<project> go run ./cmd/aliases [-limit N]
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/genai"
)

const (
	model    = "gemini-3.8-flash"
	location = "global"
	workers  = 8
	batch    = 40
	source   = "sources/aliases.jsonl"
	foods    = "catalogue/foods.jsonl"
)

const system = `You help build an open food catalogue. For each food below, given by its ID and its USDA
FoodData Central name, list the short names people type or say when logging that they ate it.

Rules:
- Lowercase. At most 5 per food, most common first. An empty list is fine.
- Each must mean this food as named, with its form and preparation where that matters: "hard boiled egg"
  for "Egg, whole, cooked, hard-boiled", not "egg" alone; "oj" for orange juice, not for oranges.
- No brand names, no amounts or sizes, no numbers unless the food's own name has them ("2% milk").
- Don't repeat the USDA name, reworded only by punctuation.
- Plain everyday words a person in the United States would use. Nothing invented.

Answer with one JSON object mapping each ID to its list, and nothing else:
{"ofc:example-id": ["first alias", "second alias"]}`

type entry struct {
	ID      string   `json:"id"`
	Aliases []string `json:"aliases"`
	Model   string   `json:"model"`
	At      string   `json:"at"`
}

func main() {
	limit := flag.Int("limit", 0, "stop after this many foods")
	flag.Parse()
	if err := run(context.Background(), *limit); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, limit int) error {
	project := os.Getenv("OFL_GCP_PROJECT")
	if project == "" {
		return errors.New("OFL_GCP_PROJECT is required")
	}
	done, err := answered()
	if err != nil {
		return err
	}
	todo, err := pending(done)
	if err != nil {
		return err
	}
	if limit > 0 && len(todo) > limit {
		todo = todo[:limit]
	}
	fmt.Printf("%d foods to ask about, %d already answered\n", len(todo), len(done))
	client, err := genai.NewClient(ctx, &genai.ClientConfig{Backend: genai.BackendVertexAI, Project: project, Location: location})
	if err != nil {
		return err
	}
	out, err := os.OpenFile(source, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	var mu sync.Mutex
	var asked, failed atomic.Int64
	jobs := make(chan [][2]string)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for b := range jobs {
				answers, err := ask(ctx, client, b)
				if err != nil {
					failed.Add(int64(len(b)))
					fmt.Fprintf(os.Stderr, "batch from %s failed: %v\n", b[0][0], err)
					continue
				}
				today := time.Now().UTC().Format("2006-01-02")
				mu.Lock()
				for _, f := range b {
					line, _ := json.Marshal(entry{ID: f[0], Aliases: answers[f[0]], Model: model, At: today})
					out.Write(append(line, '\n'))
				}
				mu.Unlock()
				if n := asked.Add(int64(len(b))); n%400 < batch {
					fmt.Printf("%d asked\n", n)
				}
			}
		})
	}
	for i := 0; i < len(todo); i += batch {
		jobs <- todo[i:min(i+batch, len(todo))]
	}
	close(jobs)
	wg.Wait()
	fmt.Printf("%d asked, %d failed (run again to retry them)\n", asked.Load(), failed.Load())
	return nil
}

// answered lists the foods sources/aliases.jsonl already holds.
func answered() (map[string]bool, error) {
	done := map[string]bool{}
	f, err := os.Open(source)
	if errors.Is(err, os.ErrNotExist) {
		return done, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var e entry
		if err := json.Unmarshal(s.Bytes(), &e); err != nil {
			return nil, err
		}
		done[e.ID] = true
	}
	return done, s.Err()
}

// pending lists the catalogue's foods, ID and name, not yet answered.
func pending(done map[string]bool) ([][2]string, error) {
	f, err := os.Open(foods)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out [][2]string
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 1<<20), 1<<20)
	for s.Scan() {
		var r struct{ ID, Name string }
		if err := json.Unmarshal(s.Bytes(), &r); err != nil {
			return nil, err
		}
		if !done[r.ID] {
			out = append(out, [2]string{r.ID, r.Name})
		}
	}
	return out, s.Err()
}

// ask sends one batch and returns the aliases by ID. An answer that leaves out a food, or names
// one it wasn't asked about, is refused, and the batch is tried again.
func ask(ctx context.Context, c *genai.Client, b [][2]string) (map[string][]string, error) {
	var prompt strings.Builder
	for _, f := range b {
		fmt.Fprintf(&prompt, "%s\t%s\n", f[0], f[1])
	}
	var last error
	for attempt := range 5 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*attempt) * 3 * time.Second)
		}
		callCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		resp, err := c.Models.GenerateContent(callCtx, model, genai.Text(prompt.String()), &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(system, genai.RoleUser),
			ResponseMIMEType:  "application/json",
			Temperature:       genai.Ptr[float32](0),
		})
		cancel()
		if err != nil {
			last = err
			continue
		}
		var answers map[string][]string
		if err := json.Unmarshal([]byte(resp.Text()), &answers); err != nil {
			last = fmt.Errorf("not the JSON asked for: %w", err)
			continue
		}
		if len(answers) != len(b) {
			last = fmt.Errorf("answered %d foods of %d", len(answers), len(b))
			continue
		}
		missing := false
		for _, f := range b {
			if _, ok := answers[f[0]]; !ok {
				missing = true
			}
		}
		if missing {
			last = errors.New("the answer's IDs aren't the ones asked about")
			continue
		}
		return answers, nil
	}
	return nil, last
}
