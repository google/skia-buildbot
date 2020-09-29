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

	// SetState sets a state to the specified issue.
	SetState(i Issue, state string) error
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

	// HER HERE
	// Actually return something here

	return matchingIssues, nil

	// // Read the entire file into memory and return a buffer.
	// contents, err = ioutil.ReadAll(reader); err != nil {
	// 	g.content = nil
	// 	return skerr.Fmt("error reading content of %s/%s: %s", g.bucket, g.name, err)
	// }

	/*
		query := ""
		if username != "" {
			query += fmt.Sprintf("assignee:%s", username)
		}
		// Do something with statuses as well!
		payLoad := struct {
			PageNum int    `json:"p"`
			Count   int    `json:"count"`
			Sort    string `json:"s"`
			Query   string `json:"q"`
		}{
			PageNum: 1,
			Count:   25,
			Sort:    "modified_time:desc",
			Query:   query,
		}

		b := new(bytes.Buffer)
		e := json.NewEncoder(b)
		if err := e.Encode(payLoad); err != nil {
			return nil, fmt.Errorf("Problem encoding json for request: %s", err)
		}

		httpClient := httputils.DefaultClientConfig().With2xxOnly().Client()
		// httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

		url := "https://issuetracker.google.com/action/issues/list"
		resp, err := httpClient.Post(url, "application/json", b)

		if err != nil || resp == nil || resp.StatusCode != 200 {
			return nil, fmt.Errorf("Failed to retrieve issue tracker response: %s", err)
		}
		defer util.Close(resp.Body)

		msg, err := ioutil.ReadAll(resp.Body)
		sklog.Infof("%s\n\nErr: %v", string(msg), err)

		return nil, nil
	*/
}

func (it *IssueTracker) GetLink(id string) string {
	return fmt.Sprintf("b/%s", id)
}

func (it *IssueTracker) AddComment(i Issue, comment string) error {
	return nil
}

func (it *IssueTracker) SetState(i Issue, status string) error {
	return nil
}

// func get(client *http.Client, u string) ([]Issue, error) {
// 	resp, err := client.Get(u)
// 	if err != nil || resp == nil || resp.StatusCode != 200 {
// 		return nil, fmt.Errorf("Failed to retrieve issue tracker response: %s Status Code: %d", err, resp.StatusCode)
// 	}
// 	defer util.Close(resp.Body)

// 	issueResponse := &IssueResponse{
// 		Items: []Issue{},
// 	}
// 	if err := json.NewDecoder(resp.Body).Decode(&issueResponse); err != nil {
// 		return nil, err
// 	}

// 	return issueResponse.Items, nil
// }

// func post(client *http.Client, dst string, request interface{}) error {
// 	b := new(bytes.Buffer)
// 	e := json.NewEncoder(b)
// 	if err := e.Encode(request); err != nil {
// 		return fmt.Errorf("Problem encoding json for request: %s", err)
// 	}

// 	resp, err := client.Post(dst, "application/json", b)

// 	if err != nil || resp == nil || resp.StatusCode != 200 {
// 		return fmt.Errorf("Failed to retrieve issue tracker response: %s", err)
// 	}
// 	defer util.Close(resp.Body)
// 	msg, err := ioutil.ReadAll(resp.Body)
// 	sklog.Infof("%s\n\nErr: %v", string(msg), err)
// 	return nil
// }

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
