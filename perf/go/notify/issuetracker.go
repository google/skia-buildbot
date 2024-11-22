package notify

import (
	"context"
	"fmt"
	"strconv"

	"go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/config"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// IssueTrackerTransport implements Transport using the issue tracker API.
type IssueTrackerTransport struct {
	client                    *issuetracker.Service
	sendNewRegression         metrics2.Counter
	sendNewRegressionFail     metrics2.Counter
	sendRegressionMissing     metrics2.Counter
	sendRegressionMissingFail metrics2.Counter
}

// NewIssueTrackerTransport returns a new IssueTrackerTransport.
func NewIssueTrackerTransport(ctx context.Context, cfg *config.NotifyConfig) (*IssueTrackerTransport, error) {
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

	return &IssueTrackerTransport{
		client:                    c,
		sendNewRegression:         metrics2.GetCounter("perf_issue_tracker_sent_new_regression"),
		sendNewRegressionFail:     metrics2.GetCounter("perf_issue_tracker_sent_new_regression_fail"),
		sendRegressionMissing:     metrics2.GetCounter("perf_issue_tracker_sent_regression_missing"),
		sendRegressionMissingFail: metrics2.GetCounter("perf_issue_tracker_sent_regression_missing_fail"),
	}, nil
}

// SendNewRegression implements Transport.
func (t *IssueTrackerTransport) SendNewRegression(ctx context.Context, alert *alerts.Alert, body, subject string) (string, error) {
	if alert.IssueTrackerComponent == 0 {
		return "", fmt.Errorf("notification not sent, no issue tracker component set for alert #%s", alert.IDAsString)
	}

	newIssue := &issuetracker.Issue{
		IssueComment: &issuetracker.IssueComment{
			Comment:        body,
			FormattingMode: "MARKDOWN",
		},
		IssueState: &issuetracker.IssueState{
			ComponentId: int64(alert.IssueTrackerComponent),
			Priority:    "P2",
			Severity:    "S2",
			Reporter: &issuetracker.User{
				EmailAddress: alert.Owner,
			},
			Status: "NEW",
			Title:  subject,
		},
	}
	resp, err := t.client.Issues.Create(newIssue).TemplateOptionsApplyTemplate(true).Do()
	if err != nil {
		t.sendNewRegressionFail.Inc(1)
		return "", skerr.Wrapf(err, "creating issue")
	}
	t.sendNewRegression.Inc(1)
	return strconv.Itoa(int(resp.IssueId)), nil
}

// SendRegressionMissing implements Transport.
func (t *IssueTrackerTransport) SendRegressionMissing(ctx context.Context, threadingReference string, alert *alerts.Alert, body, subject string) error {
	issueID, err := strconv.ParseInt(threadingReference, 10, 64)
	if err != nil {
		return skerr.Wrapf(err, "invalid issue id #%s", threadingReference)
	}

	_, err = t.client.Issues.Modify(issueID, &issuetracker.ModifyIssueRequest{
		Add: &issuetracker.IssueState{
			Status: "OBSOLETE",
		},
		IssueComment: &issuetracker.IssueComment{
			Comment:        body,
			FormattingMode: "MARKDOWN",
		},
		AddMask: "status",
	}).Do()
	if err != nil {
		t.sendRegressionMissingFail.Inc(1)
		return skerr.Wrapf(err, "updating existing issue: %d", issueID)
	}
	t.sendRegressionMissing.Inc(1)
	return nil
}

// UpdateRegressionNotification updates the bug with a comment containing the details specified.
func (t *IssueTrackerTransport) UpdateRegressionNotification(ctx context.Context, alert *alerts.Alert, body, notificationId string) error {
	if alert.IssueTrackerComponent == 0 {
		return fmt.Errorf("notification not sent, no issue tracker component set for alert #%s", alert.IDAsString)
	}
	issueComment := &issuetracker.IssueComment{
		Comment:        body,
		FormattingMode: "MARKDOWN",
	}

	issueId, err := strconv.ParseInt(notificationId, 10, 64)
	if err != nil {
		return skerr.Wrapf(err, "Error parsing issue id: %s", notificationId)
	}
	_, err = t.client.Issues.Comments.Create(issueId, issueComment).Do()
	if err != nil {
		return skerr.Wrapf(err, "Error adding a comment on issue %d", issueId)
	}

	return nil
}
