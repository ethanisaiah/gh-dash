package data

import (
	"cmp"
	"slices"
)

// LinkedPR is a pull request that will close an issue when merged.
type LinkedPR struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Url         string `json:"url"`
	State       string `json:"state"`
	HeadRefName string `json:"headRefName"`
	Repository  struct {
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

// SortLinkedPRs puts open PRs first, then the most recent (highest number).
func SortLinkedPRs(prs []LinkedPR) {
	slices.SortStableFunc(prs, func(a, b LinkedPR) int {
		aOpen, bOpen := a.State == "OPEN", b.State == "OPEN"
		if aOpen != bOpen {
			if aOpen {
				return -1
			}
			return 1
		}
		return cmp.Compare(b.Number, a.Number)
	})
}

// FetchLinkedPRs returns the PRs linked to an issue (by node id) through
// closing keywords or the Development sidebar, including closed ones.
func FetchLinkedPRs(issueId string) ([]LinkedPR, error) {
	c, err := graphQLClient()
	if err != nil {
		return nil, err
	}
	var res struct {
		Node struct {
			ClosedByPullRequestsReferences struct {
				Nodes []LinkedPR `json:"nodes"`
			} `json:"closedByPullRequestsReferences"`
		} `json:"node"`
	}
	err = c.Do(`query LinkedPRs($id: ID!) {
  node(id: $id) {
    ... on Issue {
      closedByPullRequestsReferences(first: 20, includeClosedPrs: true) {
        nodes { number title url state headRefName repository { nameWithOwner } }
      }
    }
  }
}`, map[string]any{"id": issueId}, &res)
	if err != nil {
		return nil, err
	}
	prs := res.Node.ClosedByPullRequestsReferences.Nodes
	SortLinkedPRs(prs)
	return prs, nil
}
