package monorail

// Accesses monorail v3 pRPC based API (go/monorail-v3-api).
// TODO(rmistry): Switch this to use the Go client library whenever it is available (https://bugs.chromium.org/p/monorail/issues/detail?id=8257).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/bugs-central/go/bugs"
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
	monorailApiBase             = "https://api-dot-monorail-prod.appspot.com/prpc/"
	monorailTokenTargetAudience = "https://monorail-prod.appspot.com"
)

type monorailPriorityData struct {
	FieldName       string
	PriorityMapping map[string]types.StandardizedPriority

	P0Query        string
	P1Query        string
	P2Query        string
	P3AndRestQuery string
}

var (
	// Maps the various priority configurations of different projects into the standardized priorities.
	monorailProjectToPriorityData map[string]monorailPriorityData = map[string]monorailPriorityData{
		// https://bugs.chromium.org/p/skia/fields/detail?field=Priority
		"skia": {
			FieldName: "projects/skia/fieldDefs/9",
			PriorityMapping: map[string]types.StandardizedPriority{
				"Critical": types.PriorityP0,
				"High":     types.PriorityP1,
				"Medium":   types.PriorityP2,
				"Low":      types.PriorityP3,
				"Icebox":   types.PriorityP4,
			},
			P0Query:        "Priority=Critical",
			P1Query:        "Priority=High",
			P2Query:        "Priority=Medium",
			P3AndRestQuery: "Priority=Low,Icebox",
		},
		// https://bugs.chromium.org/p/chromium/fields/detail?field=Pri
		"chromium": {
			FieldName: "projects/chromium/fieldDefs/11",
			PriorityMapping: map[string]types.StandardizedPriority{
				"0": types.PriorityP0,
				"1": types.PriorityP1,
				"2": types.PriorityP2,
				"3": types.PriorityP3,
			},
			P0Query:        "Pri=0",
			P1Query:        "Pri=1",
			P2Query:        "Pri=2",
			P3AndRestQuery: "Pri=3",
		},
	}

	// Stores the results of User.GetUser calls so we do not wastefully have to keep making them.
	userToEmailCache map[string]string = map[string]string{}
)

type monorailIssue struct {
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

// monorail implements bugs.BugFramework for monorail repos.
type monorail struct {
	token       *oauth2.Token
	httpClient  *http.Client
	openIssues  *bugs.OpenIssues
	queryConfig *MonorailQueryConfig
}

// MonorailQueryConfig is the config that will be used when querying monorail API.
type MonorailQueryConfig struct {
	// Monorail instance to query.
	Instance string
	// Monorail query to run.
	Query string
	// Which client's issues we are looking for.
	Client types.RecognizedClient
	// Which statuses are considered as untriaged.
	UntriagedStatuses []string
	// Whether unassigned issues should be considered as untriaged.
	UnassignedIsUntriaged bool
}

// New returns an instance of the monorail implementation of bugs.BugFramework.
func New(ctx context.Context, serviceAccountFilePath string, openIssues *bugs.OpenIssues, queryConfig *MonorailQueryConfig) (bugs.BugFramework, error) {
	// Perform auth as described in https://docs.google.com/document/d/1Gx78HMBexadFm-jTOCcbFAXGCtucrN-0ET1mUd_hrHQ/edit#heading=h.a9iny4rfah43
	clientOption := idtoken.WithCredentialsFile(serviceAccountFilePath)
	ts, err := idtoken.NewTokenSource(ctx, monorailTokenTargetAudience, clientOption)
	if err != nil {
		return nil, skerr.Wrapf(err, "error running idtoken.NewTokenSource")
	}
	token, err := ts.Token()
	if err != nil {
		return nil, skerr.Wrapf(err, "error running ts.Token")
	}

	return &monorail{
		token:       token,
		httpClient:  httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client(),
		openIssues:  openIssues,
		queryConfig: queryConfig,
	}, nil
}

// makeJSONCall calls monorail's v3 pRPC based API (go/monorail-v3-api).
func (m *monorail) makeJSONCall(bodyJSON []byte, service string, method string) ([]byte, error) {
	path := monorailApiBase + fmt.Sprintf("monorail.v3.%s/%s", service, method)

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

// searchIssuesWithPagination returns monorail issue results by autoamtically paginating till end of results.
// Monorail results are limited to 100 (see https://source.chromium.org/chromium/infra/infra/+/master:appengine/monorail/api/v3/api_proto/issues.proto;l=179). It paginates till all results are received.
func (m *monorail) searchIssuesWithPagination() ([]monorailIssue, error) {
	issues := []monorailIssue{}
	mc := m.queryConfig

	// Put in a loop till there are no new pages.
	nextPageToken := ""
	for {
		query := fmt.Sprintf(`{"projects": ["projects/%s"], "query": "%s", "page_token": "%s"}`, mc.Instance, mc.Query, nextPageToken)
		b, err := m.makeJSONCall([]byte(query), "Issues", "SearchIssues")
		if err != nil {
			return nil, skerr.Wrapf(err, "Issues.SearchIssues JSON API call failed")
		}
		var monorailIssues struct {
			Issues        []monorailIssue `json:"issues"`
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

// See documentation for bugs.Search interface.
func (m *monorail) Search(ctx context.Context) ([]*types.Issue, *types.IssueCountsData, error) {
	monorailIssues, err := m.searchIssuesWithPagination()
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "error when searching issues")
	}

	// Convert monorail issues into bug_framework's generic issues
	issues := []*types.Issue{}
	countsData := &types.IssueCountsData{}
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
					return nil, nil, skerr.Wrapf(err, "Users.GetUser JSON API call failed")
				}
				var monorailUser struct {
					DisplayName string `json:"displayName"`
				}
				if err := json.Unmarshal(b, &monorailUser); err != nil {
					return nil, nil, err
				}
				// Cache results for next time.
				userToEmailCache[mi.Owner.User] = monorailUser.DisplayName
				owner = monorailUser.DisplayName
			}
		}

		// Find priority using MonorailProjectToPriorityData
		priority := types.StandardizedPriority("")
		if priorityData, ok := monorailProjectToPriorityData[m.queryConfig.Instance]; ok {
			for _, fv := range mi.FieldValues {
				if priorityData.FieldName == fv.Field {
					// Found the priority field for this project. Now translate
					// the priority field value into the generic priority value (P0, P1, ...)
					if p, ok := priorityData.PriorityMapping[fv.Value]; ok {
						priority = p
						break
					} else {
						sklog.Errorf("Could not find priority value %s for project %s", fv.Value, m.queryConfig.Instance)
					}
				}
			}
		} else {
			sklog.Errorf("Could not find MonorailProjectToPriorityData for project %s", m.queryConfig.Instance)
		}

		// Populate counts data.
		countsData.OpenCount++
		if owner == "" {
			countsData.UnassignedCount++
		}
		countsData.IncPriority(priority)
		sloViolation, reason, d := types.IsPrioritySLOViolation(time.Now(), mi.CreatedTime, mi.ModifiedTime, priority)
		countsData.IncSLOViolation(sloViolation, priority)
		if util.In(mi.State.Status, m.queryConfig.UntriagedStatuses) {
			countsData.UntriagedCount++
		} else if m.queryConfig.UnassignedIsUntriaged && owner == "" {
			countsData.UntriagedCount++
		}

		// Monorail issue names look like "projects/skia/issues/10783". Extract out the "10783".
		nameTokens := strings.Split(mi.Name, "/")
		id := nameTokens[len(nameTokens)-1]

		issues = append(issues, &types.Issue{
			Id:       id,
			State:    mi.State.Status,
			Priority: priority,
			Owner:    owner,
			Link:     m.GetIssueLink(m.queryConfig.Instance, id),

			SLOViolation:         sloViolation,
			SLOViolationReason:   reason,
			SLOViolationDuration: d,

			CreatedTime:  mi.CreatedTime,
			ModifiedTime: mi.ModifiedTime,

			Title: mi.Title,
		})
	}

	return issues, countsData, nil
}

