package common

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
)

// ConfiguredProject returns the configured project's metadata if it has been
// loaded, or nil when no project is configured or it is not loaded yet.
func ConfiguredProject(cfg *config.Config) *data.Project {
	if cfg == nil || cfg.Project == nil {
		return nil
	}
	return data.CachedProject(cfg.Project.Owner, cfg.Project.Number)
}

// LoadProjectItems makes sure the configured project's metadata is loaded and
// fetches the project items of the given issues/PRs (by node id).
func LoadProjectItems(pc *config.ProjectConfig, contentIds []string) (*data.Project, error) {
	project, err := data.FetchProject(pc.Owner, pc.Number, false)
	if err != nil {
		return nil, err
	}
	if len(contentIds) == 0 {
		return project, nil
	}
	err = data.FetchProjectItems(project, pc.StatusFieldName(), pc.IterationFieldName(), contentIds)
	return project, err
}

// projectColors maps GitHub project option colors to terminal-friendly hex
// values (GitHub's dark theme palette).
var projectColors = map[string]string{
	"GRAY":   "#8b949e",
	"BLUE":   "#58a6ff",
	"GREEN":  "#3fb950",
	"YELLOW": "#d29922",
	"ORANGE": "#db6d28",
	"RED":    "#f85149",
	"PINK":   "#db61a2",
	"PURPLE": "#a371f7",
}

// ProjectOptionColor returns the color of a single-select option of the
// configured status field, or nil if unknown.
func ProjectOptionColor(cfg *config.Config, optionName string) color.Color {
	project := ConfiguredProject(cfg)
	if project == nil {
		return nil
	}
	opt := project.FieldByName(cfg.Project.StatusFieldName()).OptionByName(optionName)
	if opt == nil {
		return nil
	}
	hex, ok := projectColors[strings.ToUpper(opt.Color)]
	if !ok {
		return nil
	}
	return lipgloss.Color(hex)
}
