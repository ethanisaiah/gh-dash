package tasks

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/log/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/constants"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
)

// ProjectFieldKind says which configured project field to change.
type ProjectFieldKind int

const (
	ProjectFieldStatus ProjectFieldKind = iota
	ProjectFieldIteration
)

// UpdateProjectItemMsg is sent after an issue's project item changed. The new
// values are already in the data package's project item cache.
type UpdateProjectItemMsg struct {
	ContentId   string
	IssueNumber int
}

// setProjectField resolves a field and value by name and writes it, adding
// the issue to the project first if it is not in it. An empty value clears
// the field. It updates the project item cache.
func setProjectField(
	pc *config.ProjectConfig,
	contentId string,
	kind ProjectFieldKind,
	value string,
) error {
	project, err := data.FetchProject(pc.Owner, pc.Number, false)
	if err != nil {
		return err
	}

	fieldName := pc.StatusFieldName()
	if kind == ProjectFieldIteration {
		fieldName = pc.IterationFieldName()
	}
	field := project.FieldByName(fieldName)
	if field == nil {
		return fmt.Errorf("project %q has no field named %q", project.Title, fieldName)
	}

	update := data.ProjectFieldUpdate{FieldId: field.Id}
	var newValue *data.ProjectFieldValue
	value = strings.TrimSpace(value)
	if value != "" {
		switch {
		case kind == ProjectFieldStatus && field.IsSingleSelect():
			opt := field.OptionByName(value)
			if opt == nil {
				return fmt.Errorf("%q is not a %s option", value, field.Name)
			}
			update.OptionId = opt.Id
			newValue = &data.ProjectFieldValue{Id: opt.Id, Name: opt.Name}
		case kind == ProjectFieldIteration && field.IsIteration():
			it := field.IterationByTitle(value)
			if it == nil {
				return fmt.Errorf("%q is not a %s", value, field.Name)
			}
			update.IterationId = it.Id
			newValue = &data.ProjectFieldValue{
				Id: it.Id, Name: it.Title, StartDate: it.StartDate, Duration: it.Duration,
			}
		default:
			return fmt.Errorf("field %q has an unsupported type %s", field.Name, field.Typename)
		}
	}

	item, fetched := data.GetProjectItem(contentId)
	if !fetched {
		if err := data.FetchProjectItems(project, pc.StatusFieldName(),
			pc.IterationFieldName(), []string{contentId}); err != nil {
			return err
		}
		item, _ = data.GetProjectItem(contentId)
	}
	if item == nil {
		if newValue == nil {
			return nil // clearing a field of an issue not in the project
		}
		log.Info("Adding to project", "project", project.Title, "content", contentId)
		itemId, err := data.AddToProject(project.Id, contentId)
		if err != nil {
			return err
		}
		item = &data.ProjectItem{ItemId: itemId}
	}

	if err := data.UpdateProjectItemField(project.Id, item.ItemId, update); err != nil {
		return err
	}

	updated := *item
	if kind == ProjectFieldStatus {
		updated.Status = newValue
	} else {
		updated.Iteration = newValue
	}
	data.SetProjectItem(contentId, &updated)
	return nil
}

func describe(kind ProjectFieldKind, pc *config.ProjectConfig) string {
	if kind == ProjectFieldIteration {
		return pc.IterationFieldName()
	}
	return pc.StatusFieldName()
}

// fireFuncTask runs fn in the background and reports it like a GitHubTask.
func fireFuncTask(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	id, startText, finishedText string,
	fn func() (tea.Msg, error),
) tea.Cmd {
	startCmd := ctx.StartTask(context.Task{
		Id:           id,
		StartText:    startText,
		FinishedText: finishedText,
		State:        context.TaskStart,
	})
	return tea.Batch(startCmd, func() tea.Msg {
		msg, err := fn()
		return constants.TaskFinishedMsg{
			TaskId:      id,
			SectionId:   section.Id,
			SectionType: section.Type,
			Err:         err,
			Msg:         msg,
		}
	})
}

