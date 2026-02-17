package issuetracker

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	issuetracker "go.skia.org/infra/go/issuetracker/v1"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"

	v1 "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	ags "go.skia.org/infra/perf/go/anomalygroup/service"
)

var TOP_ANOMALIES_COUNT = 10

// IssueTracker defines an interface for accessing issuetracker v1 api.
type IssueTracker interface {
	// ListIssues sends a GET request to issuetracker api with the specified query parameter.
	// The response from the api is unmarshalled into the provided response object.
	ListIssues(ctx context.Context, requestObj ListIssuesRequest) ([]*issuetracker.Issue, error)

	// CreateComment adds a new comment to a existing bug given the bug id and the comment body.
	CreateComment(ctx context.Context, req *CreateCommentRequest) (*CreateCommentResponse, error)

	// FileBug craetes a new bug.
	FileBug(ctx context.Context, req *FileBugRequest) (int, error)
}

// / IssueTrackerImpl implements IssueTracker using the issue tracker API
type issueTrackerImpl struct {
	client                *issuetracker.Service
	FetchAnomaliesFromSql bool
	regStore              regression.Store
	urlBase               string
}

// ListIssuesRequest defines the request object for ListIssues.
type ListIssuesRequest struct {
	IssueIds []int `json:"issueIds"`
}

// CreateCommentRequest is the request object for CreateComment
type CreateCommentRequest struct {
	IssueId int64  `json:"issue_id"`
	Comment string `json:"comment"`
}

// CreateCommentResponse is the response object for CreateComment
type CreateCommentResponse struct {
	IssueId       int64 `json:"issud_id"`
	CommentNumber int64 `json:"comment_number"`
}

// FileBugRequest is the request object for filing a bug.
type FileBugRequest struct {
	Keys        []string `json:"keys"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Component   string   `json:"component"`
	Assignee    string   `json:"assignee,omitempty"`
	Ccs         []string `json:"ccs,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	TraceNames  []string `json:"trace_names,omitempty"`
	Host        string   `json:"host,omitempty"`
}

func setupSecretClient(ctx context.Context, cfg config.IssueTrackerConfig, options []option.ClientOption) (*http.Client, []option.ClientOption, error) {
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "creating secret client")
	}
	apiKey, err := secretClient.Get(ctx, cfg.IssueTrackerAPIKeySecretProject, cfg.IssueTrackerAPIKeySecretName, secret.VersionLatest)
	if err != nil {
		return nil, nil, skerr.Wrapf(err, "loading API Key secrets from project: %q  name: %q", cfg.IssueTrackerAPIKeySecretProject, cfg.IssueTrackerAPIKeySecretName)
	}
	options = append(options, option.WithAPIKey(apiKey))

	client, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/buganizer")
	return client, options, err
}

// NewIssueTracker returns a new issueTracker object.
func NewIssueTracker(ctx context.Context, cfg config.IssueTrackerConfig, fetchAnomFromSql bool, regStore regression.Store, devMode bool, urlBase string) (IssueTracker, error) {
	var client *http.Client
	var err error
	var options []option.ClientOption

	if devMode {
		sklog.Warning("Using a mock issue tracker.")
		client = http.DefaultClient
	} else {
		client, options, err = setupSecretClient(ctx, cfg, options)
		if err != nil {
			return nil, skerr.Wrapf(err, "creating authorized HTTP client")
		}
	}
	options = append(options, option.WithHTTPClient(client))
	c, err := issuetracker.NewService(ctx, options...)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating issuetracker service")
	}

	if devMode {
		c.BasePath = "http://localhost:8081"
	} else {
		c.BasePath = "https://issuetracker.googleapis.com"
	}

	return &issueTrackerImpl{
		client:                c,
		FetchAnomaliesFromSql: fetchAnomFromSql,
		regStore:              regStore,
		urlBase:               urlBase,
	}, nil
}

func (s *issueTrackerImpl) CreateComment(ctx context.Context, req *CreateCommentRequest) (*CreateCommentResponse, error) {
	if req == nil {
		return nil, skerr.Fmt("Create comment request is null.")
	}
	if req.IssueId <= 0 || req.Comment == "" {
		return nil, skerr.Fmt("Invalid CreateCommentRequest properties. Issue Id: %d, Comment: %s", req.IssueId, req.Comment)
	}

	sklog.Debugf("[Perf_issuetracker] Received CreateCommentRequest. Issue Id: %d, Comment: %s", req.IssueId, req.Comment)
	// FormattingMode is default to PLAIN
	issueComment := &issuetracker.IssueComment{
		Comment: req.Comment,
	}
	resp, err := s.client.Issues.Comments.Create(int64(req.IssueId), issueComment).Do()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create a new comment for bug id: %d. Comment: %s", req.IssueId, req.Comment)
	}
	sklog.Debugf("[Perf_issuetracker] CreateCommentResponse. Issue Id: %d, comment Id: %d", resp.IssueId, resp.CommentNumber)
	return &CreateCommentResponse{
		IssueId:       int64(resp.IssueId),
		CommentNumber: int64(resp.CommentNumber),
	}, nil
}

