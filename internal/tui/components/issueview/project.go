package issueview

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issuessection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
)

const (
	noIterationSuggestion = "(none)"
	projectPickerRows     = 8
)

func isProjectPickerMode(mode cmpcontroller.Mode) bool {
	return mode == cmpcontroller.ModeProjectStatus || mode == cmpcontroller.ModeProjectIteration ||
		mode == cmpcontroller.ModeLinkedPR
}

func (m *Model) projectItem() *data.ProjectItem {
	item, _ := data.GetProjectItem(m.issue.Data.Id)
	return item
}

func (m *Model) projectReady() bool {
	if m.issue == nil || m.ctx.Config.Project == nil {
		return false
	}
	if common.ConfiguredProject(m.ctx.Config) == nil {
		m.ctx.Error = fmt.Errorf("project not loaded yet, try again in a moment")
		return false
	}
	return true
}

func (m *Model) statusSuggestions() []fuzzyselect.Suggestion {
	project := common.ConfiguredProject(m.ctx.Config)
	field := project.FieldByName(m.ctx.Config.Project.StatusFieldName())
	if field == nil {
		return nil
	}
	current := m.projectItem().StatusName()
	res := make([]fuzzyselect.Suggestion, 0, len(field.Options))
	for _, opt := range field.Options {
		detail := opt.Description
		if opt.Name == current {
			detail = "current"
		}
		res = append(res, fuzzyselect.Suggestion{Value: opt.Name, Detail: detail})
	}
	return res
}

func (m *Model) iterationSuggestions() []fuzzyselect.Suggestion {
	project := common.ConfiguredProject(m.ctx.Config)
	field := project.FieldByName(m.ctx.Config.Project.IterationFieldName())
	if field == nil {
		return nil
	}
	current := m.projectItem().IterationTitle()
	now := time.Now()
	res := []fuzzyselect.Suggestion{}
	for _, it := range field.ActiveIterations() {
		detail := fmt.Sprintf("%s – %s",
			it.Start().Format("Jan 2"), it.End().AddDate(0, 0, -1).Format("Jan 2"))
		if it.Contains(now) {
			detail += " · this week"
		}
		if it.Title == current {
			detail += " · current"
		}
		res = append(res, fuzzyselect.Suggestion{Value: it.Title, Detail: detail})
	}
	return append(res, fuzzyselect.Suggestion{Value: noIterationSuggestion, Detail: "clear"})
}

func (m *Model) enterProjectPicker(
	mode cmpcontroller.Mode,
	prompt string,
	options []fuzzyselect.Suggestion,
) tea.Cmd {
	if len(options) == 0 {
		m.ctx.Error = fmt.Errorf("project field not found, check the `project` config")
		return nil
	}
	m.editor.SetAutocompleteSource(&fuzzyselect.OptionSource{Options: options})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:           mode,
		Prompt:         prompt,
		Repo:           m.repoRef(),
		MaxSuggestions: projectPickerRows,
	})
	m.editor.ShowCompletions()
	return cmd
}

// SetIsPickingStatus opens the project Status picker.
func (m *Model) SetIsPickingStatus(picking bool) tea.Cmd {
	return m.setPickingStatus(picking, false)
}

// SetIsHandingOff opens the Status picker followed by an assignee prompt.
func (m *Model) SetIsHandingOff(picking bool) tea.Cmd {
	return m.setPickingStatus(picking, true)
}

func (m *Model) setPickingStatus(picking, handoff bool) tea.Cmd {
	if !picking {
		if isProjectPickerMode(m.editor.Mode()) || m.editor.Mode() == cmpcontroller.ModeHandoffAssign {
			m.editor.Exit()
		}
		return nil
	}
	if !m.projectReady() {
		return nil
	}
	m.handoff = handoff
	prompt := "Set " + m.ctx.Config.Project.StatusFieldName()
	if handoff {
		prompt = "Hand off: new " + m.ctx.Config.Project.StatusFieldName()
	}
	return m.enterProjectPicker(cmpcontroller.ModeProjectStatus,
		prompt+" (enter to pick)"+constants.Ellipsis, m.statusSuggestions())
}

// SetIsPickingIteration opens the project Iteration picker.
func (m *Model) SetIsPickingIteration(picking bool) tea.Cmd {
	if !picking {
		if m.editor.Mode() == cmpcontroller.ModeProjectIteration {
			m.editor.Exit()
		}
		return nil
	}
	if !m.projectReady() {
		return nil
	}
	return m.enterProjectPicker(cmpcontroller.ModeProjectIteration,
		"Set "+m.ctx.Config.Project.IterationFieldName()+" (enter to pick)"+constants.Ellipsis,
		m.iterationSuggestions())
}

// pickedValue is the highlighted suggestion, or the typed text if nothing is
// highlighted.
func (m *Model) pickedValue() string {
	if s := m.editor.SelectedSuggestion(); s != "" {
		return s
	}
	return strings.TrimSpace(m.editor.Value())
}

// submitProjectInput handles submitting the project pickers and the handoff
// assignee prompt.
func (m *Model) submitProjectInput(mode cmpcontroller.Mode, value string) tea.Cmd {
	sid := tasks.SectionIdentifier{Id: m.sectionId, Type: issuessection.SectionType}
	issue := m.issue.Data

	switch mode {
	case cmpcontroller.ModeProjectStatus:
		if value == "" {
			return nil
		}
		if !m.handoff {
			return tasks.SetIssueProjectField(m.ctx, sid, &issue, tasks.ProjectFieldStatus, value)
		}
		m.handoffStatus = value
		assignees := strings.Join(m.issueAssignees(), " ")
		if assignees != "" {
			assignees += " "
		}
		m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: false})
		return m.editor.Enter(cmpcontroller.EnterOptions{
			Mode: cmpcontroller.ModeHandoffAssign,
			Prompt: fmt.Sprintf("%s → assign to (whitespace-separated, enter to save)%s",
				value, constants.Ellipsis),
			InitialValue: assignees,
			Repo:         m.repoRef(),
			EnterFetch:   cmpcontroller.FetchSilent,
		})

	case cmpcontroller.ModeHandoffAssign:
		status := m.handoffStatus
		m.handoffStatus = ""
		m.handoff = false
		return tasks.HandoffIssue(m.ctx, sid, &issue, status, fuzzyselect.AllWords(value))

	case cmpcontroller.ModeLinkedPR:
		return m.submitLinkedPR(value)

	case cmpcontroller.ModeProjectIteration:
		if value == "" {
			return nil
		}
		if value == noIterationSuggestion {
			value = ""
		}
		return tasks.SetIssueProjectField(m.ctx, sid, &issue, tasks.ProjectFieldIteration, value)
	}
	return nil
}

// updateProjectInput intercepts enter/ctrl+d in project modes. It reports
// whether the message was handled.
func (m *Model) updateProjectInput(msg tea.Msg) (tea.Cmd, bool) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok || m.issue == nil {
		return nil, false
	}
	mode := m.editor.Mode()
	isProjectMode := isProjectPickerMode(mode) || mode == cmpcontroller.ModeHandoffAssign
	if !isProjectMode {
		return nil, false
	}
	switch keyMsg.String() {
	case "enter", "ctrl+d":
	case "esc", "ctrl+c":
		m.handoff = false
		m.handoffStatus = ""
		m.linkedPRs = nil
		return nil, false
	default:
		return nil, false
	}

	value := strings.TrimSpace(m.editor.Value())
	if isProjectPickerMode(mode) {
		value = m.pickedValue()
	}
	m.editor.Exit()
	return m.submitProjectInput(mode, value), true
}
