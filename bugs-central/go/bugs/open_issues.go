package bugs

// Defines an interface used by the different issue frameworks to add open issues to an in-memory object.

import (
	"sync"
	"time"

	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/sklog"
)

type OpenIssues struct {
	// Mapping of client to source to query to issues. Mirrors the DB structure but stores real issues instead of counts.
	// This will be used mainly by endpoints and emails that require the actual issue IDs.
	openIssues map[types.RecognizedClient]map[types.IssueSource]map[string][]*types.Issue
	// Mutex to access the above object.
	mtx sync.RWMutex
}

func InitOpenIssues() *OpenIssues {
	return &OpenIssues{
		openIssues: map[types.RecognizedClient]map[types.IssueSource]map[string][]*types.Issue{},
	}
}

// GetIssuesOutsideSLO returns all issues outside Skia's SLO mapped by priority.
func (o *OpenIssues) GetIssuesOutsideSLO(client types.RecognizedClient, source types.IssueSource, query string) map[types.StandardizedPriority][]*types.Issue {
	o.mtx.RLock()
	defer o.mtx.RUnlock()

	now := time.Now()
	priorityToSLOIssues := map[types.StandardizedPriority][]*types.Issue{}

	if sourceToQueries, ok := o.openIssues[client]; ok {
		if queryToIssues, ok := sourceToQueries[source]; ok {
			if issues, ok := queryToIssues[query]; ok {
				for _, i := range issues {
					if types.IsPrioritySLOViolation(now, i.CreatedTime, i.ModifiedTime, i.Priority) {
						// fmt.Println("IT IS AN SLO VIOLATION!")
						// fmt.Println(i.Id)
						// fmt.Println(i.Priority)
						if sloIssues, ok := priorityToSLOIssues[i.Priority]; ok {
							priorityToSLOIssues[i.Priority] = append(sloIssues, i)
						} else {
							priorityToSLOIssues[i.Priority] = []*types.Issue{i}
						}
					}
				}
			}
		}
	}
	return priorityToSLOIssues
}

// PrettyPrintOpenIssues pretty prints the open issues in-memory object.
func (o *OpenIssues) PrettyPrintOpenIssues() {
	o.mtx.RLock()
	defer o.mtx.RUnlock()

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
	o.mtx.Lock()
	defer o.mtx.Unlock()

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