// ListIssues finds the specified issueID and returns a list of issue object.
func (s *issueTrackerImpl) ListIssues(ctx context.Context, requestObj ListIssuesRequest) ([]*issuetracker.Issue, error) {
	slice := make([]string, len(requestObj.IssueIds))
	for i, issueId := range requestObj.IssueIds {
		slice[i] = strconv.Itoa(issueId)
	}

	query := strings.Join(slice, " | ")
	query = "id:(" + query + ")"
	if len(requestObj.IssueIds) == 0 {
		return nil, skerr.Wrapf(nil, "No issue IDs provided")
	}

	sklog.Debugf("[Perf_issuetracker] Start sending list issues request to v1 issuetracker with query: %s", query)
	resp, err := s.client.Issues.List().Query(query).Do()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to find issue with request. ")
	}
	if len(resp.Issues) > 0 {
		sklog.Debugf("[Perf_issuetracker] Received list issues response from v1: IssueID=%s, State=%s", resp.Issues[0].IssueId, resp.Issues[0].IssueState.Title)
	}

	return resp.Issues, nil
}

// TODO(b/454614028) Inspect filed bugs. Determine whether Keys, TraceNames, Host, or Label
// should be included. In particular, labels will be missing in the new bugs.
func (s *issueTrackerImpl) FileBug(ctx context.Context, req *FileBugRequest) (int, error) {
	if req == nil {
		return 0, skerr.Fmt("File bug request is null.")
	}

	if len(req.Keys) == 0 {
		return 0, skerr.Fmt("File bug received an empty list of regression ids..")
	}

	if !s.FetchAnomaliesFromSql {
		return 0, skerr.Fmt("this implementation is supposed to use the DB. Please contact BERF engineers at go/berf-skia-chat.")
	}

	if req.Description != "" {
		sklog.Warningf("ignoring description provided, creating our own.\nOld description: %s", req.Description)
	}

	regressionIds := req.Keys
	regressionIdsFromSql, alertsIds, subscriptions, err := s.regStore.GetSubscriptionsForRegressions(ctx, regressionIds)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to get alert ids for regressions.")
	}
	if len(regressionIdsFromSql) != len(regressionIds) {
		return 0, skerr.Fmt("could not find alert configurations or subscriptions for some regressions")
	}
	if len(subscriptions) < 1 {
		return 0, skerr.Fmt("did not find any subscriptions linked to those regressions")
	}
	regData, err := s.regStore.GetByIDs(ctx, regressionIds)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to get regressions for regression ids")
	}

	isTestRun := s.checkTestRun(subscriptions)
	mostImpactedSub, subCcs := s.selectSub(subscriptions)
	if req.Assignee != "" {
		sklog.Warningf("ignoring assignee from request and using the one from the db")
	}
	assignee := mostImpactedSub.ContactEmail
	componentID, err := strconv.Atoi(mostImpactedSub.BugComponent)
	if err != nil {
		sklog.Errorf("failed to convert bug component %s to int", mostImpactedSub.BugComponent)
		return -1, err
	}

	topAnomalies, err := ags.TopAnomaliesMedianCmp(regData, int64(TOP_ANOMALIES_COUNT))

	description := ""

	// TODO(b/464211673) Make sure the links lead to correct graphs.
	description += s.generateLinkToGraph(req.Keys)
	description += s.describeTopAnomalies(topAnomalies)
	description += s.describeBots(regData)

	// TODO(b/454614028) Not sure alertsIds will be useful.
	_ = alertsIds

	// Most fields should be present in the DB
	if req.Component != fmt.Sprintf("%d", componentID) {
		sklog.Warningf("we ignore componentID: %s passed by fe and use data from the db: %d", req.Component, componentID)
	}

	descriptionDebugSection := "\n\n## DEBUG BELOW\n\n"
	// TODO(b/454614028) remove this assignment after migration is done.
	// This is to prevent spamming other teams while testing.
	sklog.Warningf("File Bug would use the following component: %d. Using a default component until migration is done.", componentID)
	descriptionDebugSection += fmt.Sprintf("component %d should be used.\nUntil migration is done, we use the default one.\n", componentID)
	componentID = 1325852

	description += descriptionDebugSection + "\n\n"

	var ccs []*issuetracker.User
	for _, cc := range req.Ccs {
		ccs = append(ccs, &issuetracker.User{EmailAddress: cc})
	}

	if !isTestRun {
		sklog.Warningf("disable testrun flag in issuetracker.go to use real assignee and ccs")
		assignee = ""
		subCcs = []*issuetracker.User{}
	}
	ccs = append(ccs, subCcs...)

	newIssue := &issuetracker.Issue{
		IssueComment: &issuetracker.IssueComment{
			Comment:        description,
			FormattingMode: "MARKDOWN",
		},
		IssueState: &issuetracker.IssueState{
			ComponentId: int64(componentID),
			Priority:    fmt.Sprintf("P%d", mostImpactedSub.BugPriority),
			Severity:    fmt.Sprintf("S%d", mostImpactedSub.BugSeverity),
			Status:      "NEW",
			Title:       req.Title,
			Assignee: &issuetracker.User{
				EmailAddress: assignee,
			},
			Ccs:  ccs,
			Type: "BUG",
		},
	}

	resp, err := s.client.Issues.Create(newIssue).TemplateOptionsApplyTemplate(true).Do()
	if err != nil {
		return 0, skerr.Wrapf(err,
			"[Perf_issuetracker] failed to create issue: Title=%q, Assignee=%q, ComponentID=%d",
			newIssue.IssueState.Title,
			newIssue.IssueState.Assignee.EmailAddress,
			newIssue.IssueState.ComponentId,
		)
	}

	issueId := resp.IssueId

	_, err = s.client.Issues.Comments.Create(issueId, &issuetracker.IssueComment{
		Comment:        fmt.Sprintf("Link to graph by bugID: %s/u?bugID=%d", s.urlBase, issueId),
		FormattingMode: "MARKDOWN",
	}).Do()
	if err != nil {
		sklog.Errorf("failed to post comment with bugID due to err: %s", err)
	}
	return int(issueId), nil
}

