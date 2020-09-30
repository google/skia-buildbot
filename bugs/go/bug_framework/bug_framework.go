package bug_framework

// CALL THIS bug_framework instead.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"

	"go.skia.org/infra/go/github"
	"go.skia.org/infra/go/httputils"
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
	SkiaTotalOpenKey = "c1346_total_open"
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

// TODO(rmistry): Pass in SkiaTotalOpenKey here to make it more flexible! No, it should be a list of keys instead..
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

	if _, ok := results[SkiaTotalOpenKey]; !ok {
		return []*Issue{}, nil
		//do something here
	}
	// These are all the issues returned by the issuetracker query.
	trackerIssues := results[SkiaTotalOpenKey]

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
	token      *oauth2.Token
	project    string
	components []string
	httpClient *http.Client
}

func InitMonorail(ctx context.Context, token *oauth2.Token, httpClient *http.Client, project string, components []string) (BugFramework, error) {

	// Tryign to skip target audience to see what happens
	clientOption := idtoken.WithCredentialsFile("/var/secrets/google/key.json")
	tokenSource, err := idtoken.NewTokenSource(ctx, "https://monorail-prod.appspot.com", clientOption)
	if err != nil {
		return nil, fmt.Errorf("idtoken.NewTokenSource: %v", err)
	}

	token, err = tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("TokenSource.Token: %v", err)
	}
	httpClient = &http.Client{}

	return &Monorail{
		token:      token,
		project:    project,
		components: components,
		httpClient: httpClient,
	}, nil
}

func (m *Monorail) GetBugFrameworkName() string {
	return "Monorail"
}

// Add documentationa bout what this is doing...
// issues.proto: https://source.chromium.org/chromium/infra/infra/+/master:appengine/monorail/api/v3/api_proto/issues.proto

func (m *Monorail) Search(ctx context.Context, open, unassigned bool) ([]*Issue, error) {
	path := "monorail.v3.Issues/SearchIssues"
	// Where is the documentation for this..

	base := "https://api-dot-monorail-prod.appspot.com/prpc/"
	bodyJSON := []byte(`{"projects": ["skia"], "query": "owner:rmistry@googel.com"}`)
	// bytes.NewBuffer(body)
	req, err := http.NewRequest("POST", base+path, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %v", err)
	}
	// token.AccessToken is an ID token, despite its name.
	req.Header.Add("authorization", "Bearer "+m.token.AccessToken)
	req.Header.Add("Content-Type", "application/json")
	response, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client.Do: %v", err)
	}
	fmt.Printf("\n\n\nResponse: %v", response)

	return nil, nil
}

func (m *Monorail) GetLink(id string) string {
	return fmt.Sprintf("https://bugs.chromium.org/p/%s/issues/detail?id=%s", m.project, id)
}

////////////////////////////////////////////////////////////// GITHUB //////////////////////////////////////////////////////////////

const (
	MAX_GITHUB_RESULTS = 100
)

// golden/go/code_review/github_crs/
type Github struct {
	client *github.GitHub
	labels []string
}

// If we need authenticated access to a github repo one day then could use the token
// for skia-flutter-autoroll. See bin/create-github-token-secret.sh
// https://developer.github.com/v3/issues/#list-repository-issues
func InitGithub(ctx context.Context, repoOwner, repoName, credPath string, labels []string) (BugFramework, error) {
	// HERE HERE
	// pass in owner + repo + label

	gBody, err := ioutil.ReadFile(credPath)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not find githubToken in %s", credPath)
	}
	gToken := strings.TrimSpace(string(gBody))
	githubTS := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: gToken})
	githubHttpClient := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(githubTS).Client()
	githubClient, err := github.NewGitHub(ctx, repoOwner, repoName, githubHttpClient)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not instantiate github client")
	}

	return &Github{
		client: githubClient,
		labels: labels,
	}, nil
}

func (gh *Github) GetBugFrameworkName() string {
	return "IssueTracker"
}

func (gh *Github) Search(ctx context.Context, open, unassigned bool) ([]*Issue, error) {
	githubIssues, err := gh.client.GetIssues(open, gh.labels, MAX_GITHUB_RESULTS)
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get github issues with %s labels", gh.labels)
	}

	// Convert github issues into bug_framework's generic issues
	issues := []*Issue{}
	for _, gi := range githubIssues {
		owner := ""
		if gi.GetAssignee() != nil {
			owner = gi.GetAssignee().GetEmail()
		}
		if unassigned && owner != "" {
			continue
		}
		id := strconv.Itoa(gi.GetNumber())
		issues = append(issues, &Issue{
			Id:       id,
			State:    gi.GetState(),
			Priority: "", // Nothing builtin. Flutter seems to use P0/P1/P2 labels? https://screenshot.googleplex.com/4wDC9wzmnwaPdZp
			Owner:    owner,
			Link:     gh.GetLink(id),

			CreatedTime:  gi.GetCreatedAt(),
			ModifiedTime: gi.GetUpdatedAt(),

			Title:   gi.GetTitle(),
			Summary: gi.GetBody(),
		})
	}

	return issues, nil
}

func (gh *Github) GetLink(id string) string {
	return gh.client.GetIssueUrlBase() + id
}
