package issues

import (
	"context"

	"go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// IssueTrackerService uses the issue tracker API.
type IssueTrackerService struct {
	client *issuetracker.Service
}

const (
	issueTrackerSecretProject = "skia-infra-public"
	issueTrackerSecretName    = "perf-issue-tracker-apikey"
	issueTrackerComponent     = int64(1389473) // NPM-audit-mirror component.
	serviceAccountEmail       = "skia-npm-audit-mirror@skia-public.iam.gserviceaccount.com"

	// Values used when filing NPM-audit-mirror issues.
	issuePriority         = "P1"
	issueSeverity         = "S1"
	issueStatus           = "ASSIGNED"
	defaultCCUser         = "rmistry@google.com"
	issueAccessLevel      = "LIMIT_VIEW_TRUSTED"
	commentFormattingMode = "MARKDOWN"
)

// NewIssueTrackerService returns a new IssueTrackerService.
func NewIssueTrackerService(ctx context.Context) (*IssueTrackerService, error) {
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating secret client")
	}
	apiKey, err := secretClient.Get(ctx, issueTrackerSecretProject, issueTrackerSecretName, secret.VersionLatest)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading API Key secrets from project: %q  name: %q", issueTrackerSecretProject, issueTrackerSecretName)
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

	return &IssueTrackerService{
		client: c,
	}, nil
}

// MakeIssue files a new issue in the NPM-audit-mirror component using the provided parameters.
func (s *IssueTrackerService) MakeIssue(owner, title, body string) (*issuetracker.Issue, error) {
	newIssue := &issuetracker.Issue{
		IssueComment: &issuetracker.IssueComment{
			Comment:        body,
			FormattingMode: commentFormattingMode,
		},
		IssueState: &issuetracker.IssueState{
			AccessLimit: &issuetracker.IssueAccessLimit{
				AccessLevel: issueAccessLevel,
			},
			ComponentId: issueTrackerComponent,
			Priority:    issuePriority,
			Severity:    issueSeverity,
			Reporter: &issuetracker.User{
				EmailAddress: serviceAccountEmail,
			},
			Assignee: &issuetracker.User{
				EmailAddress: owner,
			},
			Status: issueStatus,
			Title:  title,
			Ccs: []*issuetracker.User{
				{EmailAddress: defaultCCUser},
				{EmailAddress: serviceAccountEmail},
			},
		},
	}

	resp, err := s.client.Issues.Create(newIssue).TemplateOptionsApplyTemplate(true).Do()
	if err != nil {
		return nil, skerr.Wrapf(err, "creating issue")
	}
	return resp, nil
}

// GetIssue finds the specified issueID and returns the issue object.
func (s *IssueTrackerService) GetIssue(issueId int64) (*issuetracker.Issue, error) {
	resp, err := s.client.Issues.Get(issueId).Do()
	if err != nil {
		return nil, skerr.Wrapf(err, "finding issue")
	}
	return resp, nil
}
