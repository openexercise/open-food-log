package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

//go:embed templates/*.html templates/style.css
var templateFS embed.FS

type page struct {
	Title       string
	Description string
	Path        string
	HTML        template.HTML
	Schemas     []schemaDoc
	Nutrients   []nutrient
}

type nutrient struct {
	Key           string  `json:"key"`
	Name          string  `json:"name"`
	Unit          string  `json:"unit"`
	INFOODS       *string `json:"infoods"`
	FDC           *string `json:"fdc_nutrient_number"`
	HealthConnect *string `json:"health_connect"`
	HealthKit     *string `json:"healthkit"`
	Notes         string  `json:"notes"`
}

// schemaDoc is one JSON Schema file, read generically for rendering.
type schemaDoc struct {
	Kind        string
	ID          string
	Title       string
	Description string
	Root        objectDoc
	Defs        []objectDoc
}

type objectDoc struct {
	Name        string
	Description string
	Properties  []propertyDoc
}

type propertyDoc struct {
	Name        string
	Type        string
	Required    bool
	Description string
}

// documentOrder puts the schemas in reading order on the site.
var documentOrder = []string{"meal", "food", "recipe", "targets", "log"}

func buildSite(root, out string) error {
	v0 := filepath.Join(root, "schema", "v0")
	schemas, err := loadSchemas(v0)
	if err != nil {
		return err
	}
	var vocab struct {
		Nutrients []nutrient `json:"nutrients"`
	}
	raw, err := os.ReadFile(filepath.Join(v0, "nutrients.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, &vocab); err != nil {
		return fmt.Errorf("decode nutrients.json: %w", err)
	}

	tmpl, err := template.New("").Funcs(template.FuncMap{
		"or": func(s *string) string {
			if s == nil {
				return "—"
			}
			return *s
		},
	}).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}
	if err := os.RemoveAll(out); err != nil {
		return fmt.Errorf("clear output: %w", err)
	}
	render := func(path, name string, p page) error {
		p.Path = path
		var b bytes.Buffer
		if err := tmpl.ExecuteTemplate(&b, name, p); err != nil {
			return fmt.Errorf("render %s: %w", path, err)
		}
		return writeFile(filepath.Join(out, path, "index.html"), b.Bytes())
	}

	spec, err := renderMarkdown(filepath.Join(v0, "SPEC.md"))
	if err != nil {
		return err
	}
	changelog, err := renderMarkdown(filepath.Join(root, "schema", "CHANGELOG.md"))
	if err != nil {
		return err
	}
	pages := []struct {
		path, template string
		p              page
	}{
		{"", "home.html", page{Title: "Open Food Log", Description: "An open format for a food log: what someone ate, the food it matched, how much, and which numbers were guessed.", Schemas: schemas}},
		// The specification sits beside the schemas, so its links to them work as written.
		{"schema/v0", "doc.html", page{Title: "Specification", Description: "The Open Food Log specification, version 0 draft.", HTML: spec}},
		{"schema", "schema.html", page{Title: "Schema", Description: "The Open Food Log schemas, field by field: a meal, a food, a recipe, targets and a log.", Schemas: schemas}},
		{"nutrients", "nutrients.html", page{Title: "Nutrients", Description: "The nutrient keys, their units, and the matching INFOODS tagnames, FoodData Central numbers, and Health Connect and HealthKit fields.", Nutrients: vocab.Nutrients}},
		{"changelog", "doc.html", page{Title: "Changelog", Description: "What changed in the Open Food Log.", HTML: changelog}},
	}
	for _, pg := range pages {
		if err := render(pg.path, pg.template, pg.p); err != nil {
			return err
		}
	}

	// The schema files at their $id paths, the vocabulary, the vectors, the examples, the stylesheet.
	copies := map[string]string{}
	files, _ := filepath.Glob(filepath.Join(v0, "*.json"))
	for _, f := range files {
		copies["schema/v0/"+filepath.Base(f)] = f
	}
	copies["catalogue/foods.jsonl"] = filepath.Join(root, "catalogue", "foods.jsonl")
	examples, _ := filepath.Glob(filepath.Join(root, "examples", "*.json"))
	for _, e := range examples {
		copies["examples/"+filepath.Base(e)] = e
	}
	for dest, src := range copies {
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := writeFile(filepath.Join(out, dest), data); err != nil {
			return err
		}
	}
	css, err := templateFS.ReadFile("templates/style.css")
	if err != nil {
		return err
	}
	if err := writeFile(filepath.Join(out, "style.css"), css); err != nil {
		return err
	}
	fmt.Printf("site: %d schemas, %d nutrients, %d examples in %s\n", len(schemas), len(vocab.Nutrients), len(examples), out)
	return nil
}

func loadSchemas(dir string) ([]schemaDoc, error) {
	var docs []schemaDoc
	for _, kind := range documentOrder {
		p := filepath.Join(dir, kind+".json")
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var s map[string]any
		if err := json.Unmarshal(data, &s); err != nil {
			return nil, fmt.Errorf("decode %s: %w", p, err)
		}
		doc := schemaDoc{Kind: kind, ID: str(s["$id"]), Title: str(s["title"]), Description: str(s["description"]), Root: objectFor(kind, s)}
		if defs, ok := s["$defs"].(map[string]any); ok {
			names := make([]string, 0, len(defs))
			for n := range defs {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				if d, ok := defs[n].(map[string]any); ok {
					doc.Defs = append(doc.Defs, objectFor(n, d))
				}
			}
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func objectFor(name string, s map[string]any) objectDoc {
	o := objectDoc{Name: name, Description: str(s["description"])}
	required := map[string]bool{}
	if req, ok := s["required"].([]any); ok {
		for _, r := range req {
			required[str(r)] = true
		}
	}
	props, _ := s["properties"].(map[string]any)
	names := make([]string, 0, len(props))
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		p, _ := props[n].(map[string]any)
		o.Properties = append(o.Properties, propertyDoc{Name: n, Type: typeOf(p), Required: required[n], Description: str(p["description"])})
	}
	return o
}

// typeOf describes a property's shape in a few words.
func typeOf(p map[string]any) string {
	if p == nil {
		return ""
	}
	switch {
	case p["$ref"] != nil:
		ref := str(p["$ref"])
		return ref[strings.LastIndex(ref, "/")+1:]
	case p["const"] != nil:
		return fmt.Sprintf("the constant %v", p["const"])
	case p["enum"] != nil:
		parts := []string{}
		if vals, ok := p["enum"].([]any); ok {
			for _, v := range vals {
				parts = append(parts, fmt.Sprint(v))
			}
		}
		return "one of " + strings.Join(parts, ", ")
	}
	var types []string
	switch t := p["type"].(type) {
	case string:
		types = []string{t}
	case []any:
		for _, v := range t {
			types = append(types, str(v))
		}
	}
	out := strings.Join(types, " or ")
	if items, ok := p["items"].(map[string]any); ok && out == "array" {
		out = "array of " + typeOf(items)
	}
	if f := str(p["format"]); f != "" {
		out += " (" + f + ")"
	}
	return out
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func renderMarkdown(path string) (template.HTML, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var b bytes.Buffer
	if err := goldmark.New(goldmark.WithExtensions(extension.GFM)).Convert(src, &b); err != nil {
		return "", fmt.Errorf("render %s: %w", path, err)
	}
	return template.HTML(b.String()), nil // our own file in this repository
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
