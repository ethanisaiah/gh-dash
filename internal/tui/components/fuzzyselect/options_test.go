package fuzzyselect

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOptionSourceReplacesWholeInput(t *testing.T) {
	src := &OptionSource{Options: []Suggestion{{Value: "Ready to Build"}, {Value: "In Progress"}}}

	ctx := src.ExtractContext("ready to", tea.Position{X: 3})
	if ctx.Content != "ready to" {
		t.Fatalf("context should be the whole input, got %q", ctx.Content)
	}

	got, pos := src.InsertSuggestion("ready to", "Ready to Build", ctx.Start, ctx.End)
	if got != "Ready to Build" || pos.X != len("Ready to Build") {
		t.Fatalf("got %q at %d", got, pos.X)
	}
	if n := len(src.Suggestions("", tea.Position{})); n != 2 {
		t.Fatalf("want all options, got %d", n)
	}
}
