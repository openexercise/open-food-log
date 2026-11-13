// Package openfoodlog carries the Open Food Log schemas and MCP tool profile, a validator for its
// documents, and the reference calculation of the specification's section 6: an amount on a food, as grams, then as
// the nutrients it held.
package openfoodlog

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Schemas holds schema/v0, the documents served at https://openfoodlog.org/schema/v0/.
//
//go:embed schema/v0/*.json
var Schemas embed.FS

// Profile is mcp/tools.json, the MCP tool profile: the tools a compatible server lists, their
// input schemas and annotations, and the error codes their failures start with.
//
//go:embed mcp/tools.json
var Profile []byte

// A2AProfile is a2a/agent-card.json, the A2A agent profile: the reference agent's card, whose
// https://openfoodlog.org/a2a/v0 extension lists the core skills a compatible agent offers.
//
//go:embed a2a/agent-card.json
var A2AProfile []byte

// Kinds are the document kinds, by file name under schema/v0.
var Kinds = []string{"food", "meal", "recipe", "saved-meal", "targets", "log"}

// Validator holds the compiled schemas, keyed by kind and by $id.
type Validator struct {
	byID   map[string]*jsonschema.Schema
	byKind map[string]*jsonschema.Schema
}

// NewValidator compiles the embedded schemas.
func NewValidator() (*Validator, error) { return NewValidatorFS(Schemas) }

// NewValidatorFS compiles every schema under schema/v0 in fsys: a checkout, or the embedded copy.
// Each is registered under its $id, so references between them resolve without the network.
func NewValidatorFS(fsys fs.FS) (*Validator, error) {
	c := jsonschema.NewCompiler()
	ids := map[string]string{}
	for _, kind := range Kinds {
		name := path.Join("schema", "v0", kind+".json")
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		id, _ := doc.(map[string]any)["$id"].(string)
		if id == "" {
			return nil, fmt.Errorf("%s has no $id", name)
		}
		if err := c.AddResource(id, doc); err != nil {
			return nil, fmt.Errorf("add %s: %w", name, err)
		}
		ids[kind] = id
	}
	v := &Validator{byID: map[string]*jsonschema.Schema{}, byKind: map[string]*jsonschema.Schema{}}
	for _, kind := range Kinds {
		s, err := c.Compile(ids[kind])
		if err != nil {
			return nil, fmt.Errorf("compile %s: %w", kind, err)
		}
		v.byKind[kind] = s
		v.byID[ids[kind]] = s
	}
	return v, nil
}

// ValidateAs returns every problem in a document of a kind ("food", "meal", "recipe", "saved-meal",
// "targets", "log"), whatever its $schema says.
func (v *Validator) ValidateAs(kind string, data []byte) []string {
	s, ok := v.byKind[kind]
	if !ok {
		return []string{fmt.Sprintf("unknown kind %q", kind)}
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return []string{"invalid JSON: " + err.Error()}
	}
	return problems(s, doc)
}

// Validate returns every problem in one document, which names its schema in $schema.
func (v *Validator) Validate(data []byte) []string {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return []string{"invalid JSON: " + err.Error()}
	}
	obj, _ := doc.(map[string]any)
	id, _ := obj["$schema"].(string)
	s, ok := v.byID[id]
	if !ok {
		ids := make([]string, 0, len(v.byID))
		for id := range v.byID {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return []string{fmt.Sprintf("$schema %q isn't one of %v", id, ids)}
	}
	return problems(s, doc)
}

func problems(s *jsonschema.Schema, doc any) []string {
	if err := s.Validate(doc); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return []string{ve.Error()}
		}
		return []string{err.Error()}
	}
	return nil
}

// Food is the part of a food document the calculation reads.
type Food struct {
	ID                string             `json:"id"`
	Name              string             `json:"name"`
	NutrientsPer100g  map[string]float64 `json:"nutrients_per_100g"`
	NutrientsPer100ml map[string]float64 `json:"nutrients_per_100ml,omitempty"`
	DensityGPerML     float64            `json:"density_g_per_ml,omitempty"`
	Portions          []Portion          `json:"portions,omitempty"`
	Label             *Label             `json:"label,omitempty"`
}

// Portion is a named measure of a food and what one weighs.
type Portion struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Grams     float64 `json:"grams"`
	IsDefault bool    `json:"is_default,omitempty"`
}

// Label is the part of a food's nutrition label the calculation reads: its declared serving and
// what one serving holds.
type Label struct {
	Serving             *Serving           `json:"serving,omitempty"`
	NutrientsPerServing map[string]float64 `json:"nutrients_per_serving,omitempty"`
}

// Serving is a label's declared serving, with its weight or volume where printed.
type Serving struct {
	Text  string  `json:"text"`
	Grams float64 `json:"grams,omitempty"`
	ML    float64 `json:"ml,omitempty"`
}

// Amount is how much of a food, in the measure the person used.
type Amount struct {
	Value   float64 `json:"value"`
	Unit    string  `json:"unit"`
	Portion string  `json:"portion,omitempty"`
}

