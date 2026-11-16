// Package alias holds the rule every alias the catalogue keeps must pass, shared by the catalogue
// build (cmd/ofl) and the model runs that suggest aliases (cmd/aliases).
package alias

import "strings"

// Acceptable holds a suggested alias to what the catalogue keeps: a short lowercase phrase, with a
// digit only where the food's own name has it ("2% milk" for "Milk, reduced fat (2%)").
func Acceptable(alias, name string) bool {
	if alias != strings.ToLower(alias) || len(alias) < 2 || len(alias) > 60 || strings.TrimSpace(alias) != alias {
		return false
	}
	for _, r := range alias {
		if r >= '0' && r <= '9' && !strings.ContainsRune(name, r) {
			return false
		}
	}
	return true
}
