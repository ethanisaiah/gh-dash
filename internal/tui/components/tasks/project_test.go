package tasks

import (
	"slices"
	"testing"
)

func TestAssigneeChanges(t *testing.T) {
	cases := []struct {
		current, wanted, add, remove []string
	}{
		{[]string{"alice"}, []string{"bob"}, []string{"bob"}, []string{"alice"}},
		{[]string{"alice", "bob"}, []string{"bob"}, nil, []string{"alice"}},
		{nil, []string{"bob", "bob"}, []string{"bob"}, nil},
		{[]string{"alice"}, nil, nil, []string{"alice"}},
		{[]string{"alice"}, []string{"alice"}, nil, nil},
	}
	for _, c := range cases {
		add, remove := AssigneeChanges(c.current, c.wanted)
		if !slices.Equal(add, c.add) || !slices.Equal(remove, c.remove) {
			t.Errorf("%v -> %v: got +%v -%v, want +%v -%v",
				c.current, c.wanted, add, remove, c.add, c.remove)
		}
	}
}
