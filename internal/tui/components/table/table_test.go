package table

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func newTestTable(t *testing.T) Model {
	t.Helper()
	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../../../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	thm := theme.ParseTheme(&cfg)
	ctx := context.ProgramContext{Config: &cfg, Theme: thm, Styles: context.InitStyles(thm)}
	return NewModel(ctx, constants.Dimensions{Width: 80, Height: 40},
		time.Now(), time.Now(), []Column{{Title: "Title"}}, nil, "item", nil, "", false)
}

// rows: 0 header, 1, 2, 3 header, 4 header, 5, 6 header
func TestHeaderRowsAreSkipped(t *testing.T) {
	m := newTestTable(t)
	rows := []Row{{"A"}, {"a1"}, {"a2"}, {"B"}, {"C"}, {"c1"}, {"D"}}
	m.SetRowsWithHeaders(rows, map[int]bool{0: true, 3: true, 4: true, 6: true})

	require.Equal(t, 1, m.GetCurrItem(), "cursor starts on first selectable row")
	require.Equal(t, 2, m.NextItem())
	require.Equal(t, 5, m.NextItem(), "skips consecutive headers")
	require.Equal(t, 5, m.NextItem(), "trailing header is not selectable")
	require.Equal(t, 2, m.PrevItem())
	require.Equal(t, 1, m.PrevItem())
	require.Equal(t, 1, m.PrevItem(), "leading header is not selectable")
	require.Equal(t, 5, m.LastItem())
	require.Equal(t, 1, m.FirstItem())
}

func TestSetRowsClearsHeaders(t *testing.T) {
	m := newTestTable(t)
	m.SetRowsWithHeaders([]Row{{"A"}, {"a1"}}, map[int]bool{0: true})
	m.FirstItem()
	m.SetRows([]Row{{"x"}, {"y"}})
	require.False(t, m.IsHeaderRow(0))
	require.Equal(t, 0, m.FirstItem())
}
