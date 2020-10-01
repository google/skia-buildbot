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

	"go.skia.org/infra/go/sklog"

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

	Title   string `json:"title"`   // This is not populated in IssueTracker.
	Summary string `json:"summary"` // This is not returned in IssueTracker or Monorail.
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

// From https://source.chromium.org/chromium/infra/infra/+/master:appengine/monorail/api/api_proto/issue_objects.proto;l=102
// Does not match.

const (
	MonorailApiBase             = "https://api-dot-monorail-prod.appspot.com/prpc/"
	MonorailTokenTargetAudience = "https://monorail-prod.appspot.com"
)

type PriorityData struct {
	FieldName       string
	PriorityMapping map[string]string
}

var (
	// Helps figure out which
	ProjectToPriorityData map[string]PriorityData = map[string]PriorityData{
		// https://bugs.chromium.org/p/skia/fields/detail?field=Priority
		"skia": PriorityData{
			FieldName: "projects/skia/fieldDefs/9",
			PriorityMapping: map[string]string{
				"Critical": "P0",
				"High":     "P1",
				"Medium":   "P2",
				"Low":      "P3",
				"Icebox":   "P4",
			},
		},
		"chromium": PriorityData{
			FieldName: "projects/skia/fieldDefs/11",
			PriorityMapping: map[string]string{
				"0": "P0",
				"1": "P1",
				"2": "P2",
				"3": "P3",
			},
		},
	}
)

type MonorailIssue struct {
	Name  string `json:"name"`
	State struct {
		Status string `json:"status"`
	} `json:"status"`
	FieldValues []struct {
		Field string `json:"field"`
		Value string `json:"value"`
	} `json:"fieldValues"`
	Owner struct {
		User string `json:"user"`
	} `json:"owner"`

	CreatedTime  time.Time `json:"createTime"`
	ModifiedTime time.Time `json:"modifyTime"`

	Title string `json:"summary"`
}

type Monorail struct {
	token      *oauth2.Token
	project    string
	components []string
	httpClient *http.Client
}

func InitMonorail(ctx context.Context, token *oauth2.Token, httpClient *http.Client, project string, components []string) (BugFramework, error) {

	// Tryign to skip target audience to see what happens
	clientOption := idtoken.WithCredentialsFile("/var/secrets/google/key.json") // Need to be a parameter.
	tokenSource, err := idtoken.NewTokenSource(ctx, MonorailTokenTargetAudience, clientOption)
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

// issues.proto: https://source.chromium.org/chromium/infra/infra/+/master:appengine/monorail/api/v3/api_proto/issues.proto

func (m *Monorail) makeJSONCall(bodyJSON []byte, service string, method string) ([]byte, error) {
	path := MonorailApiBase + fmt.Sprintf("monorail.v3.%s/%s", service, method)

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %v", err)
	}
	req.Header.Add("authorization", "Bearer "+m.token.AccessToken)
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client.Do: %v", err)
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		return nil, skerr.Wrapf(err, "resp status code: %d status text: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Fmt("Failed to read response: %s", err)
	}
	// Strip off the XSS protection chars.
	b = b[4:]

	// For Debugging
	fmt.Print(string(b))

	return b, nil
}

func (m *Monorail) Search(ctx context.Context, open, unassigned bool) ([]*Issue, error) {
	// query := `{"projects": ["projects/skia"], "query": "id:873"}`
	query := `{"projects": ["projects/skia"], "query": "owner:rmistry@google.com status:Started"}`
	b, err := m.makeJSONCall([]byte(query), "Issues", "SearchIssues")
	if err != nil {
		return nil, skerr.Wrapf(err, "JSON API call failed")
	}

	var monorailIssues struct {
		Issues []MonorailIssue `json:"issues"`
	}
	if err := json.Unmarshal(b, &monorailIssues); err != nil {
		return nil, err
	}
	// if err := json.NewDecoder(resp.Body).Decode(&monorailIssues); err != nil {
	// 	return nil, err
	// }
	fmt.Printf("\n\n%+v\n\n", monorailIssues)

	// Convert monorail issues into bug_framework's generic issues
	issues := []*Issue{}
	for _, mi := range monorailIssues.Issues {

		fmt.Println(mi.Name)
		fmt.Println(mi.State.Status)
		fmt.Println(mi.FieldValues[0].Field)
		// Find priority using ProjectToPriorityData
		priority := ""
		if priorityData, ok := ProjectToPriorityData[m.project]; ok {
			for _, fv := range mi.FieldValues {
				if priorityData.FieldName == fv.Field {
					// Found the priority field for this project.
					// Now translate the priority field value into the generic priority value (P0, P1, ...)
					if p, ok := priorityData.PriorityMapping[fv.Value]; ok {
						priority = p
						break
					} else {
						sklog.Errorf("Could not find priority value %s for project %s", fv.Value, m.project)
					}
				}
			}
		} else {
			sklog.Errorf("Could not find ProjectToPriorityData for project %s", m.project)
		}
		fmt.Println(priority)
		fmt.Println(mi.Owner.User) // Need to figure out who the owner is!
		// owner := ""
		// Move this up to avoid makign the priority calls first.
		owner := ""
		if mi.Owner.User != "" {
			if unassigned {
				continue
			}

			// Make a call to users.GetUser to find the owner's email address.
			b, err := m.makeJSONCall([]byte(fmt.Sprintf(`{"name": "%s"}`, mi.Owner.User)), "Users", "GetUser")
			if err != nil {
				return nil, skerr.Wrapf(err, "JSON API call failed")
			}

			var monorailUser struct {
				DisplayName string `json:"displayName"`
			}
			if err := json.Unmarshal(b, &monorailUser); err != nil {
				return nil, err
			}
			owner = monorailUser.DisplayName
		}
		fmt.Println(owner)
		fmt.Println(mi.CreatedTime)
		fmt.Println(mi.ModifiedTime)
		fmt.Println(mi.Title)

		// Monorail issue names look like "projects/skia/issues/10783". Extract out the "10783".
		nameTokens := strings.Split(mi.Name, "/")
		id := nameTokens[len(nameTokens)-1]
		fmt.Println(id)

		issues = append(issues, &Issue{
			Id:       id,
			State:    mi.State.Status,
			Priority: priority,
			Owner:    owner,
			Link:     m.GetLink(id),

			CreatedTime:  mi.CreatedTime,
			ModifiedTime: mi.ModifiedTime,

			Title: mi.Title,
		})
	}

	return issues, nil
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
