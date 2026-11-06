package fuzzyselect

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

// OptionSource suggests a fixed list of values for picking exactly one, e.g.
// a project field option. The whole input is the value being completed.
type OptionSource struct {
	Options []Suggestion
}

func (*OptionSource) ExtractContext(input string, cursorPos tea.Position) Context {
	value := strings.TrimSpace(strings.ReplaceAll(input, "\n", " "))
	ls := lines(input)
	last := len(ls) - 1
	return Context{
		Start:   tea.Position{},
		End:     tea.Position{X: len([]rune(ls[last])), Y: last},
		Content: value,
	}
}

func (*OptionSource) InsertSuggestion(
	_ string,
	suggestion string,
	_ tea.Position,
	_ tea.Position,
) (string, tea.Position) {
	return suggestion, tea.Position{X: len([]rune(suggestion))}
}

func (*OptionSource) ItemsToExclude(string, tea.Position) []string {
	return nil
}

func (src *OptionSource) Suggestions(string, tea.Position) []Suggestion {
	return src.Options
}

func (*OptionSource) LoadSuggestions(LoaderContext) error {
	return nil
}
