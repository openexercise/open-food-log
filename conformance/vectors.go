package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"

	openfoodlog "github.com/openexercise/open-food-log"
)

// VectorResult is what an implementation of the specification's section 6 works out for a case:
// the amount as grams and the nutrients it held, or the error code it reports instead of guessing
// (density_unknown, portion_unknown, unit_unknown).
type VectorResult struct {
	Grams     *float64           `json:"grams,omitempty"`
	Nutrients map[string]float64 `json:"nutrients,omitempty"`
	Error     string             `json:"error,omitempty"`
}

// VectorCase is one published case (schema/v0/amount-vectors.json). Input is the case as an
// implementation receives it: its name, the whole food document the case names under "food",
// and the amount; never the expectation.
type VectorCase struct {
	Name  string
	Input json.RawMessage
	Want  VectorResult
}

// vectorTolerance is how near a result must be: the cases are exact, and an implementation that
// rounds what it stores (to two decimal places, say) agrees within it.
const vectorTolerance = 0.01

// AmountVectors reads the published cases, each with its food filled in.
func AmountVectors() ([]VectorCase, error) {
	raw, err := openfoodlog.Schemas.ReadFile("schema/v0/amount-vectors.json")
	if err != nil {
		return nil, err
	}
	var doc struct {
		Foods []json.RawMessage `json:"foods"`
		Cases []struct {
			Name   string          `json:"name"`
			Food   string          `json:"food"`
			Amount json.RawMessage `json:"amount"`
			Expect VectorResult    `json:"expect"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("amount vectors: %w", err)
	}
	foods := map[string]json.RawMessage{}
	for _, f := range doc.Foods {
		var id struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(f, &id); err != nil || id.ID == "" {
			return nil, fmt.Errorf("amount vectors: a food without an id")
		}
		foods[id.ID] = f
	}
	out := make([]VectorCase, 0, len(doc.Cases))
	for _, c := range doc.Cases {
		food, ok := foods[c.Food]
		if !ok {
			return nil, fmt.Errorf("amount vectors: %s names food %s, which isn't listed", c.Name, c.Food)
		}
		input, err := json.Marshal(map[string]any{"name": c.Name, "food": food, "amount": c.Amount})
		if err != nil {
			return nil, err
		}
		out = append(out, VectorCase{Name: c.Name, Input: input, Want: c.Expect})
	}
	return out, nil
}

// VectorReport is every case's outcome, in order.
type VectorReport struct {
	Results []Result `json:"results"`
}

// Passed reports whether every case passed.
func (r VectorReport) Passed() bool {
	return len(r.Results) > 0 && !slices.ContainsFunc(r.Results, func(res Result) bool { return res.Status != Pass })
}

// CheckVectors runs every published case through compute, an implementation of the amount
// calculation, and compares what it works out with the case's expectation. A server's own tests
// call it with their calculation; ofl vectors calls it with a command (the runner contract).
func CheckVectors(ctx context.Context, compute func(ctx context.Context, input json.RawMessage) (VectorResult, error)) (VectorReport, error) {
	cases, err := AmountVectors()
	if err != nil {
		return VectorReport{}, err
	}
	var report VectorReport
	for _, c := range cases {
		res := Result{Name: c.Name, Status: Pass}
		got, err := compute(ctx, c.Input)
		if err == nil {
			err = c.Want.compare(got)
		}
		if err != nil {
			res.Status, res.Detail = Fail, err.Error()
		}
		report.Results = append(report.Results, res)
	}
	return report, nil
}

// compare says how got differs from want. A case expects an error or grams; nutrients are compared
// only for the keys it lists, and a key it lists must be there.
func (want VectorResult) compare(got VectorResult) error {
	if want.Error != "" {
		if got.Error != want.Error {
			return fmt.Errorf("error = %q, want %q", got.Error, want.Error)
		}
		return nil
	}
	if got.Error != "" {
		return fmt.Errorf("error = %q, want grams", got.Error)
	}
	if want.Grams != nil && (got.Grams == nil || !near(*got.Grams, *want.Grams)) {
		return fmt.Errorf("grams = %s, want %g", show(got.Grams), *want.Grams)
	}
	keys := make([]string, 0, len(want.Nutrients))
	for k := range want.Nutrients {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		g, ok := got.Nutrients[k]
		if !ok || !near(g, want.Nutrients[k]) {
			return fmt.Errorf("nutrients.%s = %s, want %g", k, showValue(g, ok), want.Nutrients[k])
		}
	}
	return nil
}

func near(a, b float64) bool { return math.Abs(a-b) <= vectorTolerance+1e-9 }

func show(v *float64) string {
	if v == nil {
		return "none"
	}
	return fmt.Sprint(*v)
}

func showValue(v float64, ok bool) string {
	if !ok {
		return "absent"
	}
	return fmt.Sprint(v)
}
