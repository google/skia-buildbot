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
	"go.skia.org/infra/go/util"
)

const (
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

func (it *IssueTracker) Search(ctx context.Context, config interface{}) ([]*Issue, error) {
	itQueryConfig, ok := config.(IssueTrackerQueryConfig)
	if !ok {
		return nil, errors.New("config must be IssueTrackerQueryConfig")
	}

	obj := it.storageClient.Bucket(IssueTrackerBucket).Object(ResultsFileName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "accessing gs://%s/%s failed", IssueTrackerBucket, ResultsFileName)
	}
	defer util.Close(reader)

	var results map[string][]IssueTrackerIssue
	if err := json.NewDecoder(reader).Decode(&results); err != nil {
		return nil, skerr.Wrapf(err, "invalid JSON from %s", ResultsFileName)
	}
	fmt.Println(results)

	if _, ok := results[itQueryConfig.Query]; !ok {
		return nil, skerr.Fmt("could not find %s in %s", itQueryConfig.Query, ResultsFileName)
	}
	// These are all the issues returned by the issuetracker query.
	trackerIssues := results[itQueryConfig.Query]

	// Convert issuetracker issues into bug_framework's generic issues
	issues := []*Issue{}
	for _, i := range trackerIssues {

		id := strconv.FormatInt(i.Id, 10)
		issues = append(issues, &Issue{
			Id:           id,
			State:        i.Status,
			Priority:     types.StandardizedPriority(i.Priority),
			Owner:        i.Assignee,
			CreatedTime:  time.Unix(i.CreatedTS, 0),
			ModifiedTime: time.Unix(i.ModifiedTS, 0),

			Link: it.GetLink("", id),
		})
	}
	return issues, nil
}

func (it *IssueTracker) PutInDB(ctx context.Context, config interface{}, count int, dbClient *db.FirestoreDB) error {
	itQueryConfig, ok := config.(IssueTrackerQueryConfig)
	if !ok {
		return errors.New("config must be IssueTrackerQueryConfig")
	}

	queryLink := fmt.Sprintf("http://b/issues?q=%s", itQueryConfig.Query)
	if err := dbClient.PutInDB(ctx, itQueryConfig.Client, IssueTrackerSource, itQueryConfig.Query, queryLink, count); err != nil {
		return skerr.Wrapf(err, "error putting monorail results in DB")
	}
	return nil
}

// IssueTracker links do not need a project specified.
func (it *IssueTracker) GetLink(_, id string) string {
	return fmt.Sprintf("b/%s", id)
}
