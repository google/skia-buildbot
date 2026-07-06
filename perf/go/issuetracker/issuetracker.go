package issuetracker

// Initializes and polls the various issue frameworks.

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/regrshortcut"
	"go.skia.org/infra/perf/go/types"
	"go.skia.org/infra/perf/go/userissue"
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
var COMPONENET_ID = 1325852
var ISSUESTATUS = "ASSIGNED"

// IssueTracker defines an interface for accessing issuetracker v1 api.
type IssueTracker interface {
	// ListIssues sends a GET request to issuetracker api with the specified query parameter.
	// The response from the api is unmarshalled into the provided response object.
	ListIssues(ctx context.Context, requestObj ListIssuesRequest) ([]*issuetracker.Issue, error)

	// CreateComment adds a new comment to a existing bug given the bug id and the comment body.
	CreateComment(ctx context.Context, req *CreateCommentRequest) (*CreateCommentResponse, error)

	// FileBug craetes a new bug.
	FileBug(ctx context.Context, req *FileBugRequest) (int, error)

	// FileUserIssue creates a new user issue.
	FileUserIssue(ctx context.Context, req *CreateUserIssueRequest) (int, error)

	// CreateIssue creates a new issue with raw parameters.
	CreateIssue(ctx context.Context, req *CreateIssueRequest) (int64, error)

	// ModifyIssue modifies an existing issue (status and/or comment).
	ModifyIssue(ctx context.Context, req *ModifyIssueRequest) error
}

