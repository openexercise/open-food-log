// Package openfoodlog carries the Open Food Log schemas, a validator for its documents, and the
// reference calculation of the specification's section 6: an amount on a food, as grams, then as
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

// Kinds are the document kinds, by file name under schema/v0.
var Kinds = []string{"food", "meal", "log"}

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

// ValidateAs returns every problem in a document of a kind ("food", "meal", "log"), whatever its
// $schema says.
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
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	NutrientsPer100g map[string]float64 `json:"nutrients_per_100g"`
	DensityGPerML    float64            `json:"density_g_per_ml,omitempty"`
	Portions         []Portion          `json:"portions,omitempty"`
}

// Portion is a named measure of a food and what one weighs.
type Portion struct {
	ID        string  `json:"id"`
	Label     string  `json:"label"`
	Grams     float64 `json:"grams"`
	IsDefault bool    `json:"is_default,omitempty"`
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
	ErrDensityUnknown = errors.New("density_unknown")
	ErrPortionUnknown = errors.New("portion_unknown")
	ErrUnitUnknown    = errors.New("unit_unknown")
)

var (
	gramsPer = map[string]float64{"g": 1, "kg": 1000, "oz": 28.349523125, "lb": 453.59237}
	mlPer    = map[string]float64{"ml": 1, "l": 1000, "fl_oz": 29.5735295625}
)

// Grams is an amount of a food as a weight (section 6). A volume needs the food's density and a
// portion must be one of the food's; neither is ever assumed.
func Grams(f Food, a Amount) (float64, error) {
	if a.Unit == "portion" {
		i := slices.IndexFunc(f.Portions, func(p Portion) bool { return p.ID == a.Portion })
		if i < 0 {
			return 0, ErrPortionUnknown
		}
		return a.Value * f.Portions[i].Grams, nil
	}
	if g, ok := gramsPer[a.Unit]; ok {
		return a.Value * g, nil
	}
	ml, ok := mlPer[a.Unit]
	if !ok {
		return 0, ErrUnitUnknown
	}
	if f.DensityGPerML <= 0 {
		return 0, ErrDensityUnknown
	}
	return a.Value * ml * f.DensityGPerML, nil
}

// Nutrients is what grams of a food held: each nutrient the food has, scaled from 100 g. A
// nutrient the food lacks stays absent, never zero. Nothing is rounded.
func Nutrients(f Food, grams float64) map[string]float64 {
	out := make(map[string]float64, len(f.NutrientsPer100g))
	for k, per100 := range f.NutrientsPer100g {
		out[k] = per100 * grams / 100
	}
	return out
}

// Vectors is schema/v0/amount-vectors.json.
type Vectors struct {
	Foods []Food `json:"foods"`
	Cases []struct {
		Name   string `json:"name"`
		Note   string `json:"note,omitempty"`
		Food   string `json:"food"`
		Amount Amount `json:"amount"`
		Expect struct {
			Grams     *float64           `json:"grams,omitempty"`
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
