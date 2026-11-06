package data

import "testing"

func TestSortLinkedPRs(t *testing.T) {
	prs := []LinkedPR{
		{Number: 3, State: "MERGED"},
		{Number: 5, State: "OPEN"},
		{Number: 9, State: "CLOSED"},
		{Number: 8, State: "OPEN"},
	}
	SortLinkedPRs(prs)
	want := []int{8, 5, 9, 3}
	for i, n := range want {
		if prs[i].Number != n {
			t.Fatalf("position %d: got #%d, want #%d (%+v)", i, prs[i].Number, n, prs)
		}
	}
}
