package issuetracker

// Accesses issuetracker results from Google storage.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	issueTrackerBucket = "skia-issuetracker-details"
	// The file that contains issuetracker search results in the above bucket.
	resultsFileName = "results.json"

	issueTrackerSource types.IssueSource = "Buganizer"
)

type issueTrackerIssue struct {
	Id         int64  `json:"id"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	Assignee   string `json:"assignee"`
	CreatedTS  int64  `json:"created_ts"`
	ModifiedTS int64  `json:"modified_ts"`
}

// issueTracker implements bugs.BugsFramework for github repos.
type issueTracker struct {
	storageClient *storage.Client
	openIssues    *bugs.OpenIssues
	queryConfig   *IssueTrackerQueryConfig
}

// New returns an instance of the issuetracker implementation of bugs.BugFramework.
func New(storageClient *storage.Client, openIssues *bugs.OpenIssues, queryConfig *IssueTrackerQueryConfig) (bugs.BugFramework, error) {
	return &issueTracker{
		storageClient: storageClient,
		openIssues:    openIssues,
		queryConfig:   queryConfig,
	}, nil
}

// IssueTrackerQueryConfig is the config that will be used when querying issuetracker.
type IssueTrackerQueryConfig struct {
	// Key to find the open bugs from the storage results file.
	Query string
	// Which client's issues we are looking for.
	Client types.RecognizedClient
	// Issues are considered untriaged if they have any of these priorities.
	UntriagedPriorities []string
	// Issues are also considered untriaged if they are assigned to any of these emails.
	UntriagedAliases []string
}

// See documentation for bugs.Search interface.
func (it *issueTracker) Search(ctx context.Context) ([]*types.Issue, *types.IssueCountsData, error) {
	obj := it.storageClient.Bucket(issueTrackerBucket).Object(resultsFileName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "accessing gs://%s/%s failed", issueTrackerBucket, resultsFileName)
	}
	defer util.Close(reader)

	var results map[string][]issueTrackerIssue
	if err := json.NewDecoder(reader).Decode(&results); err != nil {
		return nil, nil, skerr.Wrapf(err, "invalid JSON from %s", resultsFileName)
	}

	if _, ok := results[it.queryConfig.Query]; !ok {
		return nil, nil, skerr.Fmt("could not find %s in %s", it.queryConfig.Query, resultsFileName)
	}
	// These are all the issues returned by the issuetracker query.
	trackerIssues := results[it.queryConfig.Query]

	// Convert issuetracker issues into bug_framework's generic issues
	issues := []*types.Issue{}
	countsData := &types.IssueCountsData{}
	for _, i := range trackerIssues {
		// Populate counts data.
		countsData.OpenCount++
		if i.Assignee == "" {
			countsData.UnassignedCount++
		}
		created := time.Unix(i.CreatedTS, 0)
		modified := time.Unix(i.ModifiedTS, 0)
		priority := types.StandardizedPriority(i.Priority)
		countsData.IncPriority(priority)
		sloViolation, reason, d := types.IsPrioritySLOViolation(time.Now(), created, modified, priority)
		countsData.IncSLOViolation(sloViolation, priority)
		if util.In(i.Priority, it.queryConfig.UntriagedPriorities) {
			countsData.UntriagedCount++
		} else if util.In(i.Assignee, it.queryConfig.UntriagedAliases) {
			countsData.UntriagedCount++
		}

		id := strconv.FormatInt(i.Id, 10)
		issues = append(issues, &types.Issue{
			Id:           id,
			State:        i.Status,
			Priority:     priority,
			Owner:        i.Assignee,
			CreatedTime:  created,
			ModifiedTime: modified,

			SLOViolation:         sloViolation,
			SLOViolationReason:   reason,
			SLOViolationDuration: d,

			Link: it.GetIssueLink("", id),
		})
	}
	return issues, countsData, nil
}

// See documentation for bugs.SearchClientAndPersist interface.
func (it *issueTracker) SearchClientAndPersist(ctx context.Context, dbClient *db.FirestoreDB, runId string) error {
	qc := it.queryConfig
	issues, countsData, err := it.Search(ctx)
	if err != nil {
		return skerr.Wrapf(err, "error when searching issuetracker")
	}
	sklog.Infof("%s issuetracker issues %+v", qc.Client, countsData)

	queryDesc := qc.Query
	countsData.QueryLink = fmt.Sprintf("http://b/issues?q=%s", qc.Query)
	// Construct query for untriaged issues.
	if len(qc.UntriagedPriorities) > 0 || len(qc.UntriagedAliases) > 0 {
		untriagedPrioritiesTokens := []string{}
		for _, p := range qc.UntriagedPriorities {
			untriagedPrioritiesTokens = append(untriagedPrioritiesTokens, fmt.Sprintf("p:%s", p))
		}
		untriagedAliasesTokens := []string{}
		for _, a := range qc.UntriagedAliases {
			untriagedAliasesTokens = append(untriagedAliasesTokens, fmt.Sprintf("assignee:%s", a))
		}
		countsData.UntriagedQueryLink = fmt.Sprintf("%s (%s|%s)", countsData.QueryLink, strings.Join(untriagedPrioritiesTokens, "|"), strings.Join(untriagedAliasesTokens, "|"))
	}
	client := qc.Client

	// Put in DB.
	if err := dbClient.PutInDB(ctx, client, issueTrackerSource, queryDesc, runId, countsData); err != nil {
		return skerr.Wrapf(err, "error putting issuetracker results in DB")
	}
	// Put in memory.
	it.openIssues.PutOpenIssues(client, issueTrackerSource, queryDesc, issues)
	return nil
}

// See documentation for bugs.GetIssueLink interface.
func (it *issueTracker) GetIssueLink(_, id string) string {
	return fmt.Sprintf("http://b/%s", id)
}
