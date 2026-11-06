package data

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"charm.land/log/v2"
	gh "github.com/cli/go-gh/v2/pkg/api"
	graphql "github.com/cli/shurcooL-graphql"
)

// ProjectOption is one option of a single-select project field.
type ProjectOption struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
}

// ProjectIteration is one iteration of a project iteration field.
type ProjectIteration struct {
	Id        string `json:"id"`
	Title     string `json:"title"`
	StartDate string `json:"startDate"`
	Duration  int    `json:"duration"`
}

const projectDateLayout = "2006-01-02"

func (it ProjectIteration) Start() time.Time {
	t, err := time.ParseInLocation(projectDateLayout, it.StartDate, time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (it ProjectIteration) End() time.Time {
	return it.Start().AddDate(0, 0, it.Duration)
}

// Contains reports whether t falls inside the iteration, by calendar day.
func (it ProjectIteration) Contains(t time.Time) bool {
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
	return !day.Before(it.Start()) && day.Before(it.End())
}

type ProjectIterationConfig struct {
	Iterations          []ProjectIteration `json:"iterations"`
	CompletedIterations []ProjectIteration `json:"completedIterations"`
}

// ProjectField is a project field. Options is set for single-select fields and
// Configuration for iteration fields.
type ProjectField struct {
	Typename      string                  `json:"__typename"`
	Id            string                  `json:"id"`
	Name          string                  `json:"name"`
	Options       []ProjectOption         `json:"options"`
	Configuration *ProjectIterationConfig `json:"configuration"`
}

func (f *ProjectField) IsSingleSelect() bool {
	return f != nil && f.Typename == "ProjectV2SingleSelectField"
}

func (f *ProjectField) IsIteration() bool {
	return f != nil && f.Typename == "ProjectV2IterationField"
}

// OptionByName finds a single-select option, ignoring case.
func (f *ProjectField) OptionByName(name string) *ProjectOption {
	if f == nil {
		return nil
	}
	for i := range f.Options {
		if strings.EqualFold(f.Options[i].Name, strings.TrimSpace(name)) {
			return &f.Options[i]
		}
	}
	return nil
}

// ActiveIterations returns the current and upcoming iterations, in order.
// GitHub lists these in `iterations`; finished ones are in `completedIterations`.
func (f *ProjectField) ActiveIterations() []ProjectIteration {
	if f == nil || f.Configuration == nil {
		return nil
	}
	return f.Configuration.Iterations
}

// IterationByTitle finds an active or completed iteration, ignoring case.
func (f *ProjectField) IterationByTitle(title string) *ProjectIteration {
	if f == nil || f.Configuration == nil {
		return nil
	}
	title = strings.TrimSpace(title)
	for _, list := range [][]ProjectIteration{
		f.Configuration.Iterations,
		f.Configuration.CompletedIterations,
	} {
		for i := range list {
			if strings.EqualFold(list[i].Title, title) {
				return &list[i]
			}
		}
	}
	return nil
}

// CurrentIteration returns the iteration containing now, if any.
func (f *ProjectField) CurrentIteration(now time.Time) *ProjectIteration {
	for _, it := range f.ActiveIterations() {
		if it.Contains(now) {
			return &it
		}
	}
	return nil
}

// Project is the metadata of a GitHub Projects (v2) board.
type Project struct {
	Id     string `json:"id"`
	Title  string `json:"title"`
	Number int    `json:"number"`
	Url    string `json:"url"`
	Fields struct {
		Nodes []ProjectField `json:"nodes"`
	} `json:"fields"`
}

// FieldByName finds a field, ignoring case.
func (p *Project) FieldByName(name string) *ProjectField {
	if p == nil {
		return nil
	}
	for i := range p.Fields.Nodes {
		if strings.EqualFold(p.Fields.Nodes[i].Name, name) {
			return &p.Fields.Nodes[i]
		}
	}
	return nil
}

// ProjectFieldValue is the value of a single-select or iteration field on an
// item. Id is the option or iteration id.
type ProjectFieldValue struct {
	Id        string
	Name      string
	StartDate string
	Duration  int
}

// ProjectItem is an issue's membership in the configured project.
type ProjectItem struct {
	ItemId    string
	Status    *ProjectFieldValue
	Iteration *ProjectFieldValue
}

func (it *ProjectItem) StatusName() string {
	if it == nil || it.Status == nil {
		return ""
	}
	return it.Status.Name
}

func (it *ProjectItem) IterationTitle() string {
	if it == nil || it.Iteration == nil {
		return ""
	}
	return it.Iteration.Name
}

var projectStore = struct {
	sync.RWMutex
	projects map[string]*Project
	items    map[string]ProjectItem
	fetched  map[string]bool
}{
	projects: map[string]*Project{},
	items:    map[string]ProjectItem{},
	fetched:  map[string]bool{},
}

// GetProjectItem returns the cached project item of an issue (by node id). The
// second result is false if the issue's project data was never fetched;
// ok && item == nil means the issue is not in the project.
func GetProjectItem(contentId string) (item *ProjectItem, ok bool) {
	projectStore.RLock()
	defer projectStore.RUnlock()
	if !projectStore.fetched[contentId] {
		return nil, false
	}
	if it, found := projectStore.items[contentId]; found {
		return &it, true
	}
	return nil, true
}

// SetProjectItem updates the cache after a local change.
func SetProjectItem(contentId string, item *ProjectItem) {
	projectStore.Lock()
	defer projectStore.Unlock()
	projectStore.fetched[contentId] = true
	if item == nil {
		delete(projectStore.items, contentId)
		return
	}
	projectStore.items[contentId] = *item
}

// CachedProject returns a project loaded earlier by FetchProject, or nil.
func CachedProject(owner string, number int) *Project {
	projectStore.RLock()
	defer projectStore.RUnlock()
	return projectStore.projects[fmt.Sprintf("%s/%d", owner, number)]
}

func graphQLClient() (*gh.GraphQLClient, error) {
	var err error
	if client == nil {
		client, err = gh.DefaultGraphQLClient()
	}
	return client, err
}

const projectQuery = `query ProjectMeta($owner: String!, $number: Int!) {
  repositoryOwner(login: $owner) {
    ... on ProjectV2Owner {
      projectV2(number: $number) {
        id title number url
        fields(first: 50) {
          nodes {
            __typename
            ... on ProjectV2FieldCommon { id name }
            ... on ProjectV2SingleSelectField { options { id name color description } }
            ... on ProjectV2IterationField {
              configuration {
                iterations { id title startDate duration }
                completedIterations { id title startDate duration }
              }
            }
          }
        }
      }
    }
  }
}`

// FetchProject loads a project's metadata. It is cached for the session;
// pass refresh to reload it (e.g. after new iterations are created).
func FetchProject(owner string, number int, refresh bool) (*Project, error) {
	key := fmt.Sprintf("%s/%d", owner, number)
	projectStore.RLock()
	cached := projectStore.projects[key]
	projectStore.RUnlock()
	if cached != nil && !refresh {
		return cached, nil
	}

	c, err := graphQLClient()
	if err != nil {
		return nil, err
	}
	var res struct {
		RepositoryOwner *struct {
			ProjectV2 *Project `json:"projectV2"`
		} `json:"repositoryOwner"`
	}
	log.Debug("Fetching project", "owner", owner, "number", number)
	err = c.Do(projectQuery, map[string]any{"owner": owner, "number": number}, &res)
	if err != nil {
		return nil, err
	}
	if res.RepositoryOwner == nil || res.RepositoryOwner.ProjectV2 == nil {
		return nil, fmt.Errorf("project %s not found", key)
	}

	projectStore.Lock()
	projectStore.projects[key] = res.RepositoryOwner.ProjectV2
	projectStore.Unlock()
	return res.RepositoryOwner.ProjectV2, nil
}

const projectItemsQuery = `query ProjectItems($ids: [ID!]!, $status: String!, $iteration: String!) {
  nodes(ids: $ids) {
    ... on Issue { id projectItems(first: 20) { ...items } }
    ... on PullRequest { id projectItems(first: 20) { ...items } }
  }
}
fragment items on ProjectV2ItemConnection {
  nodes {
    id
    project { id }
    status: fieldValueByName(name: $status) {
      ... on ProjectV2ItemFieldSingleSelectValue { optionId name }
    }
    iteration: fieldValueByName(name: $iteration) {
      ... on ProjectV2ItemFieldIterationValue { iterationId title startDate duration }
    }
  }
}`

type projectItemNode struct {
	Id      string `json:"id"`
	Project struct {
		Id string `json:"id"`
	} `json:"project"`
	Status *struct {
		OptionId string `json:"optionId"`
		Name     string `json:"name"`
	} `json:"status"`
	Iteration *struct {
		IterationId string `json:"iterationId"`
		Title       string `json:"title"`
		StartDate   string `json:"startDate"`
		Duration    int    `json:"duration"`
	} `json:"iteration"`
}

func (n projectItemNode) toProjectItem() ProjectItem {
	item := ProjectItem{ItemId: n.Id}
	if n.Status != nil && n.Status.OptionId != "" {
		item.Status = &ProjectFieldValue{Id: n.Status.OptionId, Name: n.Status.Name}
	}
	if n.Iteration != nil && n.Iteration.IterationId != "" {
		item.Iteration = &ProjectFieldValue{
			Id:        n.Iteration.IterationId,
			Name:      n.Iteration.Title,
			StartDate: n.Iteration.StartDate,
			Duration:  n.Iteration.Duration,
		}
	}
	return item
}

// FetchProjectItems loads the project items of issues or PRs (by node id) and
// caches them. Only items belonging to project are kept.
func FetchProjectItems(
	project *Project,
	statusField, iterationField string,
	contentIds []string,
) error {
	c, err := graphQLClient()
	if err != nil {
		return err
	}
	const chunk = 50
	for start := 0; start < len(contentIds); start += chunk {
		ids := contentIds[start:min(start+chunk, len(contentIds))]
		var res struct {
			Nodes []*struct {
				Id           string `json:"id"`
				ProjectItems struct {
					Nodes []projectItemNode `json:"nodes"`
				} `json:"projectItems"`
			} `json:"nodes"`
		}
		err := c.Do(projectItemsQuery, map[string]any{
			"ids":       ids,
			"status":    statusField,
			"iteration": iterationField,
		}, &res)
		if err != nil {
			return err
		}

		projectStore.Lock()
		for _, node := range res.Nodes {
			if node == nil || node.Id == "" {
				continue
			}
			projectStore.fetched[node.Id] = true
			delete(projectStore.items, node.Id)
			for _, pi := range node.ProjectItems.Nodes {
				if pi.Project.Id == project.Id {
					projectStore.items[node.Id] = pi.toProjectItem()
					break
				}
			}
		}
		projectStore.Unlock()
	}
	log.Debug("Fetched project items", "project", project.Title, "count", len(contentIds))
	return nil
}

// AddToProject adds an issue or PR to a project and returns the item id. It
// is a no-op returning the existing item if it is already in the project.
func AddToProject(projectId, contentId string) (string, error) {
	c, err := graphQLClient()
	if err != nil {
		return "", err
	}
	var res struct {
		AddProjectV2ItemById struct {
			Item struct {
				Id string `json:"id"`
			} `json:"item"`
		} `json:"addProjectV2ItemById"`
	}
	err = c.Do(`mutation AddToProject($project: ID!, $content: ID!) {
  addProjectV2ItemById(input: {projectId: $project, contentId: $content}) { item { id } }
}`, map[string]any{"project": projectId, "content": contentId}, &res)
	if err != nil {
		return "", err
	}
	return res.AddProjectV2ItemById.Item.Id, nil
}

// ProjectFieldUpdate is a new value for one field. Exactly one of OptionId and
// IterationId should be set; neither set clears the field.
type ProjectFieldUpdate struct {
	FieldId     string
	OptionId    string
	IterationId string
}

// UpdateProjectItemField sets or clears a single-select or iteration field.
func UpdateProjectItemField(projectId, itemId string, u ProjectFieldUpdate) error {
	c, err := graphQLClient()
	if err != nil {
		return err
	}
	vars := map[string]any{"project": projectId, "item": itemId, "field": u.FieldId}
	var res struct{}

	switch {
	case u.OptionId != "":
		vars["option"] = u.OptionId
		return c.Do(`mutation SetSingleSelect($project: ID!, $item: ID!, $field: ID!, $option: String!) {
  updateProjectV2ItemFieldValue(input: {projectId: $project, itemId: $item, fieldId: $field,
    value: {singleSelectOptionId: $option}}) { projectV2Item { id } }
}`, vars, &res)
	case u.IterationId != "":
		vars["iteration"] = u.IterationId
		return c.Do(`mutation SetIteration($project: ID!, $item: ID!, $field: ID!, $iteration: String!) {
  updateProjectV2ItemFieldValue(input: {projectId: $project, itemId: $item, fieldId: $field,
    value: {iterationId: $iteration}}) { projectV2Item { id } }
}`, vars, &res)
	default:
		return c.Do(`mutation ClearField($project: ID!, $item: ID!, $field: ID!) {
  clearProjectV2ItemFieldValue(input: {projectId: $project, itemId: $item, fieldId: $field}) {
    projectV2Item { id }
  }
}`, vars, &res)
	}
}

type projectIssuesQuery struct {
	Node struct {
		ProjectV2 struct {
			Items struct {
				Nodes []struct {
					Id     string
					Status struct {
						SingleSelect struct {
							OptionId string
							Name     string
						} `graphql:"... on ProjectV2ItemFieldSingleSelectValue"`
					} `graphql:"status: fieldValueByName(name: $statusField)"`
					Iteration struct {
						Iteration struct {
							IterationId string
							Title       string
							StartDate   string
							Duration    int
						} `graphql:"... on ProjectV2ItemFieldIterationValue"`
					} `graphql:"iteration: fieldValueByName(name: $iterationField)"`
					Content struct {
						Issue IssueData `graphql:"... on Issue"`
					}
				}
				TotalCount int
				PageInfo   PageInfo
			} `graphql:"items(first: $limit, after: $endCursor, query: $query)"`
		} `graphql:"... on ProjectV2"`
	} `graphql:"node(id: $projectId)"`
}

// FetchProjectIssues loads the issues of a project matching a project filter
// (the same syntax as the filter bar on github.com, e.g. `-status:Merged
// assignee:@me`). Pull requests and draft issues are skipped. Each issue's
// project item is cached.
func FetchProjectIssues(
	project *Project,
	statusField, iterationField string,
	filter string,
	limit int,
	pageInfo *PageInfo,
) (IssuesResponse, error) {
	c, err := graphQLClient()
	if err != nil {
		return IssuesResponse{}, err
	}

	var endCursor *graphql.String
	if pageInfo != nil {
		endCursor = (*graphql.String)(&pageInfo.EndCursor)
	}
	query := strings.TrimSpace("is:issue " + filter)
	variables := map[string]any{
		"projectId":      graphql.ID(project.Id),
		"statusField":    graphql.String(statusField),
		"iterationField": graphql.String(iterationField),
		"query":          graphql.String(query),
		"limit":          graphql.Int(limit),
		"endCursor":      endCursor,
	}
	var res projectIssuesQuery
	log.Debug("Fetching project issues", "project", project.Title, "query", query)
	if err := c.Query("ProjectIssues", &res, variables); err != nil {
		return IssuesResponse{}, err
	}

	items := res.Node.ProjectV2.Items
	issues := make([]IssueData, 0, len(items.Nodes))
	projectStore.Lock()
	for _, node := range items.Nodes {
		issue := node.Content.Issue
		if issue.Id == "" {
			continue
		}
		item := ProjectItem{ItemId: node.Id}
		if s := node.Status.SingleSelect; s.OptionId != "" {
			item.Status = &ProjectFieldValue{Id: s.OptionId, Name: s.Name}
		}
		if it := node.Iteration.Iteration; it.IterationId != "" {
			item.Iteration = &ProjectFieldValue{
				Id: it.IterationId, Name: it.Title, StartDate: it.StartDate, Duration: it.Duration,
			}
		}
		projectStore.fetched[issue.Id] = true
		projectStore.items[issue.Id] = item
		issues = append(issues, issue)
	}
	projectStore.Unlock()
	log.Info("Fetched project issues", "project", project.Title, "count", len(issues))

	return IssuesResponse{
		Issues:     issues,
		TotalCount: items.TotalCount,
		PageInfo:   items.PageInfo,
	}, nil
}
