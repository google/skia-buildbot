package issues

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MONORAIL_BASE_URL_TMPL = "https://monorail-prod.appspot.com/_ah/api/monorail/v1/projects/%s/issues"

	PROJECT_ANGLE    = "angleproject"
	PROJECT_CHROMIUM = "chromium"
	PROJECT_PDFIUM   = "pdfium"
	PROJECT_SKIA     = "skia"
	PROJECT_WEBRTC   = "webrtc"
)

var (
	// "Constants"
	REPO_PROJECT_MAPPING = map[string]string{
		common.REPO_ANGLE:              PROJECT_ANGLE,
		common.REPO_CHROMIUM:           PROJECT_CHROMIUM,
		common.REPO_DEPOT_TOOLS:        PROJECT_CHROMIUM,
		common.REPO_PDFIUM:             PROJECT_PDFIUM,
		common.REPO_SKIA:               PROJECT_SKIA,
		common.REPO_SKIA_INFRA:         PROJECT_SKIA,
		common.REPO_SKIA_INTERNAL:      PROJECT_SKIA,
		common.REPO_SKIA_INTERNAL_TEST: PROJECT_SKIA,
		common.REPO_WEBRTC:             PROJECT_WEBRTC,
	}
)

// IssueTracker is a genric interface to an issue tracker that allows us
// to connect issues with items (identified by an id).
type IssueTracker interface {
	// FromQueury returns issue that match the given query string.
	FromQuery(q string) ([]Issue, error)
	// AddComment adds a comment to the issue with the given id
	AddComment(id string, comment CommentRequest) error
	// AddIssue creates an issue with the passed in params.
	AddIssue(issue IssueRequest) error
}

// Issue is an individual issue returned from the project hosting response.
type Issue struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
}

// IssueResponse is used to decode JSON responses from the project hosting API.
type IssueResponse struct {
	Items []Issue `json:"items"`
}

type CommentRequest struct {
	Content string `json:"content"`
}

type MonorailPerson struct {
	Name     string `json:"name"`     // Email address
	HtmlLink string `json:"htmlLink"` // Links to user id
	Kind     string `json:"kind"`     // Is always "monorail#issuePerson"
}

type IssueRequest struct {
	Status      string           `json:"status"`
	Owner       MonorailPerson   `json:"owner"`
	CC          []MonorailPerson `json:"cc"`
	Labels      []string         `json:"labels"`
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Components  []string         `json:"components,omitempty"`
}

// MonorailIssueTracker implements IssueTracker.
//
// Note that, in order to use a MonorailIssueTracker from GCE, the client id of
// the project needs to be known to Monorail.  Also note that the
// https://www.googleapis.com/auth/userinfo.email scope is needed.
type MonorailIssueTracker struct {
	client  *http.Client
	project string
	url     string
}

func NewMonorailIssueTracker(client *http.Client, project string) IssueTracker {
	return &MonorailIssueTracker{
		client:  client,
		project: project,
		url:     fmt.Sprintf(MONORAIL_BASE_URL_TMPL, project),
	}
}

// FromQuery is part of the IssueTracker interface. See documentation there.
func (m *MonorailIssueTracker) FromQuery(q string) ([]Issue, error) {
	query := url.Values{}
	query.Add("q", q)
	query.Add("fields", "items/id,items/state,items/title")
	return get(m.client, m.url+"?"+query.Encode())
}

// AddComment adds a comment to the issue with the given id
func (m *MonorailIssueTracker) AddComment(id string, comment CommentRequest) error {
	u := fmt.Sprintf("%s/%s/comments", m.url, id)
	return post(m.client, u, comment)
}

// AddIssue creates an issue with the passed in params.
func (m *MonorailIssueTracker) AddIssue(issue IssueRequest) error {
	req := struct {
		IssueRequest
		Project string `json:"projectId"`
	}{
		IssueRequest: issue,
		Project:      m.project,
	}
	return post(m.client, m.url, req)
}

func get(client *http.Client, u string) ([]Issue, error) {
	resp, err := client.Get(u)
	if err != nil || resp == nil || resp.StatusCode != 200 {
		return nil, fmt.Errorf("Failed to retrieve issue tracker response: %s Status Code: %d", err, resp.StatusCode)
	}
	defer util.Close(resp.Body)

	issueResponse := &IssueResponse{
		Items: []Issue{},
	}
	if err := json.NewDecoder(resp.Body).Decode(&issueResponse); err != nil {
		return nil, err
	}

	return issueResponse.Items, nil
}

func post(client *http.Client, dst string, request interface{}) error {
	b := new(bytes.Buffer)
	e := json.NewEncoder(b)
	if err := e.Encode(request); err != nil {
		return fmt.Errorf("Problem encoding json for request: %s", err)
	}

	resp, err := client.Post(dst, "application/json", b)

	if err != nil || resp == nil || resp.StatusCode != 200 {
		return fmt.Errorf("Failed to retrieve issue tracker response: %s", err)
	}
	defer util.Close(resp.Body)
	msg, err := ioutil.ReadAll(resp.Body)
	sklog.Infof("%s\n\nErr: %v", string(msg), err)
	return nil
}
