package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	openfoodlog "github.com/openexercise/open-food-log"
)

// calculate works a case out with the package's own calculation, as a server's tests would with
// theirs.
func calculate(_ context.Context, input json.RawMessage) (VectorResult, error) {
	var c struct {
		Food   openfoodlog.Food   `json:"food"`
		Amount openfoodlog.Amount `json:"amount"`
		Share  float64            `json:"share"`
	}
	if err := json.Unmarshal(input, &c); err != nil {
		return VectorResult{}, err
	}
	m, err := openfoodlog.Measure(c.Food, c.Amount, c.Share)
	if err != nil {
		return VectorResult{Error: err.Error()}, nil
	}
	return VectorResult{Grams: m.Grams, ML: m.ML, Nutrients: m.Nutrients}, nil
}

func TestCheckVectors(t *testing.T) {
	report, err := CheckVectors(context.Background(), calculate)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range report.Results {
		if r.Status != Pass {
			t.Errorf("%s: %s", r.Name, r.Detail)
		}
	}
	if !report.Passed() {
		t.Error("the reference calculation disagrees with the published cases")
	}
}

// Rounding to two decimal places, as a server storing grams and nutrients might, still agrees.
func TestCheckVectorsAllowsRounding(t *testing.T) {
	round := func(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }
	report, err := CheckVectors(context.Background(), func(ctx context.Context, input json.RawMessage) (VectorResult, error) {
		got, err := calculate(ctx, input)
		if got.Grams != nil {
			g := round(*got.Grams)
			got.Grams = &g
		}
		for k, v := range got.Nutrients {
			got.Nutrients[k] = round(v)
		}
		return got, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed() {
		t.Errorf("rounded results fail: %+v", report.Results)
	}
}

// Guessing where the specification says to refuse, getting a number wrong, leaving a nutrient
// out or failing outright each fail the cases they touch.
func TestCheckVectorsCatchesMistakes(t *testing.T) {
	cases := map[string]func(VectorResult) VectorResult{
		"assumes a density": func(r VectorResult) VectorResult {
			if r.Error == openfoodlog.ErrDensityUnknown.Error() {
				g := 100.0
				return VectorResult{Grams: &g}
			}
			return r
		},
		"weighs a drink": func(r VectorResult) VectorResult {
			if r.Grams == nil && r.ML != nil {
				g := *r.ML
				r.Grams = &g
			}
			return r
		},
		"ml off": func(r VectorResult) VectorResult {
			if r.ML != nil {
				ml := *r.ML + 1
				r.ML = &ml
			}
			return r
		},
		"grams off": func(r VectorResult) VectorResult {
			if r.Grams != nil {
				g := *r.Grams + 1
				r.Grams = &g
			}
			return r
		},
		"no protein": func(r VectorResult) VectorResult {
			delete(r.Nutrients, "protein_g")
			return r
		},
	}
	for name, breaks := range cases {
		t.Run(name, func(t *testing.T) {
			report, err := CheckVectors(context.Background(), func(ctx context.Context, input json.RawMessage) (VectorResult, error) {
				got, err := calculate(ctx, input)
				return breaks(got), err
			})
			if err != nil {
				t.Fatal(err)
			}
			if report.Passed() {
				t.Error("passed")
			}
		})
	}
	report, err := CheckVectors(context.Background(), func(context.Context, json.RawMessage) (VectorResult, error) {
		return VectorResult{}, errors.New("not built")
	})
	if err != nil || report.Passed() || len(report.Results) == 0 {
		t.Errorf("an implementation that fails every case: %v %+v", err, report)
	}
}
