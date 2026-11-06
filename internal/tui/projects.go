package tui

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"text/template"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issuessection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tasks"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/utils"
)

// linkedPRSectionPrefix marks the PR section opened by jumping from an issue
// to its linked PR. It is reused for the next jump.
const linkedPRSectionPrefix = "→ PR #"

type linkedPRsFetchedMsg struct {
	issueNumber int
	prs         []data.LinkedPR
	err         error
}

// currentIssue returns the issue under the cursor in the issues view, or the
// issue opened from a notification.
func (m *Model) currentIssue() *data.IssueData {
	if m.ctx.View == config.NotificationsView {
		return m.notificationView.GetSubjectIssue()
	}
	if m.ctx.View != config.IssuesView {
		return nil
	}
	if issue, ok := m.getCurrRowData().(*data.IssueData); ok && issue != nil {
		return issue
	}
	return nil
}

func (m *Model) fetchLinkedPRs() tea.Cmd {
	issue := m.currentIssue()
	if issue == nil {
		return nil
	}
	number, id := issue.Number, issue.Id
	return tea.Batch(
		m.notify(fmt.Sprintf("Looking up PRs linked to #%d", number)),
		func() tea.Msg {
			prs, err := data.FetchLinkedPRs(id)
			return linkedPRsFetchedMsg{issueNumber: number, prs: prs, err: err}
		},
	)
}

func (m *Model) onLinkedPRsFetched(msg linkedPRsFetchedMsg) tea.Cmd {
	switch {
	case msg.err != nil:
		return m.notifyErr(fmt.Sprintf("Failed fetching linked PRs: %v", msg.err))
	case len(msg.prs) == 0:
		return m.notifyErr(fmt.Sprintf("#%d has no linked pull requests", msg.issueNumber))
	case len(msg.prs) == 1:
		return m.openLinkedPR(msg.prs[0])
	default:
		prs := msg.prs
		return m.openSidebarForInput(func(bool) tea.Cmd {
			return m.issueSidebar.PickLinkedPR(prs)
		})
	}
}

// openLinkedPR switches to the PRs view and shows the PR in a section filtered
// to its head branch, so all the usual PR actions (diff, checkout, review...)
// work on it.
func (m *Model) openLinkedPR(pr data.LinkedPR) tea.Cmd {
	if m.ctx.View == config.NotificationsView {
		keys.SetNotificationSubject(keys.NotificationSubjectNone)
		m.notificationView.ClearSubject()
	}
	m.ctx.View = config.PRsView

	cfg := config.PrsSectionConfig{
		Title: fmt.Sprintf("%s%d", linkedPRSectionPrefix, pr.Number),
		Filters: fmt.Sprintf("repo:%s head:%s %s",
			pr.Repository.NameWithOwner, pr.HeadRefName, prStateFilter(pr.State)),
	}

	sid := len(m.prs)
	for i, s := range m.prs {
		if ps, ok := s.(*prssection.Model); ok &&
			strings.HasPrefix(ps.Config.Title, linkedPRSectionPrefix) {
			sid = i
			break
		}
	}
	section := new(prssection.NewModel(sid, m.ctx, cfg, time.Now(), time.Now()))
	if sid == len(m.prs) {
		m.prs = append(m.prs, section)
	} else {
		m.prs[sid] = section
	}

	m.syncMainContentDimensions()
	m.updateTabs()
	m.setCurrSectionId(sid)

	cmds := []tea.Cmd{}
	if m.isViewSectionsStale() {
		cmds = append(cmds, m.fetchAllViewSections())
	} else {
		cmds = append(cmds, section.FetchNextPageSectionRows()...)
	}
	cmds = append(cmds, m.onViewedRowChanged())
	return tea.Batch(cmds...)
}

func prStateFilter(state string) string {
	switch state {
	case "OPEN":
		return "is:open"
	case "MERGED":
		return "is:merged"
	default:
		return ""
	}
}

type startWorkFinishedMsg struct {
	issue  data.IssueData
	status string
	err    error
}

// startWork runs the configured startWork command for the current issue with
// the TUI suspended, then sets the issue's status if the command succeeded.
func (m *Model) startWork() tea.Cmd {
	issue := m.currentIssue()
	if issue == nil {
		return nil
	}
	pc := m.ctx.Config.Project
	if pc == nil || pc.StartWork == nil {
		return m.notifyErr("Configure project.startWork.command to use this key")
	}

	input := resolveTemplateInput(&map[string]any{
		"RepoName":    issue.GetRepoNameWithOwner(),
		"IssueNumber": issue.Number,
		"IssueTitle":  issue.Title,
		"Author":      issue.Author.Login,
		"Slug":        utils.Slugify(issue.Title, 40),
	}, m.ctx.Config.RepoPaths, m.ctx.RepoPath)

	tmpl, err := template.New("start_work").Option("missingkey=error").Parse(pc.StartWork.Command)
	if err != nil {
		return m.notifyErr(fmt.Sprintf("Invalid startWork command: %v", err))
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, input); err != nil {
		return m.notifyErr(fmt.Sprintf("Invalid startWork command: %v", err))
	}

	// Keep the output on screen until enter is pressed, and keep the exit code.
	script := buf.String() + "\n__ghd=$?\n" +
		`printf '\n[gh-dash] exit %s - press enter to return' "$__ghd"; read __ignored; exit $__ghd`
	log.Info("Starting work", "issue", issue.Number, "cmd", buf.String())

	issueCopy, status := *issue, pc.StartWork.StatusName()
	return tea.ExecProcess(exec.Command("sh", "-c", script), func(err error) tea.Msg {
		return startWorkFinishedMsg{issue: issueCopy, status: status, err: err}
	})
}

func (m *Model) onStartWorkFinished(msg startWorkFinishedMsg) tea.Cmd {
	if msg.err != nil {
		return m.notifyErr(fmt.Sprintf("Start work for #%d failed (%v); status unchanged",
			msg.issue.Number, msg.err))
	}
	sid := tasks.SectionIdentifier{}
	if m.ctx.View == config.IssuesView {
		sid = tasks.SectionIdentifier{Id: m.currSectionId, Type: issuessection.SectionType}
	}
	return tasks.SetIssueProjectField(m.ctx, sid, &msg.issue, tasks.ProjectFieldStatus, msg.status)
}