// / IssueTrackerImpl implements IssueTracker using the issue tracker API
type issueTrackerImpl struct {
	client                   *issuetracker.Service
	FetchAnomaliesFromSql    bool
	OverrideComponent        bool
	regStore                 regression.Store
	regrShortcutStore        regrshortcut.Store
	userIssueStore           userissue.Store
	urlBase                  string
	commitHashRangeFormatter types.CommitHashRangeFormatter
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
// A few fields seem to be unused, we will monitor those.
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

// CreateUserIssueRequest is the request object for creating a user issue.
type CreateUserIssueRequest struct {
	TraceKey       string `json:"trace_key"`
	CommitPosition int64  `json:"commit_position"`
	Assignee       string `json:"assignee"`
}

// CreateIssueRequest defines the request object for CreateIssue.
type CreateIssueRequest struct {
	Title       string
	Description string
	ComponentId int64
	Priority    string // "P0"-"P4"
	Severity    string // "S0"-"S4"
	Reporter    string
	Assignee    string
	Ccs         []string
	AccessLevel string // e.g. "LIMIT_VIEW_TRUSTED"
	Status      string // e.g. "NEW"
}

// ModifyIssueRequest defines the request object for ModifyIssue.
type ModifyIssueRequest struct {
	IssueId int64
	Status  string // If non-empty, update status
	Comment string // If non-empty, add comment
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

// IssueTrackerDeps contains dependencies and configuration for creating a new IssueTracker.
type IssueTrackerDeps struct {
	Cfg                      config.IssueTrackerConfig
	FetchAnomaliesFromSql    bool
	OverrideBugComponent     bool
	RegStore                 regression.Store
	RegrShortcutStore        regrshortcut.Store
	UserIssueStore           userissue.Store
	DevMode                  bool
	UrlBase                  string
	CommitHashRangeFormatter types.CommitHashRangeFormatter
}

// NewIssueTracker returns a new issueTracker object.
func NewIssueTracker(ctx context.Context, deps IssueTrackerDeps) (IssueTracker, error) {
	var client *http.Client
	var err error
	var options []option.ClientOption

	if deps.DevMode {
		sklog.Warning("Using a mock issue tracker.")
		client = http.DefaultClient
	} else {
		client, options, err = setupSecretClient(ctx, deps.Cfg, options)
		if err != nil {
			return nil, skerr.Wrapf(err, "creating authorized HTTP client")
		}
	}
	options = append(options, option.WithHTTPClient(client))
	c, err := issuetracker.NewService(ctx, options...)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating issuetracker service")
	}

	if deps.DevMode {
		c.BasePath = "http://localhost:8081"
	} else {
		c.BasePath = "https://issuetracker.googleapis.com"
	}

	return &issueTrackerImpl{
		client:                   c,
		FetchAnomaliesFromSql:    deps.FetchAnomaliesFromSql,
		OverrideComponent:        deps.OverrideBugComponent,
		regStore:                 deps.RegStore,
		regrShortcutStore:        deps.RegrShortcutStore,
		userIssueStore:           deps.UserIssueStore,
		urlBase:                  deps.UrlBase,
		commitHashRangeFormatter: deps.CommitHashRangeFormatter,
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
		Comment:        req.Comment,
		FormattingMode: "MARKDOWN",
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

// Creates a buganizer issue based on the request and data in the DB.
// Example format of a bug description can be found here: https://b.corp.google.com/issues/485727375
func (s *issueTrackerImpl) FileBug(ctx context.Context, req *FileBugRequest) (int, error) {
	if req == nil {
		return 0, skerr.Fmt("File bug request is null.")
	}

	if len(req.Keys) == 0 {
		return 0, skerr.Fmt("File bug received an empty list of regression ids.")
	}

	if !s.FetchAnomaliesFromSql {
		return 0, skerr.Fmt("this implementation is supposed to use the DB. Please contact BERF engineers at go/berf-skia-chat.")
	}
	s.checkUnusedFieldsAreEmpty(req)

	regressionIds := req.Keys
	regressionIdsFromSql, subscriptions, err := s.regStore.GetSubscriptionsForRegressions(ctx, regressionIds)
	if err != nil {
		return 0, skerr.Wrapf(err, "DB failed to get subscriptions for regressions.")
	}
	if len(regressionIdsFromSql) != len(regressionIds) {
		return 0, skerr.Fmt("could not find subscriptions for some regressions")
	}
	if len(subscriptions) < 1 {
		return 0, skerr.Fmt("did not find any subscriptions linked to those regressions")
	}

	regData, err := s.regStore.GetByIDs(ctx, regressionIds)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to get regressions for regression ids")
	}
	if len(regData) != len(regressionIds) {
		return 0, skerr.Fmt("could not find data of some regressions")
	}

	isTestRun := s.checkTestRun(subscriptions)
	mostImpactedSub, subCcs := s.selectSub(subscriptions)

	componentID, err := strconv.Atoi(mostImpactedSub.BugComponent)
	if err != nil {
		sklog.Errorf("failed to convert bug component %s to int", mostImpactedSub.BugComponent)
		return -1, err
	}
	// Most fields should be present in the DB
	if req.Component != fmt.Sprintf("%d", componentID) {
		sklog.Warningf("we ignore componentID: %s passed by fe and use data from the db: %d", req.Component, componentID)
	}

	topAnomalies, err := ags.TopAnomaliesMedianCmp(regData, int64(TOP_ANOMALIES_COUNT))
	description := ""
	// TODO(b/464211673) Make sure the links lead to correct graphs.
	link, err := s.generateLinkToGraph(ctx, regressionIds)
	if err != nil {
		return 0, skerr.Wrap(err)
	}
	description += s.describeTopAnomalies(ctx, topAnomalies, link)
	description += s.intersectionFooter(ctx, regData)

	descriptionDebugSection := "\n\n## DEBUG BELOW\n\n"
	// This is to prevent spamming other teams while testing.
	if s.OverrideComponent {
		componentID = 1325852
		sklog.Warningf("File Bug would use the following component: %d. Using a default component until migration is done.", componentID)
		descriptionDebugSection += fmt.Sprintf("component %d should be used.\nUntil migration is done, we use the default one.\n", componentID)
		description += descriptionDebugSection + "\n\n"
	}

	var ccs []string
	ccs = append(ccs, req.Ccs...)

	assignee := mostImpactedSub.ContactEmail
	issueStatus := "ASSIGNED"
	if !isTestRun {
		sklog.Warningf("disable testrun flag in issuetracker.go to use real assignee and ccs")
		assignee = ""
		issueStatus = "NEW"
		subCcs = []string{}
	}
	// Our test subscription has Sergei set as the contact point.
	if assignee == "sergeirudenkov@google.com" {
		assignee = "berf-issuetracker-testing@google.com"
	}
	ccs = append(ccs, subCcs...)

	err = s.validateAssigneeAndStatus(assignee, issueStatus)
	if err != nil {
		return 0, err
	}

	issueId, err := s.CreateIssue(ctx, &CreateIssueRequest{
		Title:       req.Title,
		Description: description,
		ComponentId: int64(componentID),
		Priority:    fmt.Sprintf("P%d", mostImpactedSub.BugPriority),
		Severity:    fmt.Sprintf("S%d", mostImpactedSub.BugSeverity),
		Assignee:    assignee,
		Ccs:         ccs,
		Status:      issueStatus,
	})
	if err != nil {
		return 0, skerr.Wrapf(err,
			"[Perf_issuetracker] failed to create issue: Title=%q, Assignee=%q, ComponentID=%d",
			req.Title,
			assignee,
			componentID,
		)
	}

	_, err = s.CreateComment(ctx, &CreateCommentRequest{
		IssueId: issueId,
		Comment: fmt.Sprintf("Link to graph by bugID: %s/u?bugID=%d", s.urlBase, issueId),
	})
	if err != nil {
		sklog.Errorf("failed to post comment with bugID due to err: %s", err)
	}
	return int(issueId), nil
}

// FileUserIssue creates a new user issue.
func (s *issueTrackerImpl) FileUserIssue(ctx context.Context, req *CreateUserIssueRequest) (int, error) {
	if req == nil {
		return 0, skerr.Fmt("Create user issue request is null.")
	}

	issueStatus := ISSUESTATUS

	err := s.validateAssigneeAndStatus(req.Assignee, issueStatus)
	if err != nil {
		return 0, err
	}

	title := fmt.Sprintf("Trace ID %s shows a potential regression at commit position %d.", req.TraceKey, req.CommitPosition)

	issueId, err := s.CreateIssue(ctx, &CreateIssueRequest{
		Title:       title,
		Description: "",
		ComponentId: int64(COMPONENET_ID),
		Priority:    "P2",
		Severity:    "S2",
		Assignee:    req.Assignee,
		Status:      issueStatus,
	})
	if err != nil {
		return 0, skerr.Wrapf(err,
			"[Perf_issuetracker] failed to create user issue: Title=%q, Assignee=%q, ComponentID=%d",
			title,
			req.Assignee,
			COMPONENET_ID,
		)
	}

	_, err = s.CreateComment(ctx, &CreateCommentRequest{
		IssueId: issueId,
		Comment: fmt.Sprintf("Link to trace by bugID: %s/u?bugID=%d", s.urlBase, issueId),
	})
	if err != nil {
		sklog.Errorf("failed to post comment with bugID due to err: %s", err)
	}
	return int(issueId), nil
}

// We run full impl against our test subscription
func (s *issueTrackerImpl) checkTestRun(subscriptions []*pb.Subscription) bool {
	for _, sub := range subscriptions {
		if sub.ContactEmail != "sergeirudenkov@google.com" {
			return false
		}
	}
	return true
}

func (s *issueTrackerImpl) checkUnusedFieldsAreEmpty(req *FileBugRequest) {
	if req.Host != "" {
		sklog.Warningf("file bug: host %s is ignored", req.Host)
	}
	if len(req.Labels) > 0 {
		sklog.Warningf("file bug: labels are ignored")
	}
	if len(req.TraceNames) > 0 {
		sklog.Warningf("file bug: tracenames are ignored")
	}
	if req.Assignee != "" {
		sklog.Warningf("file bug: assignee %s is ignored, selecting from the DB", req.Assignee)
	}
	if req.Description != "" {
		sklog.Warningf("file bug: ignoring description on the request, creating our own")
	}
}

func (s *issueTrackerImpl) describeTopAnomalies(ctx context.Context, anom []*v1.Anomaly, link string) (desc string) {
	desc = fmt.Sprintf("Top %d anomalies in [this report](%s):  \n\n", len(anom), link)
	desc += generateAnomTableHeaders()
	for _, a := range anom {
		desc += s.describeAnomaly(ctx, a)
	}
	return
}

func (s *issueTrackerImpl) generateLinkToGraph(ctx context.Context, keys []string) (string, error) {
	if len(keys) == 0 {
		sklog.Error("generating empty graph, make sure it's just for testing!")
		return fmt.Sprintf("%s/u?anomalyIDs=", s.urlBase), nil
	}
	if len(keys) == 1 {
		return fmt.Sprintf("%s/u?anomalyIDs=%s", s.urlBase, keys[0]), nil
	}
	link := fmt.Sprintf("%s/u?sid=", s.urlBase)
	sid, err := s.regrShortcutStore.Create(ctx, keys)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to generate link to graph")
	}

	return link + strings.TrimPrefix(sid, "\\x"), nil
}

func (s *issueTrackerImpl) intersectionFooter(ctx context.Context, regData []*regression.Regression) string {
	if len(regData) == 0 {
		sklog.Error("empty regressions list - impossible, we've checked it earlier")
		return ""
	}
	begin := regData[0].PrevCommitNumber
	end := regData[0].CommitNumber
	for _, r := range regData {
		begin = max(begin, r.PrevCommitNumber)
		end = min(end, r.CommitNumber)
	}

	if begin >= end {
		return "\nCommit intersection of regressions in this bug is empty!\n"
	}

	// We add 1 since we want both ends inclusive.
	commitRange := fmt.Sprintf("\\[%d..%d\\]", begin+1, end)
	commitHashRange := "failed to generate"
	if s.commitHashRangeFormatter != nil {
		commitHashRange = s.commitHashRangeFormatter(ctx, int64(begin), int64(end))
	}
	return fmt.Sprintf("\nCommon commit range of all regressions in this bug: %s - Hash range: %s\n", commitRange, commitHashRange)
}

// There may be several subscriptions
// We will choose data from the sub which defines the highest <priority,severity> pair
// Additionally, all sheriffs will be CCed on the bug
// Non-emptiness of the subscriptions list here is ensured in the FileBug method.
func (s *issueTrackerImpl) selectSub(nonEmptySubscriptions []*pb.Subscription) (topSub *pb.Subscription, allContactEmails []string) {
	topSub = nonEmptySubscriptions[0]
	for _, sub := range nonEmptySubscriptions {
		if sub.BugPriority < topSub.BugPriority || (sub.BugPriority == topSub.BugPriority && sub.BugSeverity < topSub.BugSeverity) {
			topSub = sub
		}
		if trimmed := strings.TrimSpace(sub.ContactEmail); trimmed != "" {
			allContactEmails = append(allContactEmails, trimmed)
		}
	}
	return
}

func (s *issueTrackerImpl) validateAssigneeAndStatus(assignee string, status string) error {
	if assignee == "" && status != "NEW" {
		return skerr.Fmt("assignee is empty, status cannot be %s", status)
	}
	if assignee != "" && status == "NEW" {
		return skerr.Fmt("assignee is not empty, status must not be NEW")
	}
	return nil
}

func calcChange(before, after float32) float32 {
	return (after - before) / before * 100.0
}

func generateAnomTableHeaders() string {
	return "  \n| Bot | Benchmark | Measurement | Story | Change | Commit range | Commit Hashes | \n" +
		"| --- | --- | --- | --- | --- | --- | --- | \n"
}

func (s *issueTrackerImpl) describeAnomaly(ctx context.Context, a *v1.Anomaly) string {
	// MD table, see `generateAnomTableHeaders` for headers.
	// a.StartCommit is actually the commit position of previous, so we add 1.
	commitRange := fmt.Sprintf("\\[%d..%d\\]", a.StartCommit+1, a.EndCommit)
	commitHashRange := "failed to generate"
	if s.commitHashRangeFormatter != nil {
		commitHashRange = s.commitHashRangeFormatter(ctx, a.StartCommit, a.EndCommit)
	}

	return fmt.Sprintf("| %s | %s | %s | %s | %+.2f%% | %s | %s | \n",
		a.Paramset["bot"], a.Paramset["benchmark"], a.Paramset["measurement"], a.Paramset["story"],
		calcChange(a.MedianBefore, a.MedianAfter), commitRange, commitHashRange,
	)
}

// CreateIssue implements IssueTracker.
func (s *issueTrackerImpl) CreateIssue(ctx context.Context, req *CreateIssueRequest) (int64, error) {
	if req == nil {
		return 0, skerr.Fmt("Create issue request is null.")
	}

	ccs := []*issuetracker.User{}
	for _, email := range req.Ccs {
		if trimmedEmail := strings.TrimSpace(email); trimmedEmail != "" {
			ccs = append(ccs, &issuetracker.User{EmailAddress: trimmedEmail})
		}
	}

	var reporter *issuetracker.User
	if reporterEmail := strings.TrimSpace(req.Reporter); reporterEmail != "" {
		reporter = &issuetracker.User{EmailAddress: reporterEmail}
	}

	var assignee *issuetracker.User
	if assigneeEmail := strings.TrimSpace(req.Assignee); assigneeEmail != "" {
		assignee = &issuetracker.User{EmailAddress: assigneeEmail}
	}

	newIssue := &issuetracker.Issue{
		IssueComment: &issuetracker.IssueComment{
			Comment:        req.Description,
			FormattingMode: "MARKDOWN",
		},
		IssueState: &issuetracker.IssueState{
			ComponentId: req.ComponentId,
			Priority:    req.Priority,
			Severity:    req.Severity,
			Reporter:    reporter,
			Assignee:    assignee,
			Ccs:         ccs,
			Status:      req.Status,
			Title:       req.Title,
			Type:        "BUG",
		},
	}

	if req.AccessLevel != "" {
		newIssue.IssueState.AccessLimit = &issuetracker.IssueAccessLimit{
			AccessLevel: req.AccessLevel,
		}
	}

	resp, err := s.client.Issues.Create(newIssue).TemplateOptionsApplyTemplate(true).Do()
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to create issue: Title=%q, ComponentID=%d", req.Title, req.ComponentId)
	}

	return resp.IssueId, nil
}

// ModifyIssue implements IssueTracker.
func (s *issueTrackerImpl) ModifyIssue(ctx context.Context, req *ModifyIssueRequest) error {
	if req == nil {
		return skerr.Fmt("Modify issue request is null.")
	}
	if req.IssueId <= 0 {
		return skerr.Fmt("Invalid issue ID: %d", req.IssueId)
	}

	modifyReq := &issuetracker.ModifyIssueRequest{}
	mask := []string{}

	if req.Status != "" {
		modifyReq.Add = &issuetracker.IssueState{
			Status: req.Status,
		}
		mask = append(mask, "status")
	}

	if req.Comment != "" {
		modifyReq.IssueComment = &issuetracker.IssueComment{
			Comment:        req.Comment,
			FormattingMode: "MARKDOWN",
		}
	}

	if len(mask) > 0 {
		modifyReq.AddMask = strings.Join(mask, ",")
	}

	if req.Status != "" || req.Comment != "" {
		_, err := s.client.Issues.Modify(req.IssueId, modifyReq).Do()
		if err != nil {
			return skerr.Wrapf(err, "failed to modify issue %d", req.IssueId)
		}
	}

	return nil
}
