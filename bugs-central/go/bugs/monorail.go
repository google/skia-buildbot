package bugs

// Accesses monorail v3 pRPC based API (go/monorail-v3-api).
// TODO(rmistry): Switch this to use the Go client library whenever it is available (https://bugs.chromium.org/p/monorail/issues/detail?id=8257).

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/bugs-central/go/db"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
	"google.golang.org/api/idtoken"
)

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
			PriorityMapping: map[string]types.StandardizedPriority{
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
			PriorityMapping: map[string]types.StandardizedPriority{
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
	PriorityMapping map[string]types.StandardizedPriority
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

type MonorailQueryConfig struct {
	// Monorail instance to query.
	Instance string
	// Monorail query to run.
	Query string
}

// makeJSONCall calls monorail's v3 pRPC based API (go/monorail-v3-api).
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
		query := fmt.Sprintf(`{"projects": ["projects/%s"], "query": "%s", "page_token": "%s"}`, mc.Instance, mc.Query, nextPageToken)
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

// Search implements the BugFramework interface.
// The open parameter is not used for Monorail searches. Specify this in your query with 'is:open'.
// Similarly, the unassigned parameter is not used. Specify this in your query with '-has:owner'.
func (m *Monorail) Search(ctx context.Context, config interface{}) ([]*Issue, int, error) {
	mQueryConfig, ok := config.(MonorailQueryConfig)
	if !ok {
		return nil, -1, errors.New("config must be MonorailQueryConfig")
	}

	monorailIssues, err := m.searchIssuesWithPagination(mQueryConfig)
	if err != nil {
		return nil, -1, skerr.Wrapf(err, "error when searching issues")
	}

	// Convert monorail issues into bug_framework's generic issues
	issues := []*Issue{}
	unassignedIssuesCount := 0
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
					return nil, -1, skerr.Wrapf(err, "Users.GetUser JSON API call failed")
				}
				var monorailUser struct {
					DisplayName string `json:"displayName"`
				}
				if err := json.Unmarshal(b, &monorailUser); err != nil {
					return nil, -1, err
				}
				// Cache results for next time.
				userToEmailCache[mi.Owner.User] = monorailUser.DisplayName
				owner = monorailUser.DisplayName
			}
		}
		if owner == "" {
			unassignedIssuesCount++
		}

		// Find priority using MonorailProjectToPriorityData
		priority := types.StandardizedPriority("")
		if priorityData, ok := MonorailProjectToPriorityData[mQueryConfig.Instance]; ok {
			for _, fv := range mi.FieldValues {
				if priorityData.FieldName == fv.Field {
					// Found the priority field for this project. Now translate
					// the priority field value into the generic priority value (P0, P1, ...)
					if p, ok := priorityData.PriorityMapping[fv.Value]; ok {
						priority = p
						break
					} else {
						sklog.Errorf("Could not find priority value %s for project %s", fv.Value, mQueryConfig.Instance)
					}
				}
			}
		} else {
			sklog.Errorf("Could not find MonorailProjectToPriorityData for project %s", mQueryConfig.Instance)
		}

		// Monorail issue names look like "projects/skia/issues/10783". Extract out the "10783".
		nameTokens := strings.Split(mi.Name, "/")
		id := nameTokens[len(nameTokens)-1]

		issues = append(issues, &Issue{
			Id:       id,
			State:    mi.State.Status,
			Priority: priority,
			Owner:    owner,
			Link:     m.GetLink(mQueryConfig.Instance, id),

			CreatedTime:  mi.CreatedTime,
			ModifiedTime: mi.ModifiedTime,

			Title: mi.Title,
		})
	}

	return issues, unassignedIssuesCount, nil
}

func (m *Monorail) PutInDB(ctx context.Context, config interface{}, openCount, unassignedCount int, dbClient *db.FirestoreDB) error {
	mQueryConfig, ok := config.(MonorailQueryConfig)
	if !ok {
		return errors.New("config must be MonorailQueryConfig")
	}

	queryLink := fmt.Sprintf("https://bugs.chromium.org/p/%s/issues/list?can=2&q=%s", mQueryConfig.Instance, mQueryConfig.Query)
	if err := dbClient.PutInDB(ctx, ChromiumClient, MonorailSource, mQueryConfig.Query, queryLink, openCount, unassignedCount); err != nil {
		return skerr.Wrapf(err, "error putting monorail results in DB")
	}
	return nil
}

func (m *Monorail) GetLink(instance, id string) string {
	return fmt.Sprintf("https://bugs.chromium.org/p/%s/issues/detail?id=%s", instance, id)
}
