package bugs

// Accesses issuetracker results from Google storage.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/storage"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	IssueTrackerBucket = "skia-issuetracker-details"
	// The file that contains issuetracker search results in the above bucket.
	ResultsFileName = "results.json"
)

type IssueTrackerIssue struct {
	Id         int64  `json:"id"`
	Status     string `json:"status"`
	Priority   string `json:"priority"`
	Assignee   string `json:"assignee"`
	CreatedTS  int64  `json:"created_ts"`
	ModifiedTS int64  `json:"modified_ts"`
}

type IssueTracker struct {
	storageClient *storage.Client
}

func InitIssueTracker(storageClient *storage.Client) (BugFramework, error) {
	return &IssueTracker{
		storageClient: storageClient,
	}, nil
}

type IssueTrackerQueryConfig struct {
	// Key to find the open bugs from the storage results file.
	Query string
	// Which client's issues we are looking for.
	Client types.RecognizedClient
}

// See documentation for bugs.Search interface.
func (it *IssueTracker) Search(ctx context.Context, config interface{}) ([]*types.Issue, *types.IssueCountsData, error) {
	itQueryConfig, ok := config.(IssueTrackerQueryConfig)
	if !ok {
		return nil, nil, errors.New("config must be IssueTrackerQueryConfig")
	}

	obj := it.storageClient.Bucket(IssueTrackerBucket).Object(ResultsFileName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "accessing gs://%s/%s failed", IssueTrackerBucket, ResultsFileName)
	}
	defer util.Close(reader)

	var results map[string][]IssueTrackerIssue
	if err := json.NewDecoder(reader).Decode(&results); err != nil {
		return nil, nil, skerr.Wrapf(err, "invalid JSON from %s", ResultsFileName)
	}

	if _, ok := results[itQueryConfig.Query]; !ok {
		return nil, nil, skerr.Fmt("could not find %s in %s", itQueryConfig.Query, ResultsFileName)
	}
	// These are all the issues returned by the issuetracker query.
	trackerIssues := results[itQueryConfig.Query]

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
		countsData.CalculateSLOViolations(created, modified, priority)

		id := strconv.FormatInt(i.Id, 10)
		issues = append(issues, &types.Issue{
			Id:           id,
			State:        i.Status,
			Priority:     priority,
			Owner:        i.Assignee,
			CreatedTime:  created,
			ModifiedTime: modified,

			Link: it.GetIssueLink("", id),
		})
	}
	return issues, countsData, nil
}

// See documentation for bugs.SearchClientAndPersist interface.
func (it *IssueTracker) SearchClientAndPersist(ctx context.Context, config interface{}, dbClient *db.FirestoreDB, runId string) error {
	qc, ok := config.(IssueTrackerQueryConfig)
	if !ok {
		return errors.New("config must be IssueTrackerQueryConfig")
	}

	issues, countsData, err := it.Search(ctx, qc)
	if err != nil {
		return skerr.Wrapf(err, "error when searching issuetracker")
	}
	sklog.Infof("%s issuetracker issues %+v", qc.Client, countsData)

	queryDesc := qc.Query
	countsData.QueryLink = fmt.Sprintf("http://b/issues?q=%s", qc.Query)
	client := qc.Client

	// Put in DB.
	if err := dbClient.PutInDB(ctx, client, IssueTrackerSource, queryDesc, runId, countsData); err != nil {
		return skerr.Wrapf(err, "error putting issuetracker results in DB")
	}
	// Put in memory.
	putOpenIssues(client, IssueTrackerSource, queryDesc, issues)
	return nil
}

// See documentation for bugs.GetIssueLink interface.
func (it *IssueTracker) GetIssueLink(_, id string) string {
	return fmt.Sprintf("http://b/%s", id)
}
