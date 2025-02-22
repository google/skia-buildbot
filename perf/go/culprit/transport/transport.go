package transport

import (
	"context"
	"fmt"
	"strconv"

	"go.skia.org/infra/go/issuetracker/v1"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/config"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

const (
	BASE_PATH = "https://issuetracker.googleapis.com"
)

// Transport has implementations for issuetracker.
type Transport interface {
	SendNewNotification(ctx context.Context, subscription *sub_pb.Subscription, subject string, body string) (threadingReference string, err error)
}

// IssueTrackerTransport implements Transport using the issue tracker API.
type IssueTrackerTransport struct {
	client                     *issuetracker.Service
	SendNewNotificationSuccess metrics2.Counter
	SendNewNotificationFail    metrics2.Counter
}

// NewIssueTrackerTransport returns a new IssueTrackerTransport.
func NewIssueTrackerTransport(ctx context.Context, cfg *config.CulpritNotifyConfig) (*IssueTrackerTransport, error) {
	secretClient, err := secret.NewClient(ctx)
	if err != nil {
		return nil, skerr.Wrapf(err, "creating secret client")
	}
	apiKey, err := secretClient.Get(ctx, cfg.IssueTrackerAPIKeySecretProject, cfg.IssueTrackerAPIKeySecretName, secret.VersionLatest)
	if err != nil {
		return nil, skerr.Wrapf(err, "loading API Key secrets from project: %q  name: %q", cfg.IssueTrackerAPIKeySecretProject, cfg.IssueTrackerAPIKeySecretName)
	}

	client, err := google.DefaultClient(ctx, "https://www.googleapis.com/auth/buganizer")
	if err != nil {
		return nil, skerr.Wrapf(err, "creating authorized HTTP client")
	}
	c, err := issuetracker.NewService(ctx, option.WithAPIKey(apiKey), option.WithHTTPClient(client))
	if err != nil {
		return nil, skerr.Wrapf(err, "creating issuetracker service")
	}
	c.BasePath = BASE_PATH

	return &IssueTrackerTransport{
		client:                     c,
		SendNewNotificationSuccess: metrics2.GetCounter("perf_issue_tracker_sent_new_culprit"),
		SendNewNotificationFail:    metrics2.GetCounter("perf_issue_tracker_sent_new_culprit_fail"),
	}, nil
}

// SendNewNotification implements Transport.
func (t *IssueTrackerTransport) SendNewNotification(ctx context.Context,
	subscription *sub_pb.Subscription, subject string, body string) (string, error) {
	if subscription.BugComponent == "" {
		return "", fmt.Errorf("no bug component set for subscription %s~%s", subscription.Name, subscription.Revision)
	}
	componentId, err := strconv.Atoi(subscription.BugComponent)
	if err != nil {
		return "", nil
	}
	hotlists := []int64{}
	for _, i := range subscription.Hotlists {
		j, err := strconv.ParseInt(i, 10, 64)
		if err != nil {
			panic(err)
		}
		hotlists = append(hotlists, j)
	}
	ccs := []*issuetracker.User{}
	for _, i := range subscription.BugCcEmails {
		j := &issuetracker.User{
			EmailAddress: i,
		}
		ccs = append(ccs, j)
	}
	newIssue := &issuetracker.Issue{
		IssueComment: &issuetracker.IssueComment{
			Comment:        string(body),
			FormattingMode: "MARKDOWN",
		},
		IssueState: &issuetracker.IssueState{
			ComponentId: int64(componentId),
			Priority:    fmt.Sprintf("P%d", subscription.BugPriority),
			Severity:    fmt.Sprintf("S%d", subscription.BugSeverity),
			Reporter: &issuetracker.User{
				EmailAddress: subscription.ContactEmail,
			},
			Ccs:         ccs,
			HotlistIds:  hotlists,
			AccessLimit: &issuetracker.IssueAccessLimit{AccessLevel: "LIMIT_VIEW_TRUSTED"},
			// TODO(pasthana): Set Assignee to the culprit cl author and set status to ASSIGNED
			Status: "NEW",
			Title:  string(subject),
		},
	}
	resp, err := t.client.Issues.Create(newIssue).TemplateOptionsApplyTemplate(true).Do()
	if err != nil {
		t.SendNewNotificationFail.Inc(1)
		return "", skerr.Wrapf(err, "creating issue")
	}
	t.SendNewNotificationSuccess.Inc(1)
	return strconv.Itoa(int(resp.IssueId)), nil
}
