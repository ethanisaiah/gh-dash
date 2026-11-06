package issueview

import (
	"fmt"
	"image/color"
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/cmpcontroller"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/fuzzyselect"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/inputbox"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issuerow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issuessection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/markdown"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

var (
	htmlCommentRegex = regexp.MustCompile("(?U)<!--(.|[[:space:]])*-->")
	lineCleanupRegex = regexp.MustCompile(`((\n)+|^)([^\r\n]*\|[^\r\n]*(\n)?)+`)
)

type Model struct {
	ctx       *context.ProgramContext
	issue     *issuerow.Issue
	sectionId int
	width     int
	editor    cmpcontroller.Controller
	// handoff is set while the Status picker is the first step of a
	// "set status and reassign" handoff; handoffStatus is the picked status.
	handoff       bool
	handoffStatus string
	// linkedPRs are the choices while picking a linked PR.
	linkedPRs []data.LinkedPR
	// images are the choices while picking an image to open.
	images []utils.ImageRef
}

func NewModel(ctx *context.ProgramContext) Model {
	ta := inputbox.DefaultTextArea(ctx)
	cmp := cmpcontroller.New(ctx, inputbox.ModelOpts{TextArea: &ta})

	return Model{
		ctx:    ctx,
		issue:  nil,
		editor: cmp,
	}
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd, *IssueAction) {
	if cmd, handled := m.updateProjectInput(msg); handled {
		return m, cmd, nil
	}

	cmd, handled := m.editor.Update(msg)

	if msg, ok := msg.(tea.KeyMsg); ok && msg.String() == "ctrl+d" {
		value := m.editor.Value()
		mode := m.editor.Mode()
		m.editor.Exit()
		if m.issue == nil {
			return m, nil, nil
		}

		sid := tasks.SectionIdentifier{Id: m.sectionId, Type: issuessection.SectionType}

		switch mode {
		case cmpcontroller.ModeComment:
			if len(strings.TrimSpace(value)) != 0 {
				return m, tasks.CommentOnIssue(m.ctx, sid, m.issue.Data, value), nil
			}
			return m, nil, nil

		case cmpcontroller.ModeAssign:
			usernames := fuzzyselect.AllWords(value)
			if len(usernames) > 0 {
				return m, tasks.AssignIssue(m.ctx, sid, m.issue.Data, usernames), nil
			}
			return m, nil, nil

		case cmpcontroller.ModeUnassign:
			usernames := fuzzyselect.AllWords(value)
			if len(usernames) > 0 {
				return m, tasks.UnassignIssue(m.ctx, sid, m.issue.Data, usernames), nil
			}
			return m, nil, nil

		case cmpcontroller.ModeLabel:
			labels := fuzzyselect.CurrentLabels(value)
			if len(labels) > 0 || len(m.issue.Data.Labels.Nodes) > 0 {
				return m, tasks.LabelIssue(
					m.ctx,
					sid,
					m.issue.Data,
					labels,
					m.issue.Data.Labels.Nodes,
				), nil
			}
			return m, nil, nil
		}
	}
	if handled {
		return m, cmd, nil
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch {
		case key.Matches(keyMsg, keys.IssueKeys.Label):
			return m, nil, &IssueAction{Type: IssueActionLabel}
		case key.Matches(keyMsg, keys.IssueKeys.Assign):
			return m, nil, &IssueAction{Type: IssueActionAssign}
		case key.Matches(keyMsg, keys.IssueKeys.Unassign):
			return m, nil, &IssueAction{Type: IssueActionUnassign}
		case key.Matches(keyMsg, keys.IssueKeys.Comment):
			return m, nil, &IssueAction{Type: IssueActionComment}
		case key.Matches(keyMsg, keys.IssueKeys.Checkout):
			return m, nil, &IssueAction{Type: IssueActionCheckout}
		case key.Matches(keyMsg, keys.IssueKeys.Close):
			return m, nil, &IssueAction{Type: IssueActionClose}
		case key.Matches(keyMsg, keys.IssueKeys.Reopen):
			return m, nil, &IssueAction{Type: IssueActionReopen}
		case m.ctx.Config.Project != nil && key.Matches(keyMsg, keys.IssueKeys.ProjectStatus):
			return m, nil, &IssueAction{Type: IssueActionProjectStatus}
		case m.ctx.Config.Project != nil && key.Matches(keyMsg, keys.IssueKeys.ProjectIteration):
			return m, nil, &IssueAction{Type: IssueActionProjectIteration}
		case m.ctx.Config.Project != nil && key.Matches(keyMsg, keys.IssueKeys.Handoff):
			return m, nil, &IssueAction{Type: IssueActionHandoff}
		case key.Matches(keyMsg, keys.IssueKeys.LinkedPR):
			return m, nil, &IssueAction{Type: IssueActionLinkedPR}
		case m.ctx.Config.Project != nil && key.Matches(keyMsg, keys.IssueKeys.StartWork):
			return m, nil, &IssueAction{Type: IssueActionStartWork}
		case key.Matches(keyMsg, keys.IssueKeys.Images):
			return m, nil, &IssueAction{Type: IssueActionImages}
		}
	}

	return m, cmd, nil
}

func (m Model) View() string {
	s := strings.Builder{}

	s.WriteString(m.renderFullNameAndNumber())
	s.WriteString("\n")

	s.WriteString(m.renderTitle())
	s.WriteString("\n\n")
	s.WriteString(m.renderStatusPill())
	s.WriteString("\n\n")
	if project := m.renderProject(); project != "" {
		s.WriteString(project)
		s.WriteString("\n\n")
	}
	s.WriteString(m.renderAuthor())
	s.WriteString("\n\n")

	labels := m.renderLabels()
	if labels != "" {
		s.WriteString(labels)
		s.WriteString("\n\n")
	}

	s.WriteString(m.renderBody())
	s.WriteString("\n\n")
	s.WriteString(m.renderActivity())

	if m.editor.Mode() != cmpcontroller.ModeNone {
		s.WriteString(m.ctx.Styles.Sidebar.InputBox.Render(m.editor.View()))
	}

	return lipgloss.NewStyle().Padding(0, m.ctx.Styles.Sidebar.ContentPadding).Render(s.String())
}

func (m *Model) ViewCompletions() string {
	if !m.hasData() {
		return ""
	}

	return m.editor.ViewCompletions()
}

func (m *Model) InputBoxLineFromButton() int {
	return m.editor.LineFromBottom()
}

func (m *Model) renderFullNameAndNumber() string {
	return common.RenderPreviewHeader(m.ctx.Theme, m.width,
		fmt.Sprintf("#%d · %s", m.issue.Data.GetNumber(), m.issue.Data.GetRepoNameWithOwner()))
}

func (m *Model) renderTitle() string {
	return common.RenderPreviewTitle(m.ctx.Theme, m.ctx.Styles.Common, m.width, m.issue.Data.Title)
}

func (m *Model) renderStatusPill() string {
	var bgColor color.Color
	content := ""
	switch m.issue.Data.State {
	case "OPEN":
		bgColor = m.ctx.Styles.Colors.OpenIssue.Dark
		content = " Open"
	case "CLOSED":
		bgColor = m.ctx.Styles.Colors.ClosedIssue.Dark
		content = " Closed"
	}

	return m.ctx.Styles.PrView.PillStyle.
		BorderForeground(bgColor).
		Background(bgColor).
		Render(content)
}

// renderProject shows the issue's Status and Iteration in the configured
// project, or that it is not in the project yet.
func (m *Model) renderProject() string {
	project := common.ConfiguredProject(m.ctx.Config)
	if project == nil {
		return ""
	}
	faint := lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText)
	item, fetched := data.GetProjectItem(m.issue.Data.Id)
	if !fetched {
		return ""
	}
	if item == nil {
		return faint.Render(fmt.Sprintf(" Not in %s", project.Title))
	}

	pillStyle := m.ctx.Styles.PrView.PillStyle
	status := item.StatusName()
	var statusPart string
	if status == "" {
		statusPart = faint.Render("No status")
	} else {
		c := common.ProjectOptionColor(m.ctx.Config, status)
		if c == nil {
			c = m.ctx.Theme.FaintText
		}
		statusPart = pillStyle.BorderForeground(c).Background(c).Render(status)
	}

	iteration := item.IterationTitle()
	if iteration == "" {
		iteration = faint.Render("no iteration")
	} else {
		iteration = lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Render(iteration)
	}

	return lipgloss.JoinHorizontal(lipgloss.Center,
		faint.Render(" "+project.Title+" "),
		statusPart,
		faint.Render(" ⋅ 󰃭 "),
		iteration,
	)
}

