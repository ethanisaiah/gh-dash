package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
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
