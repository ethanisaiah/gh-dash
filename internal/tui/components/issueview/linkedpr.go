package issueview

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

// LinkedPRChosenMsg is sent when a linked PR was picked to be opened.
type LinkedPRChosenMsg struct {
	PR data.LinkedPR
}

func linkedPRLabel(pr data.LinkedPR) string {
	return fmt.Sprintf("#%d %s", pr.Number, pr.Title)
}

// PickLinkedPR lets the user choose one of several linked PRs.
func (m *Model) PickLinkedPR(prs []data.LinkedPR) tea.Cmd {
	if m.issue == nil {
		return nil
	}
	m.linkedPRs = prs
	options := make([]fuzzyselect.Suggestion, 0, len(prs))
	for _, pr := range prs {
		options = append(options, fuzzyselect.Suggestion{
			Value:  linkedPRLabel(pr),
			Detail: strings.ToLower(pr.State) + " · " + pr.Repository.NameWithOwner,
		})
	}
	return m.enterProjectPicker(cmpcontroller.ModeLinkedPR,
		"Open linked PR (enter to pick)"+constants.Ellipsis, options)
}

func (m *Model) submitLinkedPR(value string) tea.Cmd {
	prs := m.linkedPRs
	m.linkedPRs = nil
	for _, pr := range prs {
		if linkedPRLabel(pr) == value || strings.HasPrefix(value, fmt.Sprintf("#%d ", pr.Number)) {
			return func() tea.Msg { return LinkedPRChosenMsg{PR: pr} }
		}
	}
	return nil
}
