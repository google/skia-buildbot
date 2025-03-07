package notify

import (
	"context"
	"fmt"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	ag "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/culprit/formatter"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/culprit/transport"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
	"go.skia.org/infra/perf/go/types"
)

// TODO(wenbinzhang): considering using specific type for issue ID instead of 'string'.
type CulpritNotifier interface {
	// Sends out notification to users about the detected culprit.
	NotifyCulpritFound(ctx context.Context, culprit *pb.Culprit, subscription *sub_pb.Subscription) (string, error)

	// Sends out notification to users about the detected anomalies.
	NotifyAnomaliesFound(ctx context.Context, anomalyGroup *ag.AnomalyGroup, subscription *sub_pb.Subscription, anomalyList []*pb.Anomaly) (string, error)
}

// DefaultCulpritNotifier sends notifications.
type DefaultCulpritNotifier struct {
	formatter formatter.Formatter
	transport transport.Transport
}

// newNotifier returns a newNotifier Notifier.
func GetDefaultNotifier(ctx context.Context, cfg *config.InstanceConfig, commitURLTemplate string) (CulpritNotifier, error) {
	switch cfg.IssueTrackerConfig.NotificationType {
	case types.NoneNotify:
		return &DefaultCulpritNotifier{
			formatter: formatter.NewNoopFormatter(),
			transport: transport.NewNoopTransport(),
		}, nil
	case types.IssueNotify:
		transport, err := transport.NewIssueTrackerTransport(ctx, &cfg.IssueTrackerConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		formatter, err := formatter.NewMarkdownFormatter(commitURLTemplate, &cfg.IssueTrackerConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return &DefaultCulpritNotifier{
			formatter: formatter,
			transport: transport,
		}, nil
	default:
		return nil, skerr.Fmt("Unsupported Notifier type: %s", cfg.IssueTrackerConfig.NotificationType)
	}
}

// Creates a bug in Buganizer about the detected culprit.
func (n *DefaultCulpritNotifier) NotifyCulpritFound(ctx context.Context, culprit *pb.Culprit, subscription *sub_pb.Subscription) (string, error) {
	if subscription == nil || culprit == nil {
		sklog.Debugf("No subscription or no culprit.")
		return "nil", nil
	}
	sklog.Debugf("Culprit found for [%s]: %s", subscription.Name, culprit.Commit.Revision)
	subject, body, err := n.formatter.GetCulpritSubjectAndBody(ctx, culprit, subscription)
	if err != nil {
		return "", err
	}
	bugId, err := n.transport.SendNewNotification(ctx, subscription, subject, body)
	if err != nil {
		return "", skerr.Wrapf(err, "sending new culprit message")
	}
	return bugId, nil
}

// Creates a bug in Buganizer about the detected anomalies.
func (n *DefaultCulpritNotifier) NotifyAnomaliesFound(ctx context.Context, anomalyGroup *ag.AnomalyGroup, subscription *sub_pb.Subscription, anomalyList []*pb.Anomaly) (string, error) {
	if subscription == nil || anomalyGroup == nil {
		return "nil", nil
	}
	sklog.Debugf("Anomalies found for [%s]: %s", subscription.Name, anomalyGroup.AnomalyIds)

	// TODO(wenbinzhang): Generate subject and body from template.
	body := fmt.Sprintf("Mocked reporting anomalies: %s", anomalyGroup.AnomalyIds)
	subject := fmt.Sprintf("Mocked Bug Title for [%s]", subscription.Name)

	bugId, err := n.transport.SendNewNotification(ctx, subscription, subject, body)
	if err != nil {
		return "", skerr.Wrapf(err, "sending new anomaly group message")
	}
	return bugId, nil
}
