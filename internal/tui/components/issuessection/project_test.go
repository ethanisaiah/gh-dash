package issuessection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func TestStatusGroups(t *testing.T) {
	order := []string{"Backlog", "Ready to Build", "In Progress", "Merged"}
	statuses := []string{"In Progress", "", "Backlog", "Legacy", "In Progress", "Merged"}

	names, groups := statusGroups(order, statuses)

	require.Equal(t, []string{"Backlog", "In Progress", "Merged", "Legacy", "No Status"}, names)
	require.Equal(t, [][]int{{2}, {0, 4}, {5}, {3}, {1}}, groups)
}

func TestStatusGroupsWithoutProjectMetadata(t *testing.T) {
	names, groups := statusGroups(nil, []string{"B", "A", "B"})
	require.Equal(t, []string{"B", "A"}, names)
	require.Equal(t, [][]int{{0, 2}, {1}}, groups)
}

func TestSyncRowsKeepsSelectedIssueWhenItChangesGroup(t *testing.T) {
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	thm := theme.ParseTheme(&cfg)
	ctx := &context.ProgramContext{Config: &cfg, Theme: thm, Styles: context.InitStyles(thm)}

	m := NewModel(0, ctx, config.IssuesSectionConfig{Title: "Board", Source: "project"},
		time.Now(), time.Now())
	m.Issues = []data.IssueData{
		{Id: "I_a", Number: 1, Url: "u/1"},
		{Id: "I_b", Number: 2, Url: "u/2"},
		{Id: "I_c", Number: 3, Url: "u/3"},
	}
	set := func(id, status string) {
		data.SetProjectItem(id, &data.ProjectItem{
			ItemId: "PVTI_" + id, Status: &data.ProjectFieldValue{Name: status},
		})
	}
	set("I_a", "Backlog")
	set("I_b", "Backlog")
	set("I_c", "Reviewing")

	// rows: [Backlog] a b [Reviewing] c
	m.syncRows()
	require.Equal(t, 1, m.GetCurrRow().GetNumber())
	m.NextRow()
	require.Equal(t, 2, m.GetCurrRow().GetNumber())

	// b moves to Reviewing -> rows: [Backlog] a [Reviewing] b c
	set("I_b", "Reviewing")
	m.syncRows()
	require.Equal(t, 2, m.GetCurrRow().GetNumber(), "cursor follows the issue")
	require.Equal(t, 3, m.Table.GetCurrItem())
}
