package types

import (
	"qf/go/helpers"
	"testing"
)

func TestEdges_Iter(t *testing.T) {
	edges := Edges[Customer]{
		Edges: []Edge[Customer]{
			{
				Cursor: helpers.StringPtr("CURSOR1"),
				Node: Customer{
					Id: helpers.StringPtr("NODE1"),
				},
			},
			{
				Cursor: helpers.StringPtr("CURSOR2"),
				Node: Customer{
					Id: helpers.StringPtr("NODE2"),
				},
			},
			{
				Cursor: helpers.StringPtr("CURSOR3"),
				Node: Customer{
					Id: helpers.StringPtr("NODE3"),
				},
			},
			{
				Cursor: helpers.StringPtr("CURSOR4"),
				Node: Customer{
					Id: helpers.StringPtr("NODE4"),
				},
			},
		},
	}
	length := edges.Length()
	if edges.Length() != 4 {
		t.Fatalf("Edges length expected to be 4, got %v", length)
	}
	result := make([]string, length*2)
	for i, iterNode := range edges.Iter {
		expectedEdge := &edges.Edges[i]

		expectedNode := &expectedEdge.Node
		nodeByGet := edges.Get(i)
		if iterNode.Id != expectedNode.Id {
			t.Fatalf(
				"Iterating through edges in index %v expected %v (%v) but got %v (%v)",
				i, *expectedNode.Id, expectedNode.Id, *iterNode.Id, iterNode.Id,
			)
		}
		if nodeByGet.Id != expectedNode.Id {
			t.Fatalf(
				"Getting item in index %v expected %v (%v) but got %v (%v)",
				i, *expectedNode.Id, expectedNode.Id, *nodeByGet.Id, nodeByGet.Id,
			)
		}

		cursorByGet := edges.GetCursor(i)
		if cursorByGet != expectedEdge.Cursor {
			t.Fatalf(
				"Getting cursor in index %v expected %v (%v) but got %v (%v)",
				i, *expectedEdge.Cursor, expectedEdge.Cursor, *cursorByGet, cursorByGet,
			)
		}

		result[i*2] = *iterNode.Id
		result[(i*2)+1] = *cursorByGet
	}
	expectedResult := []string{"NODE1", "CURSOR1", "NODE2", "CURSOR2", "NODE3", "CURSOR3", "NODE4", "CURSOR4"}
	for i, expected := range expectedResult {
		if expected != result[i] {
			t.Fatalf(
				"Error during iteration, result slices don't match: EXPECTED=%v GOT=%v",
				expectedResult, result,
			)
		}
	}
}
