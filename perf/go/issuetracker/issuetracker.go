package issuetracker

// Initializes and polls the various issue frameworks.

import (
	"context"
	"strconv"
	"strings"

	"go.skia.org/infra/perf/go/config"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"

	issuetracker "go.skia.org/infra/go/issuetracker/v1"
)

// IssueTracker defines an interface for accessing issuetracker v1 api.
type IssueTracker interface {
	// ListIssues sends a GET request to issuetracker api with the specified query parameter.
	// The response from the api is unmarshalled into the provided response object.
	ListIssues(ctx context.Context, requestObj ListIssuesRequest) ([]*issuetracker.Issue, error)
}

// / IssueTrackerImpl implements IssueTracker using the issue tracker API
type issueTrackerImpl struct {
	client *issuetracker.Service
}

// ListIssuesRequest defines the request object for ListIssues.
type ListIssuesRequest struct {
	IssueIds []int `json:"issueIds"`
}

// NewIssueTracker returns a new issueTracker object.
func NewIssueTracker(ctx context.Context, cfg config.IssueTrackerConfig) (*issueTrackerImpl, error) {
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating secret client")
	}
	apiKey, err := secretClient.Get(ctx, cfg.IssueTrackerAPIKeySecretProject, cfg.IssueTrackerAPIKeySecretName, secret.VersionLatest)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading API Key secrets from project: %q  name: %q", cfg.IssueTrackerAPIKeySecretProject, cfg.IssueTrackerAPIKeySecretName)
	}

	client, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/buganizer")
	if err != nil {
		return nil, skerr.Wrapf(err, "creating authorized HTTP client")
	}
	c, err := issuetracker.NewService(ctx, option.WithAPIKey(apiKey), option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrapf(err, "creating issuetracker service")
	}
	c.BasePath = "https://issuetracker.googleapis.com"

	return &issueTrackerImpl{
		client: c,
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
	getIssueId := requestObj.IssueIds[0]
	sklog.Debugf("[Perf_issuetracker] Start sending list issues request to v1 issuetracker with query: %s", query)
	resp, err := s.client.Issues.List().Query(query).Do()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to find issue with request. ")
	}
	sklog.Debugf("[Perf_issuetracker] list issues response received from v1 issuetracker: %s", resp.Issues)
	// ===== debuging calls to verify issue tracker client =====
	sklog.Debugf("[Perf_issuetracker] Start sending get issue request to v1 issuetracker with issueId: %s", getIssueId)
	resp1, err := s.client.Issues.Get(int64(getIssueId)).Do()
	if err != nil {
		sklog.Debugf("[Perf_issuetracker] error on Get Issue debug call")
	}
	sklog.Debugf("[Perf_issuetracker] get issue response received from v1 issuetracker: %s", resp1.IssueId)

	q := "status:open"
	sklog.Debugf("[Perf_issuetracker] Start sending list request on open issues to v1 issuetracker with: %s", q)
	resp2, err := s.client.Issues.List().PageSize(10).Query(q).Do()
	if err != nil {
		sklog.Debugf("[Perf_issuetracker] error on List Issues debug call")
	}
	sklog.Debugf("[Perf_issuetracker] Start sending list request on open issues to v1 issuetracker with issueId: %s", len(resp2.Issues))
	// ===== end of debuging =====

	return resp.Issues, nil
}