// We run full impl against our test subscription
func (s *issueTrackerImpl) checkTestRun(subscriptions []*pb.Subscription) bool {
	for _, sub := range subscriptions {
		if len(sub.BugLabels) != 1 {
			return false
		}
		if sub.BugLabels[0] != "BerfDevTest" {
			return false
		}
	}
	return true
}

func (s *issueTrackerImpl) describeBots(regs []*regression.Regression) string {
	uniqueBots := make(map[string]struct{})
	for _, r := range regs {
		for _, b := range r.Frame.DataFrame.ParamSet["bot"] {
			uniqueBots[b] = struct{}{}
		}
	}
	sortedBots := slices.Collect(maps.Keys(uniqueBots))
	slices.Sort(sortedBots)
	desc := "  \nBots for regressions of this bug:  \n"
	for _, b := range sortedBots {
		desc += fmt.Sprintf("  - %s  \n", b)
	}
	return desc + "  \n\n"
}

func (s *issueTrackerImpl) describeTopAnomalies(anom []*v1.Anomaly) (desc string) {
	desc = fmt.Sprintf("Top %d anomalies in this report:  \n", len(anom))
	for _, a := range anom {
		desc += describeAnomaly(a)
	}
	return
}

func (s *issueTrackerImpl) generateLinkToGraph(keys []string) string {
	anomalyIdsLink := "anomalyIDs"
	graphLink := fmt.Sprintf("%s/u?%s=", s.urlBase, anomalyIdsLink)
	link := fmt.Sprintf("Link to graph with regressions:  \n  %s", graphLink)

	BAN_LONG_URLS := true
	// Links longer than 2k might be problematic. We will rely on report by bugID.
	MAX_LEN := 2000
	urlLength := len(link)

	for _, key := range keys {
		if BAN_LONG_URLS && urlLength+len(key)+1 >= MAX_LEN {
			sklog.Warningf("URL is too long, need to use link by bug id - there are %d regressions", len(keys))
			prefix := "The link to a graph with all regressions would be too long.  \n"
			prefix += "Please check the first comment for an alternative link to the graph\n\n"
			link = prefix
			break
		}
		link += key + ","
		urlLength += len(key) + 1
	}

	return fmt.Sprintf("%s\n\n", strings.TrimSuffix(link, ","))
}

// There may be several subscriptions
// We will choose data from the sub which defines the highest <priority,severity> pair
// Additionally, all sheriffs will be CCed on the bug
// Non-emptiness of the subscriptions list here is ensured in the FileBug method.
func (s *issueTrackerImpl) selectSub(nonEmptySubscriptions []*pb.Subscription) (topSub *pb.Subscription, allContactEmails []*issuetracker.User) {
	topSub = nonEmptySubscriptions[0]
	for _, sub := range nonEmptySubscriptions {
		if sub.BugPriority < topSub.BugPriority || (sub.BugPriority == topSub.BugPriority && sub.BugSeverity < topSub.BugSeverity) {
			topSub = sub
		}
		allContactEmails = append(allContactEmails, &issuetracker.User{EmailAddress: sub.ContactEmail})
	}
	return
}

func calcChange(before, after float32) float32 {
	return (after - before) / before * 100.0
}

func describeAnomaly(a *v1.Anomaly) string {
	return fmt.Sprintf("  - Bot: %s, Benchmark: %s, Measurement: %s, Story: %s.  \n    Change: %.4f -> %.4f (%+.2f%%); Commit range: %d -> %d\n\n",
		a.Paramset["bot"], a.Paramset["benchmark"], a.Paramset["measurement"], a.Paramset["story"],
		a.MedianBefore, a.MedianAfter, calcChange(a.MedianBefore, a.MedianAfter), a.StartCommit, a.EndCommit,
	)
}
