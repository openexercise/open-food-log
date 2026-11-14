package main

import (
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	hrefPattern = regexp.MustCompile(`(?:href|src)="([^"]*)"`)
	idPattern   = regexp.MustCompile(`\sid="([^"]*)"`)
)

// TestSiteLinks builds the site and checks every link on every page: none is left pointing at a
// Markdown file, and each one inside the site names a file the site has, and a section that page
// has.
func TestSiteLinks(t *testing.T) {
	out := t.TempDir()
	if err := buildSite("../..", out); err != nil {
		t.Fatal(err)
	}
	ids := map[string]map[string]bool{} // site file → its element IDs
	idsOf := func(file string) map[string]bool {
		if m, ok := ids[file]; ok {
			return m
		}
		m := map[string]bool{}
		if data, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(file))); err == nil {
			for _, id := range idPattern.FindAllStringSubmatch(string(data), -1) {
				m[id[1]] = true
			}
		}
		ids[file] = m
		return m
	}
	pages, links := 0, 0
	err := filepath.WalkDir(out, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".html") {
			return err
		}
		pages++
		rel, _ := filepath.Rel(out, p)
		page := "/" + filepath.ToSlash(rel)
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range hrefPattern.FindAllStringSubmatch(string(data), -1) {
			links++
			u, err := url.Parse(m[1])
			if err != nil {
				t.Errorf("%s: bad link %q", page, m[1])
				continue
			}
			if u.Scheme != "" || u.Host != "" {
				continue // outside the site, GitHub included
			}
			if strings.HasSuffix(u.Path, ".md") {
				t.Errorf("%s: link to a Markdown file %q", page, m[1])
				continue
			}
			target := u.Path
			switch {
			case target == "":
				target = page
			case !strings.HasPrefix(target, "/"):
				target = path.Join(path.Dir(page), target)
				if strings.HasSuffix(u.Path, "/") {
					target += "/"
				}
			}
			if strings.HasSuffix(target, "/") {
				target += "index.html"
			}
			if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(target))); err != nil {
				t.Errorf("%s: link %q: the site has no %s", page, m[1], target)
				continue
			}
			if u.Fragment != "" && strings.HasSuffix(target, ".html") && !idsOf(target[1:])[u.Fragment] {
				t.Errorf("%s: link %q: %s has no section %q", page, m[1], target, u.Fragment)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages == 0 || links == 0 {
		t.Fatalf("checked %d pages and %d links", pages, links)
	}
}

func TestResolveLinks(t *testing.T) {
	l := linker{root: "../..", copies: map[string]string{"examples/meal.json": "", "schema/v0/meal.json": ""}}
	cases := []struct{ from, dest, want string }{
		{"docs/developer-guide.md", "../schema/v0/SPEC.md", "/schema/v0/"},
		{"docs/developer-guide.md", "../schema/v0/SPEC.md#6-amounts", "/schema/v0/#6-amounts"},
		{"docs/developer-guide.md", "../schema/v0", "/schema/"},
		{"docs/developer-guide.md", "../examples/meal.json", "/examples/meal.json"},
		{"schema/v0/SPEC.md", "meal.json", "/schema/v0/meal.json"},
		{"docs/developer-guide.md", "../CONTRIBUTING.md#registering", repoURL + "/blob/main/CONTRIBUTING.md#registering"},
		{"docs/developer-guide.md", "../clients/go", repoURL + "/tree/main/clients/go"},
		{"docs/developer-guide.md", "#3-foods", "#3-foods"},
		{"docs/developer-guide.md", "https://example.org/a.md", "https://example.org/a.md"},
	}
	for _, c := range cases {
		got, err := l.resolve(c.from, c.dest)
		if err != nil || got != c.want {
			t.Errorf("resolve(%q, %q) = %q, %v; want %q", c.from, c.dest, got, err, c.want)
		}
	}
	if _, err := l.resolve("docs/developer-guide.md", "../no-such-file.md"); err == nil {
		t.Error("a link to a missing file resolved")
	}
}
