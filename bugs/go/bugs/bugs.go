package bugs

// CALL THIS bug_framework instead.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const ()

type Issue struct {
	Id       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	State    string `json:"state"`
	Priority string `json:"priority"`
	Owner    string `json:"owner"`
	Link     string `json:"link"`

	CreatedTime  time.Time `json:"created"`
	ModifiedTime time.Time `json:"modified"`
	ClosedTime   time.Time `json:"closed,omitempty"`

	// Maybe:
	//   Any other extra information like labels or tags or something else
}

type BugFramework interface {

	// GetBugFrameworkName returns the name of the bug framework. Eg: Monorail, IssueTracker, Github.
	GetBugFrameworkName() string

	// Search returns issues that match the provided parameters.
	Search(username string, statuses []string) ([]Issue, error)

	// Modifying might not be possible.. because will the service account modify or the actual user?

	// AddComment adds a comment to the specified issue.
	AddComment(i Issue, comment string) error

	// SetState sets a state to the specified issue.
	SetState(i Issue, state string) error

	// SetTitle

	// SetSummary

	// Should have a way to modify the title and summary as well...

}

////////////////////////////////////////////////////////////// IssueTracker //////////////////////////////////////////////////////////////

type IssueTracker struct {
	ComponentIds []int64  `json:"component_ids"`
	UserNames    []string `json:"usernames"`
}

func InitIssueTracker() (BugFramework, error) {
	return &IssueTracker{}, nil
}

func (it *IssueTracker) GetBugFrameworkName() string {
	return "IssueTracker"
}

func (it *IssueTracker) Search(username string, statuses []string) ([]Issue, error) {
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
	// url := "https://b.corp.google.com/action/issues/list"
	resp, err := httpClient.Post(url, "application/json", b)

	if err != nil || resp == nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to retrieve issue tracker response: %s", err)
	}
	defer util.Close(resp.Body)

	msg, err := ioutil.ReadAll(resp.Body)
	sklog.Infof("%s\n\nErr: %v", string(msg), err)

	return nil, nil
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
