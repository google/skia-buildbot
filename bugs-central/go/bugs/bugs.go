package bugs

// Defines a generic interface used by the different issue frameworks.

import (
	"context"
	"sync"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/sklog"
)

var (
	// Mapping of client to source to query to issues. Mirrors the DB structure but stores real issues instead of counts.
	// This will be used mainly by endpoints and emails that require the actual issue IDs.
	openIssues = map[types.RecognizedClient]map[types.IssueSource]map[string][]*types.Issue{}
	// Mutex to access to above object.
	openIssuesMutex sync.RWMutex
)

type BugFramework interface {

	// Search returns issues that match the provided parameters.
	Search(ctx context.Context, config interface{}) ([]*types.Issue, *types.IssueCountsData, error)

	// SearchClientAndPersist queries issues that match the provided client parameters.
	// The results should be put into the DB and into the openIssues in-memory object.
	SearchClientAndPersist(ctx context.Context, config interface{}, dbClient *db.FirestoreDB, runId string) error

	// GetIssueLink returns a link to the specified issue ID.
	GetIssueLink(project, id string) string
}

// PrettyPrintOpenIssues pretty prints the openIssues in-memory object.
func PrettyPrintOpenIssues() {
	openIssuesMutex.RLock()
	defer openIssuesMutex.RUnlock()

	sklog.Info("---- open issues ----")
	for c, sourceToQueries := range openIssues {
		sklog.Infof("%s", c)
		for s, queriesToIssues := range sourceToQueries {
			sklog.Infof("    %s", s)
			for q, issues := range queriesToIssues {
				sklog.Infof("        \"%s\"", q)
				sklog.Infof("            Open Issues: %d", len(issues))
			}
		}
	}
	sklog.Info("---------------------")
}

// putOpenIssues adds/removes data from the openIssues in-memory object. It is a convenient utility function that
// can be called from the different implementations of the BugFramework interface.
func putOpenIssues(client types.RecognizedClient, source types.IssueSource, query string, issues []*types.Issue) {
	openIssuesMutex.Lock()
	defer openIssuesMutex.Unlock()

	if sourceToQueries, ok := openIssues[client]; ok {
		if queryToIssues, ok := sourceToQueries[source]; ok {
			// Replace existing slice with new issues.
			queryToIssues[query] = issues
		} else {
			sourceToQueries[source] = map[string][]*types.Issue{
				query: issues,
			}
		}
	} else {
		openIssues[client] = map[types.IssueSource]map[string][]*types.Issue{
			source: {
				query: issues,
			},
		}
	}
}
