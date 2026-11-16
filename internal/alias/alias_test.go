package alias

import "testing"

func TestAcceptable(t *testing.T) {
	cases := []struct {
		alias, name string
		want        bool
	}{
		{"hard boiled egg", "Egg, whole, cooked, hard-boiled", true},
		{"2% milk", "Milk, reduced fat (2%)", true},
		{"2 eggs", "Egg, whole, raw", false},
		{"Oj", "Orange juice, raw", false},
		{" oj", "Orange juice, raw", false},
		{"x", "X", false},
	}
	for _, c := range cases {
		if got := Acceptable(c.alias, c.name); got != c.want {
			t.Errorf("Acceptable(%q, %q) = %v", c.alias, c.name, got)
		}
	}
}
