package backends

import (
	"bytes"
	"context"
	"fmt"
	"text/template"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"

	_ "embed"

	issuetracker "go.skia.org/infra/go/issuetracker/v1"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

//go:embed culprit_detected.tmpl
var culpritDetected string

const (
	// Project name to fetch secret key from.
	secretKeyProject = "skia-infra-public"
	// API Key name in secretKeyProject that stores the actual API Key.
	secretAPIKey = "perf-issue-tracker-apikey"
)

type IssueTracker interface {
	ReportCulprit(issueID int64, culprits []*pinpoint_proto.CombinedCommit) error
}

// issueTrackerTransport implements IssueTracker.
type issueTrackerTransport struct {
	client *issuetracker.Service

	tmpl *template.Template
}

// configureTemplates sets up text templates for all Pinpoint comments.
func configureTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"toString": func(c *pinpoint_proto.CombinedCommit) string {
			str := fmt.Sprintf("%s/+/%s", c.Main.RepositoryUrl, c.Main.GitHash)
			for _, md := range c.ModifiedDeps {
				str += fmt.Sprintf(" %s/+/%s", md.RepositoryUrl, md.GitHash)
			}
			return str
		},
	}

	tmpl, err := template.New("pinpoint_templates").Funcs(funcMap).Parse(culpritDetected)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create template")
	}
	return tmpl, nil
}

// NewIssueTrackerTransport returns a issueTrackerTransport object configured with templates.
func NewIssueTrackerTransport(ctx context.Context) (*issueTrackerTransport, error) {
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating secret client")
	}
	apiKey, err := secretClient.Get(ctx, secretKeyProject, secretAPIKey, secret.VersionLatest)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading API Key secrets from project")
	}

	client, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/buganizer")
	if err != nil {
		return nil, skerr.Wrapf(err, "creating authorized HTTP client")
	}

	issueTrackerService, err := issuetracker.NewService(ctx, option.WithAPIKey(apiKey), option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrapf(err, "problem setting up issue tracker service")
	}

	issueTrackerService.BasePath = "https://issuetracker.googleapis.com"

	tmpl, err := configureTemplates()
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to load template files")
	}

	return &issueTrackerTransport{
		client: issueTrackerService,
		tmpl:   tmpl,
	}, nil
}

func (t *issueTrackerTransport) fillTemplate(culprits []*pinpoint_proto.CombinedCommit) (string, error) {
	if len(culprits) < 1 {
		// Return empty string, don't bother filling in template for 0 culprits.
		return "", nil
	}

	var b bytes.Buffer
	err := t.tmpl.Execute(&b, culprits)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to fill template")
	}

	return b.String(), nil
}

// ReportCulprit adds information about the culprit as a comment to the issueID.
//
// TODO(b/329502712) - The template lacks information about the regression
// because it's not yet returned from the bisect workflow.
func (t *issueTrackerTransport) ReportCulprit(issueID int64, culprits []*pinpoint_proto.CombinedCommit) error {
	content, err := t.fillTemplate(culprits)
	if err != nil {
		return skerr.Wrapf(err, "failed to generate comment to report")
	}
	_, err = t.client.Issues.Modify(issueID, &issuetracker.ModifyIssueRequest{
		IssueComment: &issuetracker.IssueComment{
			Comment:        content,
			FormattingMode: "MARKDOWN",
		},
	}).Do()
	if err != nil {
		return skerr.Wrapf(err, "failed to report culprit")
	}

	return nil
}