func (m *Model) renderAuthor() string {
	authorAssociation := m.issue.Data.AuthorAssociation
	if authorAssociation == "" {
		authorAssociation = "unknown role"
	}
	time := lipgloss.NewStyle().Render(utils.TimeElapsed(m.issue.Data.CreatedAt))
	return lipgloss.JoinHorizontal(lipgloss.Top,
		" by ",
		lipgloss.NewStyle().Foreground(m.ctx.Theme.PrimaryText).Render(
			lipgloss.NewStyle().Bold(true).Render("@"+m.issue.Data.Author.Login)),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, " ⋅ ", time, " ago", " ⋅ ")),
		lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(
			lipgloss.JoinHorizontal(lipgloss.Top, data.GetAuthorRoleIcon(m.issue.Data.AuthorAssociation,
				m.ctx.Theme),
				" ", lipgloss.NewStyle().Foreground(m.ctx.Theme.FaintText).Render(strings.ToLower(authorAssociation))),
		),
	)
}

func (m *Model) renderBody() string {
	width := m.getIndentedContentWidth()
	// Strip HTML comments from body and cleanup body.
	body := htmlCommentRegex.ReplaceAllString(m.issue.Data.Body, "")
	body = lineCleanupRegex.ReplaceAllString(body, "")

	body = strings.TrimSpace(body)
	if body == "" {
		return lipgloss.NewStyle().
			Italic(true).
			Foreground(m.ctx.Theme.FaintText).
			Render("No description provided.")
	}

	markdownRenderer := markdown.GetMarkdownRenderer(width, m.ctx)
	rendered, err := markdownRenderer.Render(body)
	if err != nil {
		return ""
	}

	return lipgloss.NewStyle().
		Width(width).
		MaxWidth(width).
		Align(lipgloss.Left).
		Render(rendered)
}