// SetIssueProjectField sets an issue's Status or Iteration in the configured
// project, adding the issue to the project if needed.
func SetIssueProjectField(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	issue *data.IssueData,
	kind ProjectFieldKind,
	value string,
) tea.Cmd {
	pc := ctx.Config.Project
	if pc == nil {
		return nil
	}
	field := describe(kind, pc)
	shown := value
	if shown == "" {
		shown = "(none)"
	}
	return fireFuncTask(ctx, section,
		fmt.Sprintf("issue_project_%s_%d", strings.ToLower(field), issue.Number),
		fmt.Sprintf("Setting %s of #%d to %s", field, issue.Number, shown),
		fmt.Sprintf("%s of #%d set to %s", field, issue.Number, shown),
		func() (tea.Msg, error) {
			err := setProjectField(pc, issue.Id, kind, value)
			return UpdateProjectItemMsg{ContentId: issue.Id, IssueNumber: issue.Number}, err
		},
	)
}

// AssigneeChanges returns who to add and remove to go from current to wanted.
func AssigneeChanges(current, wanted []string) (add, remove []string) {
	for _, w := range wanted {
		if !slices.Contains(current, w) && !slices.Contains(add, w) {
			add = append(add, w)
		}
	}
	for _, c := range current {
		if !slices.Contains(wanted, c) {
			remove = append(remove, c)
		}
	}
	return add, remove
}

// HandoffIssue sets an issue's Status and then replaces its assignees, for
// workflows where the assignee is whoever's turn it is. Assignees are only
// changed if setting the status succeeded.
func HandoffIssue(
	ctx *context.ProgramContext,
	section SectionIdentifier,
	issue *data.IssueData,
	status string,
	assignees []string,
) tea.Cmd {
	pc := ctx.Config.Project
	if pc == nil {
		return nil
	}
	current := make([]string, 0, len(issue.Assignees.Nodes))
	for _, a := range issue.Assignees.Nodes {
		current = append(current, a.Login)
	}
	add, remove := AssigneeChanges(current, assignees)
	who := strings.Join(assignees, ", ")
	if who == "" {
		who = "nobody"
	}

	return fireFuncTask(ctx, section,
		fmt.Sprintf("issue_handoff_%d", issue.Number),
		fmt.Sprintf("Moving #%d to %s for %s", issue.Number, status, who),
		fmt.Sprintf("#%d is %s, assigned to %s", issue.Number, status, who),
		func() (tea.Msg, error) {
			if err := setProjectField(pc, issue.Id, ProjectFieldStatus, status); err != nil {
				return nil, err
			}
			msg := UpdateIssueMsg{IssueNumber: issue.Number}
			if len(add) == 0 && len(remove) == 0 {
				return msg, nil
			}

			args := []string{
				"issue", "edit", fmt.Sprint(issue.Number),
				"-R", issue.GetRepoNameWithOwner(),
			}
			for _, a := range add {
				args = append(args, "--add-assignee", a)
			}
			for _, r := range remove {
				args = append(args, "--remove-assignee", r)
			}
			log.Info("Running task", "cmd", "gh "+strings.Join(args, " "))
			if out, err := exec.Command("gh", args...).CombinedOutput(); err != nil {
				return msg, fmt.Errorf("status set, but reassigning failed: %w: %s",
					err, strings.TrimSpace(string(out)))
			}

			msg.AddedAssignees = toAssignees(add)
			msg.RemovedAssignees = toAssignees(remove)
			return msg, nil
		},
	)
}

func toAssignees(logins []string) *data.Assignees {
	res := &data.Assignees{Nodes: []data.Assignee{}}
	for _, l := range logins {
		res.Nodes = append(res.Nodes, data.Assignee{Login: l})
	}
	return res
}
