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
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	issuetracker "go.skia.org/infra/go/issuetracker/v1"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

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
func NewIssueTracker(ctx context.Context, cfg config.IssueTrackerConfig, fetchAnomFromSql bool, regStore regression.Store, devMode bool) (IssueTracker, error) {
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

func (s *issueTrackerImpl) getComponentID(ctx context.Context, subscriptions []*pb.Subscription) (int64, error) {
	componentID := int64(-1)
	for _, sub := range subscriptions {
		id, err := strconv.ParseInt(sub.BugComponent, 10, 64)
		if err != nil {
			return -1, err
		}
		if componentID == -1 {
			componentID = id
			continue
		}
		if componentID != id {
			return -1, skerr.Fmt("cannot file a bug against multiple components at once")
		}
	}
	if componentID == -1 {
		return -1, skerr.Fmt("failed to retrieve the bug component from a subscription list of length %d", len(subscriptions))
	}
	return componentID, nil
}

// TODO(b/454614028) Inspect filed bugs. Determine whether Keys, TraceNames, Host, or Label
// should be included. In particular, labels will be missing in the new bugs.
func (s *issueTrackerImpl) FileBug(ctx context.Context, req *FileBugRequest) (int, error) {
	if req == nil {
		return 0, skerr.Fmt("File bug request is null.")
	}

	if !s.FetchAnomaliesFromSql {
		return 0, skerr.Fmt("this implementation is supposed to use the DB. Please contact BERF engineers at go/berf-skia-chat.")
	}

	regressionIds := req.Keys
	regressionIdsFromSql, alertsIds, subscriptions, err := s.regStore.GetSubscriptionsForRegressions(ctx, regressionIds)
	if err != nil {
		return 0, skerr.Wrapf(err, "failed to get alert ids for regressions.")
	}

	if len(regressionIdsFromSql) != len(regressionIds) {
		return 0, skerr.Fmt("could not find alert configurations or subscriptions for some regressions")
	}

	// TODO(b/454614028) Not sure alertsIds will be useful.
	_ = alertsIds

	componentID, err := s.getComponentID(ctx, subscriptions)
	if err != nil {
		return 0, err
	}

	// Most fields should be present in the DB
	if req.Component != fmt.Sprintf("%d", componentID) {
		sklog.Warningf("we ignore componentID passed by fe and use data from the db")
	}

	// TODO(b/454614028) remove this assignment after migration is done.
	// This is to prevent spamming other teams while testing.
	sklog.Warningf("File Bug would use the following component: %d. Using a default component until migration is done.", componentID)
	componentID = 1325852

	var ccs []*issuetracker.User
	for _, cc := range req.Ccs {
		ccs = append(ccs, &issuetracker.User{EmailAddress: cc})
	}

	newIssue := &issuetracker.Issue{
		IssueComment: &issuetracker.IssueComment{
			Comment:        req.Description,
			FormattingMode: "MARKDOWN",
		},
		IssueState: &issuetracker.IssueState{
			ComponentId: componentID,
			Priority:    "P2",
			Severity:    "S2",
			Status:      "NEW",
			Title:       req.Title,
			Assignee: &issuetracker.User{
				EmailAddress: req.Assignee,
			},
			Ccs: ccs,
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
	return int(resp.IssueId), nil
}
