// Package notify is a package for sending notifications.
package notify

import (
	"context"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/alerts"
	ag "go.skia.org/infra/perf/go/anomalygroup/notifier"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/notifytypes"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/ui/frame"
)

// Formatter has implementations for both HTML and Markdown.
type Formatter interface {
	// Return body and subject.
	FormatNewRegression(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string, frame *frame.FrameResponse) (string, string, error)
	FormatRegressionMissing(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, URL string, frame *frame.FrameResponse) (string, string, error)
}

// Transport has implementations for email, issuetracker, and the noop implementation.
type Transport interface {
	SendNewRegression(ctx context.Context, alert *alerts.Alert, body, subject string) (threadingReference string, err error)
	SendRegressionMissing(ctx context.Context, threadingReference string, alert *alerts.Alert, body, subject string) (err error)
	UpdateRegressionNotification(ctx context.Context, alert *alerts.Alert, body, notificationId string) (err error)
}

const (
	fromAddress = "alertserver@skia.org"
)

// TemplateContext is used in expanding the message templates.
type TemplateContext struct {
	// URL is the root URL of the Perf instance.
	URL string

	// ViewOnDashboard is the URL to view the regressing traces on the explore
	// page.
	ViewOnDashboard string

	// PreviousCommit is the previous commit the regression was found at.
	//
	// All commits that might be blamed for causing the regression
	// are in the range `(PreviousCommit, Commit]`, that is inclusive of
	// Commit but exclusive of PreviousCommit.
	PreviousCommit provider.Commit

	// Commit is the commit the regression was found at.
	Commit provider.Commit

	// CommitURL is a URL that points to the above Commit. The value of this URL
	// can be controlled via the `--commit_range_url` flag.
	CommitURL string

	// Alert is the configuration for the alert that found the regression.
	Alert *alerts.Alert

	// Cluster is all the information found about the regression.
	Cluster *clustering2.ClusterSummary

	// ParamSet for all the matching traces.
	ParamSet paramtools.ReadOnlyParamSet
}

// Notifier provides an interface for regression notification functions
type Notifier interface {
	// RegressionFound sends a notification for the given cluster found at the given commit.
	RegressionFound(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, regressionID string) (string, error)

	// RegressionMissing sends a notification that a previous regression found for
	// the given cluster found at the given commit has disappeared after more data
	// has arrived.
	RegressionMissing(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, threadingReference string) error

	// ExampleSend sends an example for dummy data for the given alerts.Config.
	ExampleSend(ctx context.Context, alert *alerts.Alert) error

	UpdateNotification(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, notificationId string) error
}

// defaultNotifier sends notifications.
type defaultNotifier struct {
	formatter Formatter

	transport Transport

	// url is the URL of this instance of Perf.
	url string
}

// newNotifier returns a newNotifier Notifier.
func newNotifier(formatter Formatter, transport Transport, url string) Notifier {
	return &defaultNotifier{
		formatter: formatter,
		transport: transport,
		url:       url,
	}
}

// RegressionFound sends a notification for the given cluster found at the given commit. Where to send it is defined in the alerts.Config.
func (n *defaultNotifier) RegressionFound(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, regressionID string) (string, error) {
	body, subject, err := n.formatter.FormatNewRegression(ctx, commit, previousCommit, alert, cl, n.url, frame)
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
func (n *defaultNotifier) RegressionMissing(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, threadingReference string) error {
	body, subject, err := n.formatter.FormatRegressionMissing(ctx, commit, previousCommit, alert, cl, n.url, frame)
	if err != nil {
		return err
	}
	if err := n.transport.SendRegressionMissing(ctx, threadingReference, alert, body, subject); err != nil {
		return skerr.Wrapf(err, "sending regression missing message")
	}

	return nil
}

// ExampleSend sends an example for dummy data for the given alerts.Config.
func (n *defaultNotifier) ExampleSend(ctx context.Context, alert *alerts.Alert) error {
	commit := provider.Commit{
		Subject:   "An example commit use for testing.",
		URL:       "https://skia.googlesource.com/skia/+show/d261e1075a93677442fdf7fe72aba7e583863664",
		GitHash:   "d261e1075a93677442fdf7fe72aba7e583863664",
		Timestamp: 1498176000,
	}

	previousCommit := provider.Commit{
		Subject:   "An example previous commit to use for testing.",
		URL:       "https://skia.googlesource.com/skia/+/fb49909acafba5e031b90a265a6ce059cda85019",
		GitHash:   "fb49909acafba5e031b90a265a6ce059cda85019",
		Timestamp: 1687824470,
	}

	cl := &clustering2.ClusterSummary{
		Num: 10,
		StepFit: &stepfit.StepFit{
			Status: stepfit.HIGH,
		},
		StepPoint: &dataframe.ColumnHeader{
			Offset:    2,
			Timestamp: 1498176000,
		},
	}

	frame := &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: 1, Timestamp: 1687824470},
				{Offset: 2, Timestamp: 1498176000},
			},
			ParamSet: paramtools.ReadOnlyParamSet{
				"device_name": []string{"sailfish", "sargo", "wembley"},
			},
		},
	}

	threadingReference, err := n.RegressionFound(ctx, commit, previousCommit, alert, cl, frame, "")
	if err != nil {
		return skerr.Wrap(err)
	}
	err = n.RegressionMissing(ctx, commit, previousCommit, alert, cl, frame, threadingReference)
	if err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func (n *defaultNotifier) UpdateNotification(ctx context.Context, commit, previousCommit provider.Commit, alert *alerts.Alert, cl *clustering2.ClusterSummary, frame *frame.FrameResponse, notificationId string) error {
	body, _, err := n.formatter.FormatNewRegression(ctx, commit, previousCommit, alert, cl, n.url, frame)
	if err != nil {
		return err
	}
	return n.transport.UpdateRegressionNotification(ctx, alert, body, notificationId)
}

// New returns a Notifier of the selected type.
func New(ctx context.Context, cfg *config.NotifyConfig, URL, commitRangeURITemplate string) (Notifier, error) {
	switch cfg.Notifications {
	case notifytypes.None:
		return newNotifier(NewHTMLFormatter(commitRangeURITemplate), NewNoopTransport(), URL), nil
	case notifytypes.HTMLEmail:
		return newNotifier(NewHTMLFormatter(commitRangeURITemplate), NewEmailTransport(), URL), nil
	case notifytypes.MarkdownIssueTracker:
		tracker, err := NewIssueTrackerTransport(ctx, cfg)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		f, err := NewMarkdownFormatter(commitRangeURITemplate, cfg)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return newNotifier(f, tracker, URL), nil
	case notifytypes.ChromeperfAlerting:
		return NewChromePerfNotifier(ctx, nil)
	case notifytypes.AnomalyGrouper:
		return ag.NewAnomalyGroupNotifier(ctx, nil), nil
	default:
		return nil, skerr.Fmt("invalid Notifier type: %s, must be one of: %v", cfg.Notifications, notifytypes.AllNotifierTypes)
	}
}