// See documentation for bugs.SearchClientAndPersist interface.
func (m *monorail) SearchClientAndPersist(ctx context.Context, dbClient *db.FirestoreDB, runId string) error {
	qc := m.queryConfig
	issues, countsData, err := m.Search(ctx)
	if err != nil {
		return skerr.Wrapf(err, "error when searching monorail")
	}
	sklog.Infof("%s Monorail issues %+v", qc.Client, countsData)

	queryDesc := qc.Query
	countsData.QueryLink = fmt.Sprintf("https://bugs.chromium.org/p/%s/issues/list?can=2&q=%s", qc.Instance, qc.Query)
	untriagedTokens := []string{}
	if len(qc.UntriagedStatuses) > 0 {
		statusesQuery := fmt.Sprintf("status:%s", strings.Join(qc.UntriagedStatuses, ","))
		untriagedTokens = append(untriagedTokens, statusesQuery)
	}
	if qc.UnassignedIsUntriaged {
		untriagedTokens = append(untriagedTokens, "-has:owner")
	}
	countsData.UntriagedQueryLink = fmt.Sprintf("%s (%s)", countsData.QueryLink, strings.Join(untriagedTokens, " OR "))
	// Calculate priority links.
	if priorityData, ok := monorailProjectToPriorityData[m.queryConfig.Instance]; ok {
		countsData.P0Link = fmt.Sprintf("%s %s", countsData.QueryLink, priorityData.P0Query)
		countsData.P1Link = fmt.Sprintf("%s %s", countsData.QueryLink, priorityData.P1Query)
		countsData.P2Link = fmt.Sprintf("%s %s", countsData.QueryLink, priorityData.P2Query)
		countsData.P3AndRestLink = fmt.Sprintf("%s %s", countsData.QueryLink, priorityData.P3AndRestQuery)
	}
	client := qc.Client

	// Put in DB.
	if err := dbClient.PutInDB(ctx, client, types.MonorailSource, queryDesc, runId, countsData); err != nil {
		return skerr.Wrapf(err, "error putting monorail results in DB")
	}
	// Put in memory.
	m.openIssues.PutOpenIssues(client, types.MonorailSource, queryDesc, issues)
	return nil
}

// See documentation for bugs.GetIssueLink interface.
func (m *monorail) GetIssueLink(instance, id string) string {
	return fmt.Sprintf("https://bugs.chromium.org/p/%s/issues/detail?id=%s", instance, id)
}

// See documentation for bugs.SetOwnerAndAddComment interface.
func (m *monorail) SetOwnerAndAddComment(owner, comment, id string) error {
	query := fmt.Sprintf(`{"deltas": [{"issue": {"name": "projects/%s/issues/%s", "owner": {"user": "users/%s"}}, "update_mask": "owner"}], "comment_content": "%s", "notify_type": "EMAIL"}`, m.queryConfig.Instance, id, owner, comment)
	if _, err := m.makeJSONCall([]byte(query), "Issues", "ModifyIssues"); err != nil {
		return skerr.Wrapf(err, "Issues.ModifyIssues JSON API call failed")
	}
	return nil
}
