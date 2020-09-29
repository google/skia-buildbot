package bug_framework

// CALL THIS bug_framework instead.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const ()

type Issue struct {
	Id       string `json:"id"`
	State    string `json:"state"`
	Priority string `json:"priority"`
	Owner    string `json:"owner"`
	Link     string `json:"link"`

	CreatedTime  time.Time `json:"created"`
	ModifiedTime time.Time `json:"modified"`
	ClosedTime   time.Time `json:"closed,omitempty"`

	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type BugFramework interface {

	// GetBugFrameworkName returns the name of the bug framework. Eg: Monorail, IssueTracker, Github.
	GetBugFrameworkName() string

	// Search returns issues that match the provided parameters.
	Search(ctx context.Context, open, unassigned bool) ([]*Issue, error)

	// Returns the bug framework specific link to the issue.
	GetLink(id string) string
}

////////////////////////////////////////////////////////////// IssueTracker //////////////////////////////////////////////////////////////

const (
	IssueTrackerBucket = "skia-issuetracker-details"
	// The file that contains issuetracker search results in the above bucket.
	ResultsFileName = "results.json"
	// Key to find the total open bugs from the above results file.
	TotalOpenKey = "c1346_total_open"
)

type IssueTrackerIssue struct {
	Id       int64  `json:"id"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Assignee string `json:"assignee"`
}

type IssueTracker struct {
	storageClient *storage.Client

	// ComponentIds []int64  `json:"component_ids"`
	// UserNames    []string `json:"usernames"`
}

func InitIssueTracker(storageClient *storage.Client) (BugFramework, error) {
	return &IssueTracker{
		storageClient: storageClient,
	}, nil
}

func (it *IssueTracker) GetBugFrameworkName() string {
	return "IssueTracker"
}

func (it *IssueTracker) Search(ctx context.Context, open, unassigned bool) ([]*Issue, error) {

	obj := it.storageClient.Bucket(IssueTrackerBucket).Object(ResultsFileName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "accessing gs://%s/%s failed: %s", IssueTrackerBucket, ResultsFileName)
	}
	defer util.Close(reader)

	var results map[string][]IssueTrackerIssue
	if err := json.NewDecoder(reader).Decode(&results); err != nil {
		return nil, skerr.Wrapf(err, "invalid JSON from %s", ResultsFileName)
	}
	fmt.Println(results)

	if _, ok := results[TotalOpenKey]; !ok {
		return []*Issue{}, nil
		//do something here
	}
	// These are all the issues returned by the issuetracker query.
	trackerIssues := results[TotalOpenKey]

	// Now filter them against the open/unassigned bools
	matchingIssues := []*Issue{}
	for _, i := range trackerIssues {
		// All issues returned by the issuetracker query are open.
		if !open {
			continue
		}
		// Check for unassigned.
		if unassigned && i.Assignee != "" {
			continue
		}

		id := strconv.FormatInt(i.Id, 10)
		matchingIssues = append(matchingIssues, &Issue{
			Id:       id,
			State:    i.Status,
			Priority: i.Priority,
			Owner:    i.Assignee,
			Link:     it.GetLink(id),
		})
	}
	return matchingIssues, nil
}

func (it *IssueTracker) GetLink(id string) string {
	return fmt.Sprintf("b/%s", id)
}

////////////////////////////////////////////////////////////// MONORAIL //////////////////////////////////////////////////////////////

type Monorail struct {
	Projects  []string `json:"projects"`
	UserNames []string `json:"usernames"`
}

////////////////////////////////////////////////////////////// GITHUB //////////////////////////////////////////////////////////////

type Github struct {
	Projects  []string `json:"projects"`
	UserNames []string `json:"usernames"`
}

func InitGithub(storageClient *storage.Client) (BugFramework, error) {
	return &IssueTracker{
		storageClient: storageClient,
	}, nil
}

func (gh *Github) GetBugFrameworkName() string {
	return "IssueTracker"
}

func (gh *Github) Search(ctx context.Context, open, unassigned bool) ([]*Issue, error) {
	return nil, nil
}

func (gh *Github) GetLink(id string) string {
	return ""
}
