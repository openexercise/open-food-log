// Command ofl validates Open Food Log documents, checks a server against the API core and MCP
// profile, checks an implementation of the amount calculation against the vectors, and builds the
// website.
package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	openfoodlog "github.com/openexercise/open-food-log"
)

const usage = `usage:
  ofl validate [path...]                                 validate documents (examples/ by default); each names its schema in $schema
  ofl conformance [--json] [--token T] <base-url>        check a server's REST API
  ofl conformance mcp [--json] [--token T] [--read-token R] <mcp-url>
                                                         check a server's MCP tools
  ofl vectors [--json] -- <command> [args...]            check an amount calculation against the vectors
  ofl site [outdir]                                      build the website (dist/ by default)
  ofl catalogue <dir>                                    rebuild catalogue/ from the FoodData Central downloads in dir`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "validate":
		err = validate(os.Args[2:])
	case "conformance":
		if len(os.Args) > 2 && os.Args[2] == "mcp" {
			err = runMCPConformance(os.Args[3:])
		} else {
			err = runConformance(os.Args[2:])
		}
	case "vectors":
		err = runVectors(os.Args[2:])
	case "site":
		out := "dist"
		if len(os.Args) > 2 {
			out = os.Args[2]
		}
		err = buildSite(".", out)
	case "catalogue":
		if len(os.Args) < 3 {
			err = errors.New("want the directory holding the FoodData Central downloads")
			break
		}
		err = buildCatalogue(os.Args[2], ".")
	default:
		err = fmt.Errorf("unknown command %q\n%s", os.Args[1], usage)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// validate checks every JSON file under the paths (files or directories; examples by default).
func validate(paths []string) error {
	if len(paths) == 0 {
		paths = []string{"examples"}
	}
	v, err := openfoodlog.NewValidatorFS(os.DirFS("."))
	if err != nil {
		v, err = openfoodlog.NewValidator() // outside a checkout, the embedded schemas
		if err != nil {
			return err
		}
	}
	var problems, count int
	for _, root := range paths {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".json") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			count++
			for _, p := range v.Validate(data) {
				problems++
				fmt.Printf("%s: %s\n", path, p)
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if problems > 0 {
		return fmt.Errorf("%d problem(s) in %d document(s)", problems, count)
	}
	if count == 0 {
		return errors.New("no documents found")
	}
	fmt.Printf("%d document(s) valid\n", count)
	return nil
}