func (m *Model) renderLabels() string {
	width := m.getIndentedContentWidth()
	labels := m.issue.Data.Labels.Nodes
	style := m.ctx.Styles.PrView.PillStyle

	return common.RenderLabels(labels, common.LabelOpts{
		Width:     width,
		PillStyle: style,
	})
}

func (m *Model) getIndentedContentWidth() int {
	return m.width - 6
}

func (m *Model) SetWidth(width int) {
	m.width = width
	m.editor.SetWidth(
		m.getIndentedContentWidth() - m.ctx.Styles.Sidebar.InputBox.GetHorizontalFrameSize(),
	)
}

func (m *Model) SetSectionId(id int) {
	m.sectionId = id
}

func (m *Model) SetRow(data *data.IssueData) {
	if data == nil {
		m.issue = nil
	} else {
		m.issue = &issuerow.Issue{Ctx: m.ctx, Data: *data}
	}
}

func (m *Model) IsTextInputBoxFocused() bool {
	return m.editor.Active()
}

func (m *Model) UpdateProgramContext(ctx *context.ProgramContext) {
	m.ctx = ctx
	m.editor.UpdateProgramContext(ctx)

	// TODO: move this to the NewModel func
	// currently it's not possible since the styles aren't yet instantiated when NewModel is called
	m.editor.SetSelectStyles(ctx.Styles.Select)
}

