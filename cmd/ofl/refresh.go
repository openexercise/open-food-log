package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// refreshCatalogue applies the rules that work on records alone again, over the catalogue
// already built, so changing one needs no FoodData Central downloads: placeholder portions are
// left out, and every food's aliases are attached again from the name rules and sources/. It
// returns the new foods.jsonl and REPORT.md.
func refreshCatalogue(root string) (foods, report []byte, err error) {
	curated, err := readCuratedAliases(root)
	if err != nil {
		return nil, nil, err
	}
	model, err := readModelAliases(root)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(filepath.Join(root, "catalogue", "foods.jsonl"))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	var all []*catalogueFood
	names := map[string]string{}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		food := &catalogueFood{}
		if err := json.Unmarshal(scanner.Bytes(), food); err != nil {
			return nil, nil, err
		}
		all = append(all, food)
		names[food.ID] = food.Name
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}
	if err := checkCurated(curated, names); err != nil {
		return nil, nil, err
	}
	model = soleModelAliases(all, curated, model)
	var buf bytes.Buffer
	var n aliasCounts
	withAliases := 0
	noPortions := map[string]int{} // by the source a record's numbers come from
	for _, food := range all {
		dropPlaceholderPortions(food)
		if len(food.Portions) == 0 && len(food.Provenance.Sources) > 0 {
			noPortions[food.Provenance.Sources[0].Name]++
		}
		dropAliases(food)
		got := attachAliases(food, curated, model)
		n.add(got)
		if got.total() > 0 {
			withAliases++
		}
		line, err := json.Marshal(food)
		if err != nil {
			return nil, nil, err
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	old, err := os.ReadFile(filepath.Join(root, "catalogue", "REPORT.md"))
	if err != nil {
		return nil, nil, err
	}
	if !aliasReportLine.Match(old) {
		return nil, nil, errors.New("catalogue/REPORT.md has no line on aliases to update")
	}
	report = aliasReportLine.ReplaceAll(old, []byte(aliasReport(len(all), withAliases, n)))
	report, err = withPortionCounts(report, noPortions)
	if err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), report, nil
}

// withPortionCounts rewrites the last column of the report's source table, "Kept without
// portions", from the records themselves.
func withPortionCounts(report []byte, noPortions map[string]int) ([]byte, error) {
	lines := strings.Split(string(report), "\n")
	found, inTable := 0, false
	for i, line := range lines {
		if strings.HasSuffix(line, "| Kept without portions |") {
			inTable = true
			continue
		}
		if inTable && !strings.HasPrefix(line, "|") {
			break
		}
		if !inTable {
			continue
		}
		for _, src := range sources {
			if !strings.HasPrefix(line, "| "+src.label+" |") {
				continue
			}
			cells := strings.Split(strings.Trim(line, "| "), " | ")
			cells[len(cells)-1] = strconv.Itoa(noPortions[src.label])
			lines[i] = "| " + strings.Join(cells, " | ") + " |"
			found++
		}
	}
	if found != len(sources) {
		return nil, fmt.Errorf("catalogue/REPORT.md lists %d of the %d sources", found, len(sources))
	}
	return []byte(strings.Join(lines, "\n")), nil
}

// writeRefreshedCatalogue is `ofl catalogue refresh`.
func writeRefreshedCatalogue(root string) error {
	foods, report, err := refreshCatalogue(root)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "catalogue", "foods.jsonl"), foods, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(root, "catalogue", "REPORT.md"), report, 0o644); err != nil {
		return err
	}
	fmt.Println("catalogue: refreshed")
	return nil
}
