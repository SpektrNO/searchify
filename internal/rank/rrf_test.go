package rank

import "testing"

func TestRRF_prefersItemsInBothLists(t *testing.T) {
	listA := []RankedItem{{ID: "a", Score: 1}, {ID: "b", Score: 0.5}, {ID: "c", Score: 0.1}}
	listB := []RankedItem{{ID: "b", Score: 1}, {ID: "a", Score: 0.8}, {ID: "d", Score: 0.2}}

	got := RRF([][]RankedItem{listA, listB}, 60, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	if got[0].ID != "a" && got[0].ID != "b" {
		t.Fatalf("expected top result a or b, got %q", got[0].ID)
	}
}

func TestRRF_singleListPreservesOrder(t *testing.T) {
	list := []RankedItem{{ID: "x", Score: 1}, {ID: "y", Score: 0.5}}
	got := RRF([][]RankedItem{list}, 60, 2)
	if len(got) != 2 || got[0].ID != "x" || got[1].ID != "y" {
		t.Fatalf("unexpected order: %+v", got)
	}
}
