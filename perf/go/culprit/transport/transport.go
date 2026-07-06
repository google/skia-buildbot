package transport

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
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
	client                     perf_issuetracker.IssueTracker
	SendNewNotificationSuccess metrics2.Counter
	SendNewNotificationFail    metrics2.Counter
}

// NewIssueTrackerTransport returns a new IssueTrackerTransport.
func NewIssueTrackerTransport(client perf_issuetracker.IssueTracker) *IssueTrackerTransport {
	return &IssueTrackerTransport{
		client:                     client,
		SendNewNotificationSuccess: metrics2.GetCounter("perf_issue_tracker_sent_new_culprit"),
		SendNewNotificationFail:    metrics2.GetCounter("perf_issue_tracker_sent_new_culprit_fail"),
	}
}

// SendNewNotification implements Transport.
func (t *IssueTrackerTransport) SendNewNotification(ctx context.Context,
	subscription *sub_pb.Subscription, subject string, body string) (string, error) {
	if subscription.BugComponent == "" {
		return "", fmt.Errorf("no bug component set for subscription %s~%s", subscription.Name, subscription.Revision)
	}
	componentId, err := strconv.Atoi(subscription.BugComponent)
	if err != nil {
		return "", skerr.Wrapf(err, "failed to convert bug component %s to int", subscription.BugComponent)
	}

	reporter := subscription.ContactEmail
	if strings.TrimSpace(reporter) == "" {
		body += "\n\nWarning: subscription this issue belongs to has no proper contact email!"
	}

	req := &perf_issuetracker.CreateIssueRequest{
		Title:       subject,
		Description: body,
		ComponentId: int64(componentId),
		Priority:    fmt.Sprintf("P%d", subscription.BugPriority),
		Severity:    fmt.Sprintf("S%d", subscription.BugSeverity),
		Reporter:    reporter,
		Ccs:         subscription.BugCcEmails,
		AccessLevel: "LIMIT_VIEW_TRUSTED",
		Status:      "NEW",
	}

	issueId, err := t.client.CreateIssue(ctx, req)
	if err != nil {
		t.SendNewNotificationFail.Inc(1)
		errmsg := ""
		issueData, encodeErr := json.Marshal(req)
		if encodeErr == nil {
			errmsg = string(issueData)
		}
		return "", skerr.Wrapf(err, "creating issue: %s", errmsg)
	}
	t.SendNewNotificationSuccess.Inc(1)
	return strconv.FormatInt(issueId, 10), nil
}