// The errors an implementation reports instead of guessing. Their text is the code the
// specification and the amount vectors use.
var (
	ErrDensityUnknown       = errors.New("density_unknown")
	ErrPortionUnknown       = errors.New("portion_unknown")
	ErrServingUnknown       = errors.New("serving_unknown")
	ErrServingWeightUnknown = errors.New("serving_weight_unknown")
	ErrUnitUnknown          = errors.New("unit_unknown")
)

var (
	gramsPer = map[string]float64{"g": 1, "kg": 1000, "oz": 28.349523125, "lb": 453.59237}
	mlPer    = map[string]float64{"ml": 1, "l": 1000, "fl_oz": 29.5735295625}
)

// Measured is an amount worked out: its weight and volume where they can be known (nil where
// not, which isn't an error for a drink known per 100 ml or a supplement), and what it held.
type Measured struct {
	Grams     *float64
	ML        *float64
	Nutrients map[string]float64
}

// Measure is section 6: the amount times share (0 for all of it) becomes grams, millilitres or
// servings, and the nutrients come from the first of the food's sets the amount reaches: per
// 100 g, per 100 ml, then per serving. A density, a portion or a serving is never assumed, and
// nothing is rounded.
func Measure(f Food, a Amount, share float64) (Measured, error) {
	if share == 0 {
		share = 1
	}
	v := a.Value * share
	var m Measured
	var servings *float64
	switch {
	case a.Unit == "portion":
		i := slices.IndexFunc(f.Portions, func(p Portion) bool { return p.ID == a.Portion })
		if i < 0 {
			return m, ErrPortionUnknown
		}
		m.Grams = ptrTo(v * f.Portions[i].Grams)
	case gramsPer[a.Unit] > 0:
		m.Grams = ptrTo(v * gramsPer[a.Unit])
	case mlPer[a.Unit] > 0:
		m.ML = ptrTo(v * mlPer[a.Unit])
	case a.Unit == "serving":
		if f.Label == nil || f.Label.Serving == nil {
			return m, ErrServingUnknown
		}
		servings = ptrTo(v)
		if s := f.Label.Serving; s.Grams > 0 {
			m.Grams = ptrTo(v * s.Grams)
		}
		if s := f.Label.Serving; s.ML > 0 {
			m.ML = ptrTo(v * s.ML)
		}
	default:
		return m, ErrUnitUnknown
	}
	if m.Grams == nil && m.ML != nil && f.DensityGPerML > 0 {
		m.Grams = ptrTo(*m.ML * f.DensityGPerML)
	}
	var perServing map[string]float64
	if f.Label != nil {
		perServing = f.Label.NutrientsPerServing
	}
	switch {
	case m.Grams != nil && len(f.NutrientsPer100g) > 0:
		m.Nutrients = scaled(f.NutrientsPer100g, *m.Grams/100)
	case m.ML != nil && len(f.NutrientsPer100ml) > 0:
		m.Nutrients = scaled(f.NutrientsPer100ml, *m.ML/100)
	case servings != nil && len(perServing) > 0:
		m.Nutrients = scaled(perServing, *servings)
	case len(f.NutrientsPer100ml) > 0 || len(perServing) == 0:
		return m, ErrDensityUnknown
	default:
		return m, ErrServingWeightUnknown
	}
	return m, nil
}

// Grams is an amount of a food as a weight (section 6), or ErrDensityUnknown when it can't be
// weighed. Use Measure for drinks and supplements, which may have no weight.
func Grams(f Food, a Amount) (float64, error) {
	if a.Unit == "serving" || mlPer[a.Unit] > 0 || gramsPer[a.Unit] > 0 || a.Unit == "portion" {
		m, err := Measure(f, a, 0)
		if err != nil && m.Grams == nil {
			return 0, err
		}
		if m.Grams == nil {
			return 0, ErrDensityUnknown
		}
		return *m.Grams, nil
	}
	return 0, ErrUnitUnknown
}

// Nutrients is what grams of a food held: each nutrient the food has, scaled from 100 g. A
// nutrient the food lacks stays absent, never zero. Nothing is rounded.
func Nutrients(f Food, grams float64) map[string]float64 {
	return scaled(f.NutrientsPer100g, grams/100)
}

func scaled(m map[string]float64, by float64) map[string]float64 {
	out := make(map[string]float64, len(m))
	for k, v := range m {
		out[k] = v * by
	}
	return out
}

func ptrTo(v float64) *float64 { return &v }

// Vectors is schema/v0/amount-vectors.json.
type Vectors struct {
	Foods []Food `json:"foods"`
	Cases []struct {
		Name   string  `json:"name"`
		Note   string  `json:"note,omitempty"`
		Food   string  `json:"food"`
		Amount Amount  `json:"amount"`
		Share  float64 `json:"share,omitempty"`
		Expect struct {
			// Grams is nil both when absent and when null (the amount can't be weighed).
			Grams     *float64           `json:"grams,omitempty"`
			ML        *float64           `json:"ml,omitempty"`
			Nutrients map[string]float64 `json:"nutrients,omitempty"`
			Error     string             `json:"error,omitempty"`
		} `json:"expect"`
	} `json:"cases"`
}

// LoadVectors reads the embedded amount vectors.
func LoadVectors() (Vectors, error) {
	var v Vectors
	data, err := Schemas.ReadFile("schema/v0/amount-vectors.json")
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(data, &v)
	return v, err
}
