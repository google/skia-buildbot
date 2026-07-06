package notify

import (
	"context"
	"fmt"
	"strconv"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	perf_issuetracker "go.skia.org/infra/perf/go/issuetracker"
)

// IssueTrackerTransport implements Transport using the issue tracker API.
type IssueTrackerTransport struct {
	client                    perf_issuetracker.IssueTracker
	sendNewRegression         metrics2.Counter
	sendNewRegressionFail     metrics2.Counter
	sendRegressionMissing     metrics2.Counter
	sendRegressionMissingFail metrics2.Counter
}

// NewIssueTrackerTransport returns a new IssueTrackerTransport.
func NewIssueTrackerTransport(client perf_issuetracker.IssueTracker) *IssueTrackerTransport {
	return &IssueTrackerTransport{
		client:                    client,
		sendNewRegression:         metrics2.GetCounter("perf_issue_tracker_sent_new_regression"),
		sendNewRegressionFail:     metrics2.GetCounter("perf_issue_tracker_sent_new_regression_fail"),
		sendRegressionMissing:     metrics2.GetCounter("perf_issue_tracker_sent_regression_missing"),
		sendRegressionMissingFail: metrics2.GetCounter("perf_issue_tracker_sent_regression_missing_fail"),
	}
}

// SendNewRegression implements Transport.
func (t *IssueTrackerTransport) SendNewRegression(ctx context.Context, alert *alerts.Alert, body, subject string) (string, error) {
	if alert.IssueTrackerComponent == 0 {
		return "", fmt.Errorf("notification not sent, no issue tracker component set for alert #%s", alert.IDAsString)
	}

	req := &perf_issuetracker.CreateIssueRequest{
		Title:       subject,
		Description: body,
		ComponentId: int64(alert.IssueTrackerComponent),
		Priority:    "P2",
		Severity:    "S2",
		Reporter:    alert.Owner,
		Status:      "NEW",
	}
	issueId, err := t.client.CreateIssue(ctx, req)
	if err != nil {
		t.sendNewRegressionFail.Inc(1)
		return "", skerr.Wrapf(err, "creating issue")
	}
	t.sendNewRegression.Inc(1)
	return strconv.FormatInt(issueId, 10), nil
}

// SendRegressionMissing implements Transport.
func (t *IssueTrackerTransport) SendRegressionMissing(ctx context.Context, threadingReference string, alert *alerts.Alert, body, subject string) error {
	issueID, err := strconv.ParseInt(threadingReference, 10, 64)
	if err != nil {
		return skerr.Wrapf(err, "invalid issue id #%s", threadingReference)
	}

	err = t.client.ModifyIssue(ctx, &perf_issuetracker.ModifyIssueRequest{
		IssueId: issueID,
		Status:  "OBSOLETE",
		Comment: body,
	})
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

	issueId, err := strconv.ParseInt(notificationId, 10, 64)
	if err != nil {
		return skerr.Wrapf(err, "Error parsing issue id: %s", notificationId)
	}
	_, err = t.client.CreateComment(ctx, &perf_issuetracker.CreateCommentRequest{
		IssueId: issueId,
		Comment: body,
	})
	if err != nil {
		return skerr.Wrapf(err, "Error adding a comment on issue %d", issueId)
	}

	return nil
}