func (m *Model) GetIsCommenting() bool {
	return m.editor.Mode() == cmpcontroller.ModeComment
}

func (m *Model) SetIsCommenting(isCommenting bool) tea.Cmd {
	if m.issue == nil {
		return nil
	}

	if !isCommenting {
		if m.editor.Mode() == cmpcontroller.ModeComment {
			m.editor.Exit()
		}
		return nil
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: true})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeComment,
		Prompt:                           constants.CommentPrompt,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		ConfirmDiscardOnCancel:           true,
		HideAutocompleteWhenContextEmpty: true,
	})
	return cmd
}

func (m *Model) GetIsAssigning() bool {
	return m.editor.Mode() == cmpcontroller.ModeAssign
}

func (m *Model) SetIsAssigning(isAssigning bool) tea.Cmd {
	if m.issue == nil {
		return nil
	}

	if !isAssigning {
		if m.editor.Mode() == cmpcontroller.ModeAssign {
			m.editor.Exit()
		}
		return nil
	}

	initialValue := ""
	if !m.userAssignedToIssue(m.ctx.User) {
		initialValue = m.ctx.User
	}

	m.editor.SetAutocompleteSource(&fuzzyselect.UserMentionSource{WithAtSymbol: false})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeAssign,
		Prompt:                           constants.AssignPrompt,
		InitialValue:                     initialValue,
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		HideAutocompleteWhenContextEmpty: false,
	})
	return cmd
}

func (m *Model) SetIsLabeling(isLabeling bool) tea.Cmd {
	if m.issue == nil {
		return nil
	}

	if !isLabeling {
		if m.editor.Mode() == cmpcontroller.ModeLabel {
			m.editor.Exit()
		}
		return nil
	}

	labels := make([]string, 0, len(m.issue.Data.Labels.Nodes)+1)
	for _, label := range m.issue.Data.Labels.Nodes {
		labels = append(labels, label.Name)
	}
	labels = append(labels, "")

	m.editor.SetAutocompleteSource(&fuzzyselect.LabelSource{})
	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:                             cmpcontroller.ModeLabel,
		Prompt:                           constants.LabelPrompt,
		InitialValue:                     strings.Join(labels, ", "),
		Repo:                             m.repoRef(),
		EnterFetch:                       cmpcontroller.FetchSilent,
		HideAutocompleteWhenContextEmpty: false,
	})
	m.editor.ShowCompletions()
	return cmd
}

func (m *Model) userAssignedToIssue(login string) bool {
	for _, a := range m.issue.Data.Assignees.Nodes {
		if login == a.Login {
			return true
		}
	}
	return false
}

func (m *Model) GetIsUnassigning() bool {
	return m.editor.Mode() == cmpcontroller.ModeUnassign
}

func (m *Model) SetIsUnassigning(isUnassigning bool) tea.Cmd {
	if m.issue == nil {
		return nil
	}

	if !isUnassigning {
		if m.editor.Mode() == cmpcontroller.ModeUnassign {
			m.editor.Exit()
		}
		return nil
	}

	cmd := m.editor.Enter(cmpcontroller.EnterOptions{
		Mode:         cmpcontroller.ModeUnassign,
		Prompt:       constants.UnassignPrompt,
		InitialValue: strings.Join(m.issueAssignees(), "\n"),
		Repo:         m.repoRef(),
	})
	return cmd
}

func (m *Model) issueAssignees() []string {
	var assignees []string
	for _, n := range m.issue.Data.Assignees.Nodes {
		assignees = append(assignees, n.Login)
	}
	return assignees
}

func (m *Model) repoRef() cmpcontroller.RepoRef {
	owner, repo := m.issue.Data.GetRepoNameAndOwner()
	return cmpcontroller.RepoRef{
		NameWithOwner: m.issue.Data.GetRepoNameWithOwner(),
		Owner:         owner,
		Name:          repo,
	}
}

func (m *Model) hasData() bool {
	return m.issue != nil
}
