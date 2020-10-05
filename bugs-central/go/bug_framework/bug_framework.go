package bug_framework

// CALL THIS bug_framework instead.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

const (
	// All bug frameworks will be standardized to these priorities.
	PriorityP0 = "P0"
	PriorityP1 = "P1"
	PriorityP2 = "P2"
	PriorityP3 = "P3"
	PriorityP4 = "P4"
)

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

	// REMOVE???
	// GetBugFrameworkName returns the name of the bug framework. Eg: Monorail, IssueTracker, Github.
	GetBugFrameworkName() string

	// Search returns issues that match the provided parameters.
	Search(ctx context.Context, config interface{}) ([]*Issue, error)

	// Returns the bug framework specific link to the issue.
	GetLink(project, id string) string
}

////////////////////////////////////////////////////////////// IssueTracker //////////////////////////////////////////////////////////////

const (
	IssueTrackerBucket = "skia-issuetracker-details"
	// The file that contains issuetracker search results in the above bucket.
	ResultsFileName = "results.json"
)

type IssueTrackerIssue struct {
	Id       int64  `json:"id"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Assignee string `json:"assignee"`
}

type IssueTracker struct {
	storageClient *storage.Client
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

type IssueTrackerQueryConfig struct {
	// Key to find the total open bugs from the above results file.
	QueryName string
	// If true will return only unassigned issues.
	UnAssigned bool
}

func (it *IssueTracker) Search(ctx context.Context, config interface{}) ([]*Issue, error) {
	itQueryConfig, ok := config.(IssueTrackerQueryConfig)
	if !ok {
		return nil, errors.New("config must be IssueTrackerQueryConfig")
	}

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
			Id:       id,
			State:    i.Status,
			Priority: i.Priority,
			Owner:    i.Assignee,
			Link:     it.GetLink("", id),
		})
	}
	return matchingIssues, nil
}

// IssueTracker links do not need a project specified.
func (it *IssueTracker) GetLink(_, id string) string {
	return fmt.Sprintf("b/%s", id)
}

////////////////////////////////////////////////////////////// MONORAIL //////////////////////////////////////////////////////////////

const (
	MonorailApiBase             = "https://api-dot-monorail-prod.appspot.com/prpc/"
	MonorailTokenTargetAudience = "https://monorail-prod.appspot.com"
)

var (
	// Maps the various priority configurations of different projects into the standardized priorities.
	MonorailProjectToPriorityData map[string]MonorailPriorityData = map[string]MonorailPriorityData{
		// https://bugs.chromium.org/p/skia/fields/detail?field=Priority
		"skia": MonorailPriorityData{
			FieldName: "projects/skia/fieldDefs/9",
			PriorityMapping: map[string]string{
				"Critical": PriorityP0,
				"High":     PriorityP1,
				"Medium":   PriorityP2,
				"Low":      PriorityP3,
				"Icebox":   PriorityP4,
			},
		},
		// https://bugs.chromium.org/p/chromium/fields/detail?field=Pri
		"chromium": MonorailPriorityData{
			FieldName: "projects/skia/fieldDefs/11",
			PriorityMapping: map[string]string{
				"0": PriorityP0,
				"1": PriorityP1,
				"2": PriorityP2,
				"3": PriorityP3,
			},
		},
	}

	// Stores the results of User.GetUser calls so we do not wastefully have to keep making them.
	userToEmailCache map[string]string = map[string]string{}
)

type MonorailPriorityData struct {
	FieldName       string
	PriorityMapping map[string]string
}

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
	httpClient *http.Client
}

func InitMonorail(ctx context.Context, serviceAccountFilePath string) (BugFramework, error) {
	// Perform auth as described in https://docs.google.com/document/d/1Gx78HMBexadFm-jTOCcbFAXGCtucrN-0ET1mUd_hrHQ/edit#heading=h.a9iny4rfah43
	clientOption := idtoken.WithCredentialsFile(serviceAccountFilePath)
	ts, err := idtoken.NewTokenSource(ctx, MonorailTokenTargetAudience, clientOption)
	if err != nil {
		return nil, skerr.Wrapf(err, "error running idtoken.NewTokenSource")
	}
	token, err := ts.Token()
	if err != nil {
		return nil, skerr.Wrapf(err, "error running ts.Token")
	}

	return &Monorail{
		token:      token,
		httpClient: httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client(),
	}, nil
}

func (m *Monorail) GetBugFrameworkName() string {
	return "Monorail"
}

type MonorailQueryConfig struct {
	// Name of the monorail project to query.
	Project string
	// Monorail query to run.
	Query string
}

// makeJSONCall calls monorail's v3 pRPC based API (go/monorail-v3-api).
// TODO(rmistry): Switch this to use the Go client library whenever it is available (https://bugs.chromium.org/p/monorail/issues/detail?id=8257).
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
		return nil, skerr.Wrapf(err, "resp status_code: %d status_text: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Fmt("Failed to read response: %s", err)
	}
	// Strip off the XSS protection chars.
	b = b[4:]

	// TODO(rmistry): For Debugging
	// fmt.Print(string(b))
	// sklog.Fatal("exit")

	return b, nil
}

// TODO(rmistry): There is currently a bug with 400 results topping off: https://bugs.chromium.org/p/monorail/issues/detail?id=8410
// searchIssues returns monorail issue results by autoamtically paginating till end of results.
// Monorail results are limited to 100 (see https://source.chromium.org/chromium/infra/infra/+/master:appengine/monorail/api/v3/api_proto/issues.proto;l=179). It paginates till all results are received.
func (m *Monorail) searchIssuesWithPagination(mc MonorailQueryConfig) ([]MonorailIssue, error) {
	issues := []MonorailIssue{}

	// Put in a loop till there are no new pages.
	nextPageToken := ""
	for {
		query := fmt.Sprintf(`{"projects": ["projects/%s"], "query": "%s", "page_token": "%s"}`, mc.Project, mc.Query, nextPageToken)
		b, err := m.makeJSONCall([]byte(query), "Issues", "SearchIssues")
		if err != nil {
			return nil, skerr.Wrapf(err, "Issues.SearchIssues JSON API call failed")
		}
		var monorailIssues struct {
			Issues        []MonorailIssue `json:"issues"`
			NextPageToken string          `json:"nextPageToken"`
		}
		if err := json.Unmarshal(b, &monorailIssues); err != nil {
			return nil, err
		}
		issues = append(issues, monorailIssues.Issues...)
		nextPageToken = monorailIssues.NextPageToken
		if nextPageToken == "" {
			// fmt.Println("DONE WITH PAGINATION")
			break
		}
		// fmt.Println(nextPageToken)
		// fmt.Println("STILL PAGINATING!")
	}

	return issues, nil
}

// Search implements the BugFramework interface.
// The open parameter is not used for Monorail searches. Specify this in your query with 'is:open'.
// Similarly, the unassigned parameter is not used. Specify this in your query with '-has:owner'.
func (m *Monorail) Search(ctx context.Context, config interface{}) ([]*Issue, error) {
	mQueryConfig, ok := config.(MonorailQueryConfig)
	if !ok {
		return nil, errors.New("config must be MonorailQueryConfig")
	}

	monorailIssues, err := m.searchIssuesWithPagination(mQueryConfig)
	if err != nil {
		return nil, skerr.Wrapf(err, "error when searching issues")
	}

	// Convert monorail issues into bug_framework's generic issues
	issues := []*Issue{}
	for _, mi := range monorailIssues {
		// Find the owner.
		owner := ""
		if mi.Owner.User != "" {
			// Check the cache before making an external API call.
			if email, ok := userToEmailCache[mi.Owner.User]; ok {
				owner = email
			} else {
				// Find the owner's email address.
				b, err := m.makeJSONCall([]byte(fmt.Sprintf(`{"name": "%s"}`, mi.Owner.User)), "Users", "GetUser")
				if err != nil {
					return nil, skerr.Wrapf(err, "Users.GetUser JSON API call failed")
				}
				var monorailUser struct {
					DisplayName string `json:"displayName"`
				}
				if err := json.Unmarshal(b, &monorailUser); err != nil {
					return nil, err
				}
				// Cache results for next time.
				userToEmailCache[mi.Owner.User] = monorailUser.DisplayName
				owner = monorailUser.DisplayName
			}
		}

		// Find priority using MonorailProjectToPriorityData
		priority := ""
		if priorityData, ok := MonorailProjectToPriorityData[mQueryConfig.Project]; ok {
			for _, fv := range mi.FieldValues {
				if priorityData.FieldName == fv.Field {
					// Found the priority field for this project. Now translate
					// the priority field value into the generic priority value (P0, P1, ...)
					if p, ok := priorityData.PriorityMapping[fv.Value]; ok {
						priority = p
						break
					} else {
						sklog.Errorf("Could not find priority value %s for project %s", fv.Value, mQueryConfig.Project)
					}
				}
			}
		} else {
			sklog.Errorf("Could not find ProjectToPriorityData for project %s", mQueryConfig.Project)
		}

		// Monorail issue names look like "projects/skia/issues/10783". Extract out the "10783".
		nameTokens := strings.Split(mi.Name, "/")
		id := nameTokens[len(nameTokens)-1]

		issues = append(issues, &Issue{
			Id:       id,
			State:    mi.State.Status,
			Priority: priority,
			Owner:    owner,
			Link:     m.GetLink(mQueryConfig.Project, id),

			CreatedTime:  mi.CreatedTime,
			ModifiedTime: mi.ModifiedTime,

			Title: mi.Title,
		})
	}

	return issues, nil
}

func (m *Monorail) GetLink(project, id string) string {
	return fmt.Sprintf("https://bugs.chromium.org/p/%s/issues/detail?id=%s", project, id)
}

////////////////////////////////////////////////////////////// GITHUB //////////////////////////////////////////////////////////////

const (
	// Not clear what the maximum allowable results are for github API.
	MAX_GITHUB_RESULTS = 1000
)

// golden/go/code_review/github_crs/
type Github struct {
	client *github.GitHub
	labels []string
}

type GithubQueryConfig struct {
	// Slice of labels to look for in Github issues.
	Labels []string
	// Return only open issues.
	Open bool
	// Return only unassigned issues.
	UnAssigned bool
}

// If we need authenticated access to a github repo one day then could use the token
// for skia-flutter-autoroll. See bin/create-github-token-secret.sh
// https://developer.github.com/v3/issues/#list-repository-issues
func InitGithub(ctx context.Context, repoOwner, repoName, credPath string) (BugFramework, error) {
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
	}, nil
}

func (gh *Github) GetBugFrameworkName() string {
	return "IssueTracker"
}

func (gh *Github) Search(ctx context.Context, config interface{}) ([]*Issue, error) {
	githubQueryConfig, ok := config.(GithubQueryConfig)
	if !ok {
		return nil, errors.New("config must be GithubQueryConfig")
	}

	githubIssues, err := gh.client.GetIssues(githubQueryConfig.Open, githubQueryConfig.Labels, MAX_GITHUB_RESULTS)
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
		if githubQueryConfig.UnAssigned && owner != "" {
			continue
		}
		id := strconv.Itoa(gi.GetNumber())
		fmt.Println("LABELS:")
		fmt.Printf("%+v", gi.Labels)
		issues = append(issues, &Issue{
			Id:       id,
			State:    gi.GetState(),
			Priority: "", // Nothing builtin. Flutter seems to use P0/P1/P2 labels? https://screenshot.googleplex.com/4wDC9wzmnwaPdZp
			Owner:    owner,
			Link:     gh.GetLink("", id),

			CreatedTime:  gi.GetCreatedAt(),
			ModifiedTime: gi.GetUpdatedAt(),

			Title:   gi.GetTitle(),
			Summary: gi.GetBody(),
		})
	}

	return issues, nil
}

func (gh *Github) GetLink(_, id string) string {
	return gh.client.GetIssueUrlBase() + id
}
