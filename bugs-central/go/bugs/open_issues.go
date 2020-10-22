package bugs

// Defines an interface used by the different issue frameworks to add open issues to an in-memory object.

import (
	"sync"

	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/sklog"
)

var (
	// Mutex to access the open issues object.
	openIssuesMutex sync.RWMutex
)

type OpenIssues struct {
	// Mapping of client to source to query to issues. Mirrors the DB structure but stores real issues instead of counts.
	// This will be used mainly by endpoints and emails that require the actual issue IDs.
	openIssues map[types.RecognizedClient]map[types.IssueSource]map[string][]*types.Issue
}

func InitOpenIssues() OpenIssues {
	return OpenIssues{
		openIssues: map[types.RecognizedClient]map[types.IssueSource]map[string][]*types.Issue{},
	}
}

// PrettyPrintOpenIssues pretty prints the open issues in-memory object.
func (o *OpenIssues) PrettyPrintOpenIssues() {
	openIssuesMutex.RLock()
	defer openIssuesMutex.RUnlock()

	sklog.Info("---- open issues ----")
	for c, sourceToQueries := range o.openIssues {
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

// PutOpenIssues adds/removes data from the open issues in-memory object.
func (o *OpenIssues) PutOpenIssues(client types.RecognizedClient, source types.IssueSource, query string, issues []*types.Issue) {
	openIssuesMutex.Lock()
	defer openIssuesMutex.Unlock()

	if sourceToQueries, ok := o.openIssues[client]; ok {
		if queryToIssues, ok := sourceToQueries[source]; ok {
			// Replace existing slice with new issues.
			queryToIssues[query] = issues
		} else {
			sourceToQueries[source] = map[string][]*types.Issue{
				query: issues,
			}
		}
	} else {
		o.openIssues[client] = map[types.IssueSource]map[string][]*types.Issue{
			source: {
				query: issues,
			},
		}
	}
}
