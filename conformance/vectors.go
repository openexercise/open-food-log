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
// the amount as grams and millilitres where they can be known, and the nutrients it held, or the
// error code it reports instead of guessing (density_unknown, portion_unknown, serving_unknown,
// serving_weight_unknown, unit_unknown). A drink known per 100 ml, or a supplement by the
// serving, has no grams, and leaves Grams nil.
type VectorResult struct {
	Grams     *float64           `json:"grams,omitempty"`
	ML        *float64           `json:"ml,omitempty"`
	Nutrients map[string]float64 `json:"nutrients,omitempty"`
	Error     string             `json:"error,omitempty"`
}

// VectorCase is one published case (schema/v0/amount-vectors.json). Input is the case as an
// implementation receives it: its name, the whole food document the case names under "food",
// the amount, and the share of it eaten when the case gives one; never the expectation.
// GramsUnknown is a case expecting grams to stay unknown ("grams": null).
type VectorCase struct {
	Name         string
	Input        json.RawMessage
	Want         VectorResult
	GramsUnknown bool
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
			Share  float64         `json:"share,omitempty"`
			Expect json.RawMessage `json:"expect"`
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
		in := map[string]any{"name": c.Name, "food": food, "amount": c.Amount}
		if c.Share != 0 {
			in["share"] = c.Share
		}
		input, err := json.Marshal(in)
		if err != nil {
			return nil, err
		}
		var want VectorResult
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(c.Expect, &want); err != nil {
			return nil, fmt.Errorf("amount vectors: %s: %w", c.Name, err)
		}
		if err := json.Unmarshal(c.Expect, &fields); err != nil {
			return nil, fmt.Errorf("amount vectors: %s: %w", c.Name, err)
		}
		grams, named := fields["grams"]
		out = append(out, VectorCase{Name: c.Name, Input: input, Want: want, GramsUnknown: named && string(grams) == "null"})
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
			err = c.compare(got)
		}
		if err != nil {
			res.Status, res.Detail = Fail, err.Error()
		}
		report.Results = append(report.Results, res)
	}
	return report, nil
}

// compare says how got differs from the case's expectation. A case expects an error, or grams
// and millilitres where it names them (grams may be expected unknown); nutrients are compared only
// for the keys it lists, and a key it lists must be there.
func (c VectorCase) compare(got VectorResult) error {
	want := c.Want
	if want.Error != "" {
		if got.Error != want.Error {
			return fmt.Errorf("error = %q, want %q", got.Error, want.Error)
		}
		return nil
	}
	if got.Error != "" {
		return fmt.Errorf("error = %q, want a result", got.Error)
	}
	if c.GramsUnknown && got.Grams != nil {
		return fmt.Errorf("grams = %g, want unknown", *got.Grams)
	}
	if want.Grams != nil && (got.Grams == nil || !near(*got.Grams, *want.Grams)) {
		return fmt.Errorf("grams = %s, want %g", show(got.Grams), *want.Grams)
	}
	if want.ML != nil && (got.ML == nil || !near(*got.ML, *want.ML)) {
		return fmt.Errorf("ml = %s, want %g", show(got.ML), *want.ML)
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
