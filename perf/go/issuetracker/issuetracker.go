package issuetracker

// Initializes and polls the various issue frameworks.

import (
	"context"
	"encoding/json"

	"go.skia.org/infra/perf/go/config"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"

	issuetracker "go.skia.org/infra/go/issuetracker/v1"
)

// IssueTracker defines an interface for accessing issuetracker v1 api.
type IssueTracker interface {
	// ListIssues sends a GET request to issuetracker api with the specified query parameter.
	// The response from the api is unmarshalled into the provided response object.
	ListIssues(ctx context.Context, requestObj ListIssuesRequest) ([]*issuetracker.Issue, error)
}

// / issueTracker implements IssueTracker using the issue tracker API
type issueTracker struct {
	client *issuetracker.Service
}

// ListIssuesRequest defines the request object for ListIssues.
type ListIssuesRequest struct {
	Query string `json:"query"`
}

// NewIssueTracker returns a new issueTracker object.
func NewIssueTracker(ctx context.Context, cfg *config.NotifyConfig) (*issueTracker, error) {
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

	return &issueTracker{
		client: c,
	}, nil
}

// ListIssues finds the specified issueID and returns a list of issue object.
func (s *issueTracker) ListIssues(ctx context.Context, requestObj ListIssuesRequest) ([]*issuetracker.Issue, error) {
	requestBodyStr, err := json.Marshal(requestObj)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create chrome perf request.")
	}
	resp, err := s.client.Issues.List().Query(string(requestBodyStr)).Do()
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to find issue with request. ")
	}

	return resp.Issues, nil
}
