package helpers

import (
	"strings"
	"testing"
)

func TestTraverse(t *testing.T) {
	tests := []struct {
		Title         string
		Run           func() (any, error)
		Object        any
		Keys          []any
		Fallback      any
		Expected      any
		ExpectedError string
	}{
		{
			Title: "slice: OK",
			Run: func() (any, error) {
				return TraverseWithError([]int{1, 2, 3}, []any{1}, 0)
			},
			Expected: 2,
		},
		{
			Title: "slice: invalid key",
			Run: func() (any, error) {
				return TraverseWithError([]int{1, 2, 3}, []any{"x"}, 0)
			},
			Expected:      0,
			ExpectedError: "expected int key",
		},
		{
			Title: "slice: out of range",
			Run: func() (any, error) {
				return TraverseWithError([]int{1, 2, 3}, []any{4}, 5)
			},
			Expected:      5,
			ExpectedError: "index 4 out of range 2",
		},
		{
			Title: "slice: invalid result type",
			Run: func() (any, error) {
				return TraverseWithError([]int{1, 2, 3}, []any{1}, "?")
			},
			Expected:      "?",
			ExpectedError: "could not type assert final value int into string",
		},
		{
			Title: "map: OK",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{"a": 1}, []any{"a"}, 1)
			},
			Expected: 1,
		},
		{
			Title: "map: invalid key",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{"a": 1}, []any{1}, 1)
			},
			Expected:      1,
			ExpectedError: "expected string key",
		},
		{
			Title: "map: key not found",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{"a": 1}, []any{"b"}, 2)
			},
			Expected:      2,
			ExpectedError: "key b not found",
		},
		{
			Title: "map: invalid result type",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{"a": 1}, []any{"a"}, "?")
			},
			Expected:      "?",
			ExpectedError: "could not type assert final value int into string",
		},
		{
			Title: "slice_map: OK",
			Run: func() (any, error) {
				return TraverseWithError([]any{nil, map[string]any{"a": 1}}, []any{1, "a"}, 1)
			},
			Expected: 1,
		},
		{
			Title: "slice_map: invalid object to traverse",
			Run: func() (any, error) {
				return TraverseWithError([]any{1, map[string]any{"a": 1}}, []any{0, "a"}, 1)
			},
			Expected:      1,
			ExpectedError: "cannot traverse object of type int",
		},
		{
			Title: "map_slice: OK",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{"a": []any{1, 2, 4}, "b": "c"}, []any{"a", 1}, 1)
			},
			Expected: 2,
		},
		{
			Title: "deep: OK",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{
					"a": map[string]any{
						"b": []any{
							0,
							0,
							map[string]any{
								"c": []any{1, 2, 3, 4, 5},
							},
						},
					},
				}, []any{"a", "b", 2, "c", 3}, 0)
			},
			Expected: 4,
		},
		{
			Title: "deep: index error",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{
					"a": map[string]any{
						"b": []any{
							0,
							0,
							map[string]any{
								"c": []any{1, 2, 3, 4, 5},
							},
						},
					},
				}, []any{"a", "b", 5, "c", 3}, 0)
			},
			Expected:      0,
			ExpectedError: "index 5 out of range 2",
		},
		{
			Title: "deep: key error",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{
					"a": map[string]any{
						"b": []any{
							0,
							0,
							map[string]any{
								"c": []any{1, 2, 3, 4, 5},
							},
						},
					},
				}, []any{"a", "b", 2, "d", 3}, 0)
			},
			Expected:      0,
			ExpectedError: "key d not found",
		},
		{
			Title: "deep: traverse error",
			Run: func() (any, error) {
				return TraverseWithError(map[string]any{
					"a": map[string]any{
						"b": []any{
							0,
							nil,
							map[string]any{
								"c": []any{1, 2, 3, 4, 5},
							},
						},
					},
				}, []any{"a", "b", 1, "d", 3}, 4)
			},
			Expected:      4,
			ExpectedError: "cannot traverse object of type <nil>",
		},
	}
	for _, tt := range tests {
		t.Run(tt.Title, func(t *testing.T) {
			res, err := tt.Run()
			if tt.ExpectedError == "" && err != nil {
				t.Fatalf("no error expected, but got one: %v", err)
			}
			if tt.ExpectedError != "" {
				if err == nil {
					t.Fatalf("expected '%s' in error, but got no error", tt.ExpectedError)
				} else if !strings.Contains(err.Error(), tt.ExpectedError) {
					t.Fatalf("expected '%s' in error, but got: %v", tt.ExpectedError, err)
				}
			}
			if res != tt.Expected {
				t.Fatalf("expected %v (%T), got %v (%T)", tt.Expected, tt.Expected, res, res)
			}
		})
	}
}
