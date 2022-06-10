package monorail // import "go.skia.org/infra/go/monorail/v3"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/idtoken"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	monorailApiBase             = "https://api-dot-monorail-prod.appspot.com/prpc/"
	monorailTokenTargetAudience = "https://monorail-prod.appspot.com"

	// Supported monorail instances.
	chromiumMonorailInstance = "chromium"
	skiaMonorailInstance     = "skia"

	// Project specific field constants.
	ChromiumPriorityFieldName = "projects/chromium/fieldDefs/11"
	ChromiumTypeFieldName     = "projects/chromium/fieldDefs/10"
	SkiaPriorityFieldName     = "projects/skia/fieldDefs/9"
	SkiaTypeFieldName         = "projects/skia/fieldDefs/8"

	RestrictViewGoogleLabelName = "Restrict-View-Google"
)

var (
	ProjectToPriorityFieldNames = map[string]string{
		chromiumMonorailInstance: ChromiumPriorityFieldName,
		skiaMonorailInstance:     SkiaPriorityFieldName,
	}

	ProjectToTypeFieldNames = map[string]string{
		chromiumMonorailInstance: ChromiumTypeFieldName,
		skiaMonorailInstance:     SkiaTypeFieldName,
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
	ClosedTime   time.Time `json:"closeTime"`

	Title string `json:"summary"`
}

type MonorailUser struct {
	DisplayName string `json:"displayName"`
}

// IMonorailService is the interface implemented by all monorail service impls.
type IMonorailService interface {
	// GetEmail returns the registered email of the provided user name.
	GetEmail(userName string) (*MonorailUser, error)

	// SetOwnerAndAddComment sets the provided owner for the monorail issue with
	// the provided comment.
	SetOwnerAndAddComment(instance, owner, comment, id string) error

	// GetIssueLink returns a link that points to the provided monorail issue.
	GetIssueLink(instance, id string) string

	// GetIssue returns a MonorailIssue object for the provided issue name.
	// Issue names look like this: "projects/skia/issues/13158".
	GetIssue(issueName string) (*MonorailIssue, error)

	// MakeIssue creates a new monorail issue.
	MakeIssue(instance, owner, summary, description, status, priority, issueType string, labels, componentDefIDs []string) (*MonorailIssue, error)

	// SearchIssuesWithPagination returns monorail issue results by autoamtically
	// paginating till end of results.
	// Monorail results are limited to 100 (see https://source.chromium.org/chromium/infra/infra/+/master:appengine/monorail/api/v3/api_proto/issues.proto;l=179).
	// It paginates till all results are received.
	SearchIssuesWithPagination(instance, query string) ([]MonorailIssue, error)
}

type MonorailService struct {
	HttpClient *http.Client
}

func New(ctx context.Context, serviceAccountFilePath string) (*MonorailService, error) {
	// Perform auth as described in https://docs.google.com/document/d/1Gx78HMBexadFm-jTOCcbFAXGCtucrN-0ET1mUd_hrHQ/edit#heading=h.a9iny4rfah43
	clientOption := idtoken.WithCredentialsFile(serviceAccountFilePath)
	ts, err := idtoken.NewTokenSource(ctx, monorailTokenTargetAudience, clientOption)
	if err != nil {
		return nil, skerr.Wrapf(err, "error running idtoken.NewTokenSource")
	}

	return &MonorailService{
		HttpClient: httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client(),
	}, nil
}

// makeJSONCall calls monorail's v3 pRPC based API (go/monorail-v3-api).
func (m *MonorailService) makeJSONCall(bodyJSON []byte, service string, method string) ([]byte, error) {
	path := monorailApiBase + fmt.Sprintf("monorail.v3.%s/%s", service, method)

	req, err := http.NewRequest("POST", path, bytes.NewBuffer(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	resp, err := m.HttpClient.Do(req)
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

	return b, nil
}

// GetEmail implements the IMonorailService interface.
func (m *MonorailService) GetEmail(userName string) (*MonorailUser, error) {
	b, err := m.makeJSONCall([]byte(fmt.Sprintf(`{"name": "%s"}`, userName)), "Users", "GetUser")
	if err != nil {
		return nil, skerr.Wrapf(err, "Users.GetUser JSON API call failed")
	}
	var user *MonorailUser
	if err := json.Unmarshal(b, &user); err != nil {
		return nil, err
	}
	return user, nil
}

// SetOwnerAndAddComment implements the IMonorailService interface.
func (m *MonorailService) SetOwnerAndAddComment(instance, owner, comment, id string) error {
	query := fmt.Sprintf(`{"deltas": [{"issue": {"name": "projects/%s/issues/%s", "owner": {"user": "users/%s"}}, "update_mask": "owner"}], "comment_content": "%s", "notify_type": "EMAIL"}`, instance, id, owner, comment)
	if _, err := m.makeJSONCall([]byte(query), "Issues", "ModifyIssues"); err != nil {
		return skerr.Wrapf(err, "Issues.ModifyIssues JSON API call failed")
	}
	return nil
}

// GetIssueLink implements the IMonorailService interface.
func (m *MonorailService) GetIssueLink(instance, id string) string {
	return fmt.Sprintf("https://bugs.chromium.org/p/%s/issues/detail?id=%s", instance, id)
}

// GetIssue implements the IMonorailService interface.
func (m *MonorailService) GetIssue(issueName string) (*MonorailIssue, error) {
	rpc := fmt.Sprintf(`{"name": "%s"}`, issueName)
	b, err := m.makeJSONCall([]byte(rpc), "Issues", "GetIssue")
	if err != nil {
		return nil, skerr.Wrapf(err, "Issues.GetIssue JSON API call failed")
	}
	var issue *MonorailIssue
	if err := json.Unmarshal(b, &issue); err != nil {
		return nil, skerr.Wrap(err)
	}
	return issue, nil
}

// MakeIssue implements the IMonorailService interface.
func (m *MonorailService) MakeIssue(instance, owner, summary, description, status, priority, issueType string, labels, componentDefIDs []string) (*MonorailIssue, error) {
	priorityFieldName, ok := ProjectToPriorityFieldNames[instance]
	if !ok {
		return nil, fmt.Errorf("We do not have priority field name information for project %s", instance)
	}
	typeFieldName, ok := ProjectToTypeFieldNames[instance]
	if !ok {
		return nil, fmt.Errorf("We do not have type field name information for project %s", instance)
	}
	labelsJSON := []string{}
	for _, l := range labels {
		labelsJSON = append(labelsJSON, fmt.Sprintf(`{"label": "%s"}`, l))
	}
	componentsJSON := []string{}
	for _, c := range componentDefIDs {
		componentsJSON = append(componentsJSON, fmt.Sprintf(`{"component": "projects/%s/componentDefs/%s"}`, instance, c))
	}

	rpc := fmt.Sprintf(`{"parent": "projects/%s", "issue": {"owner": {"user": "users/%s"}, "status": {"status": "%s"}, "summary": "%s", "labels": [%s], "components": [%s], "field_values": [{"field": "%s", "value": "%s"}, {"field": "%s", "value": "%s"}]}, "description": "%s"}`, instance, owner, status, summary, strings.Join(labelsJSON, ","), strings.Join(componentsJSON, ","), priorityFieldName, priority, typeFieldName, issueType, description)
	b, err := m.makeJSONCall([]byte(rpc), "Issues", "MakeIssue")
	if err != nil {
		return nil, skerr.Wrapf(err, "Issues.MakeIssue JSON API call failed")
	}
	var issue *MonorailIssue
	if err := json.Unmarshal(b, &issue); err != nil {
		return nil, skerr.Wrap(err)
	}
	return issue, nil
}

// SearchIssuesWithPagination implements the IMonorailService interface.
func (m *MonorailService) SearchIssuesWithPagination(instance, query string) ([]MonorailIssue, error) {
	issues := []MonorailIssue{}

	// Put in a loop till there are no new pages.
	nextPageToken := ""
	for {
		query := fmt.Sprintf(`{"projects": ["projects/%s"], "query": "%s", "page_token": "%s"}`, instance, query, nextPageToken)
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
			break
		}
	}

	return issues, nil
}
