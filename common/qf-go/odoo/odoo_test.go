package odoo

import (
	"testing"
)

func TestMapToDomain(t *testing.T) {
	res := MapToDomain(map[string]any{
		"string": "string",
		"int":    1,
		"float":  20.0,
		"bool":   false,
		"map":    map[string]any{"x": "x"},
		"slice":  []int{0},
	})
	if len(res) != 4 {
		t.Fatalf("expected 4 elements in domain, got %v: %v", len(res), res)
	}
	expected := [][]any{
		{"string", "=ilike", "string"},
		{"int", "=", 1},
		{"float", "=", 20.0},
		{"bool", "=", false},
	}
	for i := range 4 {
		r := res[i].([]any)
		e := expected[i]
		if len(r) != 3 || r[0] != e[0] || r[1] != e[1] || r[2] != e[2] {
			t.Fatalf("element %v is not a match, expected %v, got %v", i, expected, res)
		}
	}
}
