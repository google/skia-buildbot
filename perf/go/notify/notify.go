// Package notify is a package for sending notifications.
package notify

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/stepfit"
)

// Formatter has implementations for both HTML and Markdown.
type Formatter interface {
	// Return body and subject.
	FormatNewRegression(ctx context.Context, c provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string) (string, string, error)
	FormatRegressionMissing(ctx context.Context, c provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string) (string, string, error)
}

// Transport has implementations for email, issuetracker, and the noop implementation.
type Transport interface {
	SendNewRegression(ctx context.Context, alert *alerts.Alert, body, subject string) (threadingReference string, err error)
	SendRegressionMissing(ctx context.Context, threadingReference string, alert *alerts.Alert, body, subject string) (err error)
}

const (
	fromAddress = "alertserver@skia.org"
)

// context is used in expanding the message templates.
type templateContext struct {
	URL     string
	Commit  provider.Commit
	Alert   *alerts.Alert
	Cluster *clustering2.ClusterSummary
}

// Notifier sends notifications.
type Notifier struct {
	formatter Formatter

	transport Transport

	// url is the URL of this instance of Perf.
	url string
}

// newNotifier returns a newNotifier Notifier.
func newNotifier(formatter Formatter, transport Transport, url string) *Notifier {
	return &Notifier{
		formatter: formatter,
		transport: transport,
		url:       url,
	}
}

// RegressionFound sends a notification for the given cluster found at the given commit. Where to send it is defined in the alerts.Config.
func (n *Notifier) RegressionFound(ctx context.Context, c provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary) (string, error) {
	body, subject, err := n.formatter.FormatNewRegression(ctx, c, alert, cl, n.url)
	if err != nil {
		return "", err
	}
	threadingReference, err := n.transport.SendNewRegression(ctx, alert, body, subject)
	if err != nil {
		return "", skerr.Wrapf(err, "sending new regression message")
	}

	return threadingReference, nil
}

// RegressionMissing sends a notification that a previous regression found for
// the given cluster found at the given commit has disappeared after more data
// has arrived. Where to send it is defined in the alerts.Config.
func (n *Notifier) RegressionMissing(ctx context.Context, c provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, threadingReference string) error {
	body, subject, err := n.formatter.FormatRegressionMissing(ctx, c, alert, cl, n.url)
	if err != nil {
		return err
	}
	if err := n.transport.SendRegressionMissing(ctx, threadingReference, alert, body, subject); err != nil {
		return skerr.Wrapf(err, "sending regression missing message")
	}

	return nil
}

// ExampleSend sends an example for dummy data for the given alerts.Config.
func (n *Notifier) ExampleSend(ctx context.Context, alert *alerts.Alert) error {
	c := provider.Commit{
		Subject:   "An example commit use for testing.",
		URL:       "https://skia.googlesource.com/skia/+show/d261e1075a93677442fdf7fe72aba7e583863664",
		GitHash:   "d261e1075a93677442fdf7fe72aba7e583863664",
		Timestamp: 1498176000,
	}
	cl := &clustering2.ClusterSummary{
		Num: 10,
		StepFit: &stepfit.StepFit{
			Status: stepfit.HIGH,
		},
	}
	threadingReference, err := n.RegressionFound(ctx, c, alert, cl)
	if err != nil {
		return skerr.Wrap(err)
	}
	err = n.RegressionMissing(ctx, c, alert, cl, threadingReference)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// New returns a Notifier of the selected type.
func New(ctx context.Context, cfg *config.NotifyConfig, URL string) (*Notifier, error) {
	switch cfg.Notifications {
	case notifytypes.None:
		return newNotifier(NewHTMLFormatter(), NewNoopTransport(), URL), nil
	case notifytypes.HTMLEmail:
		return newNotifier(NewHTMLFormatter(), NewEmailTransport(), URL), nil
	case notifytypes.MarkdownIssueTracker:
		tracker, err := NewIssueTrackerTransport(ctx, cfg)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return newNotifier(NewMarkdownFormatter(), tracker, URL), nil

	default:
		return nil, skerr.Fmt("invalid Notifier type: %s, must be one of: %v", cfg.Notifications, notifytypes.AllNotifierTypes)
	}
}
