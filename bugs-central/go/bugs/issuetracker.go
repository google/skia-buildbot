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
	// Key to find the total open bugs from the above results file.
	QueryName string
	// If true will return only unassigned issues.
	UnAssigned bool
	// The client the found issues should be attributed to.
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

	if _, ok := results[itQueryConfig.QueryName]; !ok {
		return nil, skerr.Fmt("could not find %s in %s", itQueryConfig.QueryName, ResultsFileName)
	}
	// These are all the issues returned by the issuetracker query.
	trackerIssues := results[itQueryConfig.QueryName]

	// Now filter them against the open/unassigned bools
	matchingIssues := []*Issue{}
	for _, i := range trackerIssues {
		// Check for unassigned.
		if itQueryConfig.UnAssigned && i.Assignee != "" {
			continue
		}

		id := strconv.FormatInt(i.Id, 10)
		matchingIssues = append(matchingIssues, &Issue{
			Id:           id,
			State:        i.Status,
			Priority:     StandardizedPriority(i.Priority),
			Owner:        i.Assignee,
			CreatedTime:  time.Unix(i.CreatedTS, 0),
			ModifiedTime: time.Unix(i.ModifiedTS, 0),

			Link:   it.GetLink("", id),
			Source: "Buganizer",
			Client: itQueryConfig.Client,
		})
	}
	return matchingIssues, nil
}

// IssueTracker links do not need a project specified.
func (it *IssueTracker) GetLink(_, id string) string {
	return fmt.Sprintf("b/%s", id)
}
