package issuessection

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/common"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issuerow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/table"
)

const (
	noStatusGroup = "No Status"
	titleColumn   = 2
)

func (m *Model) fetchProjectIssues(limit int) (data.IssuesResponse, error) {
	pc := m.Ctx.Config.Project
	if pc == nil {
		return data.IssuesResponse{}, fmt.Errorf(
			"section %q has `source: project` but no `project` is configured", m.Config.Title)
	}
	project, err := data.FetchProject(pc.Owner, pc.Number, false)
	if err != nil {
		return data.IssuesResponse{}, err
	}
	return data.FetchProjectIssues(project, pc.StatusFieldName(), pc.IterationFieldName(),
		m.GetFilters(), limit, m.PageInfo)
}

// statusGroups groups issue indices by project status: first the statuses in
// the field's option order, then unknown statuses as they appear, then issues
// without a status. Empty groups are left out.
func statusGroups(statusOrder []string, statuses []string) (names []string, groups [][]int) {
	byStatus := map[string][]int{}
	var extra []string
	for i, s := range statuses {
		if s == "" {
			s = noStatusGroup
		}
		if _, seen := byStatus[s]; !seen && s != noStatusGroup {
			known := false
			for _, o := range statusOrder {
				if o == s {
					known = true
					break
				}
			}
			if !known {
				extra = append(extra, s)
			}
		}
		byStatus[s] = append(byStatus[s], i)
	}

	for _, name := range append(append(append([]string{}, statusOrder...), extra...), noStatusGroup) {
		if idx := byStatus[name]; len(idx) > 0 {
			names = append(names, name)
			groups = append(groups, idx)
			delete(byStatus, name)
		}
	}
	return names, groups
}

func (m *Model) statusOrder() []string {
	project := common.ConfiguredProject(m.Ctx.Config)
	if project == nil {
		return nil
	}
	field := project.FieldByName(m.Ctx.Config.Project.StatusFieldName())
	if field == nil {
		return nil
	}
	order := make([]string, 0, len(field.Options))
	for _, o := range field.Options {
		order = append(order, o.Name)
	}
	return order
}

func (m *Model) renderGroupHeader(name string, count int) table.Row {
	style := lipgloss.NewStyle().Bold(true).Foreground(m.Ctx.Theme.PrimaryText)
	if c := common.ProjectOptionColor(m.Ctx.Config, name); c != nil {
		style = style.Foreground(c)
	}
	row := make(table.Row, len(m.Columns))
	row[titleColumn] = style.Render(fmt.Sprintf("● %s", name)) +
		lipgloss.NewStyle().Foreground(m.Ctx.Theme.FaintText).Render(fmt.Sprintf("  %d", count))
	return row
}

// groupedRows builds the rows of a project section: a header row per status
// followed by its issues.
func (m *Model) groupedRows() ([]table.Row, map[int]bool, []int) {
	statuses := make([]string, len(m.Issues))
	for i, issue := range m.Issues {
		item, _ := data.GetProjectItem(issue.Id)
		statuses[i] = item.StatusName()
	}
	names, groups := statusGroups(m.statusOrder(), statuses)

	rows := []table.Row{}
	headers := map[int]bool{}
	rowIssues := []int{}
	for g, name := range names {
		headers[len(rows)] = true
		rows = append(rows, m.renderGroupHeader(name, len(groups[g])))
		rowIssues = append(rowIssues, -1)
		for _, i := range groups[g] {
			issue := issuerow.Issue{Ctx: m.Ctx, Data: m.Issues[i], ShowAuthorIcon: m.ShowAuthorIcon}
			rows = append(rows, issue.ToTableRow())
			rowIssues = append(rowIssues, i)
		}
	}
	return rows, headers, rowIssues
}

// syncRows rebuilds the table. In project sections the cursor stays on the
// same issue even if it moved to another group.
func (m *Model) syncRows() {
	if !m.fromProject {
		m.rowIssues = nil
		m.Table.SetRows(m.BuildRows())
		return
	}

	selectedUrl := ""
	if curr := m.GetCurrRow(); curr != nil {
		selectedUrl = curr.GetUrl()
	}
	rows, headers, rowIssues := m.groupedRows()
	m.rowIssues = rowIssues
	m.Table.SetRowsWithHeaders(rows, headers)

	if selectedUrl == "" {
		return
	}
	for row, i := range rowIssues {
		if i >= 0 && m.Issues[i].Url == selectedUrl {
			m.moveCursorTo(row)
			return
		}
	}
}

func (m *Model) moveCursorTo(row int) {
	if m.Table.GetCurrItem() == row {
		return
	}
	m.Table.FirstItem()
	for m.Table.GetCurrItem() < row {
		before := m.Table.GetCurrItem()
		if m.Table.NextItem() == before {
			return
		}
	}
}

// currIssuePosition is the 1-based position of the selected issue among the
// issues, not counting group header rows.
func (m *Model) currIssuePosition() int {
	row := m.Table.GetCurrItem()
	if m.rowIssues == nil {
		return row + 1
	}
	pos := 0
	for i := 0; i <= row && i < len(m.rowIssues); i++ {
		if m.rowIssues[i] >= 0 {
			pos++
		}
	}
	return pos
}
