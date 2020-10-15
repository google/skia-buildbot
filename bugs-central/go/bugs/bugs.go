package bugs

// A generic interface used by the different issue frameworks.

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/sklog"
)

var (
	// Mapping of client to source to query to issues. Mirrors the DB structure but stores real issues instead of counts.
	// This will be used for emailing.
	openIssues = map[types.RecognizedClient]map[types.IssueSource]map[string][]*Issue{}
	// Mutex to access to above object.
	openIssuesMutex sync.RWMutex
)

type Issue struct {
	Id       string                     `json:"id"`
	State    string                     `json:"state"`
	Priority types.StandardizedPriority `json:"priority"`
	Owner    string                     `json:"owner"`
	Link     string                     `json:"link"`

	CreatedTime  time.Time `json:"created"`
	ModifiedTime time.Time `json:"modified"`

	Title   string `json:"title"`   // This is not populated in IssueTracker.
	Summary string `json:"summary"` // This is not returned in IssueTracker or Monorail.
}

const (
	// All recognized clients.
	AndroidClient       types.RecognizedClient = "Android"
	ChromiumClient      types.RecognizedClient = "Chromium"
	FlutterNativeClient types.RecognizedClient = "Flutter-native"
	FlutterOnWebClient  types.RecognizedClient = "Flutter-on-web"
	SkiaClient          types.RecognizedClient = "Skia"

	// Supported issue sources.
	GithubSource       types.IssueSource = "Github"
	IssueTrackerSource types.IssueSource = "Buganizer"
	MonorailSource     types.IssueSource = "Monorail"

	// // All bug frameworks will be standardized to these priorities.
	// PriorityP0 types.StandardizedPriority = "P0"
	// PriorityP1 types.StandardizedPriority = "P1"
	// PriorityP2 types.StandardizedPriority = "P2"
	// PriorityP3 types.StandardizedPriority = "P3"
	// PriorityP4 types.StandardizedPriority = "P4"
	// PriorityP5 types.StandardizedPriority = "P5"
	// PriorityP6 types.StandardizedPriority = "P6"
)

type BugFramework interface {

	// Search returns issues that match the provided parameters.
	Search(ctx context.Context, config interface{}) ([]*Issue, *types.IssueCountsData, error)

	// SearchClientAndPersist queries issues that match the provided client parameters.
	// The results should be put into the DB and into the openIssues in-memory object.
	SearchClientAndPersist(ctx context.Context, config interface{}, dbClient *db.FirestoreDB, runId string) error

	// GetIssueLink returns a link to the specified issue ID.
	GetIssueLink(project, id string) string
}

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
				sklog.Infof("            %d", len(issues))
			}
		}
	}
	sklog.Info("---------------------")
}

func putOpenIssues(client types.RecognizedClient, source types.IssueSource, query string, issues []*Issue) {
	openIssuesMutex.Lock()
	defer openIssuesMutex.Unlock()

	if sourceToQueries, ok := openIssues[client]; ok {
		if queryToIssues, ok := sourceToQueries[source]; ok {
			// Replace existing slice with new issues.
			queryToIssues[query] = issues
		} else {
			sourceToQueries[source] = map[string][]*Issue{
				query: issues,
			}
		}
	} else {
		openIssues[client] = map[types.IssueSource]map[string][]*Issue{
			source: map[string][]*Issue{
				query: issues,
			},
		}
	}
}

// Need tests for this and the above.
// Is returning better or are all these branches better? branche are hard to read...
// func GetCountsFromOpenIssues(client types.RecognizedClient, source types.IssueSource, query string) (int, error) {
// 	openIssuesMutex.RLock()
// 	defer openIssuesMutex.RUnlock()

// 	totalCount := 0
// 	if client == "" {
// 		// Client has not been specified. Return the total count of all clients.
// 		for _, sourceToQueries := range openIssues {
// 			for _, queryToIssues := range sourceToQueries {
// 				for _, issues := range queryToIssues {
// 					totalCount += len(issues)
// 				}
// 			}
// 		}
// 	} else {
// 		if sourceToQueries, ok := openIssues[client]; ok {
// 			if source == "" {
// 				// Source has not been specified. Return the total count of this client.
// 				for _, queryToIssues := range sourceToQueries {
// 					for _, issues := range queryToIssues {
// 						totalCount += len(issues)
// 					}
// 				}
// 			} else {
// 				if queryToIssues, ok := sourceToQueries[source]; ok {
// 					if query == "" {
// 						// Query has not been specified. Return the total count of this client+source.
// 						for _, issues := range queryToIssues {
// 							totalCount += len(issues)
// 						}
// 					} else {
// 						if issues, ok := queryToIssues[query]; ok {
// 							// Retuen the total count of this client+source+query.
// 							totalCount = len(issues)
// 						} else {
// 							return -1, fmt.Errorf("Query %s is not recognized", query)
// 						}
// 					}
// 				} else {
// 					return -1, fmt.Errorf("Source %s is not recognized", source)
// 				}
// 			}
// 		} else {
// 			return -1, fmt.Errorf("Client %s is not recognized", client)
// 		}
// 	}

// 	return totalCount, nil
// }
