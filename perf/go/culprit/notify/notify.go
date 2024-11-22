package notify

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
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
	NotifyAnomaliesFound(ctx context.Context, anomalies []*pb.Anomaly, subscription *sub_pb.Subscription) (string, error)
}

// DefaultCulpritNotifier sends notifications.
type DefaultCulpritNotifier struct {
	formatter formatter.Formatter
	transport transport.Transport
}

// newNotifier returns a newNotifier Notifier.
func GetDefaultNotifier(ctx context.Context, cfg *config.InstanceConfig, commitURLTemplate string) (CulpritNotifier, error) {
	switch cfg.CulpritNotifyConfig.NotificationType {
	case types.NoneNotify:
		return &DefaultCulpritNotifier{
			formatter: formatter.NewNoopFormatter(),
			transport: transport.NewNoopTransport(),
		}, nil
	case types.IssueNotify:
		transport, err := transport.NewIssueTrackerTransport(ctx, &cfg.CulpritNotifyConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		formatter, err := formatter.NewMarkdownFormatter(commitURLTemplate, &cfg.CulpritNotifyConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return &DefaultCulpritNotifier{
			formatter: formatter,
			transport: transport,
		}, nil
	default:
		return nil, skerr.Fmt("Unsupported Notifier type: %s", cfg.CulpritNotifyConfig.NotificationType)
	}
}

// Creates a bug in Buganizer about the detected culprit.
func (n *DefaultCulpritNotifier) NotifyCulpritFound(ctx context.Context, culprit *pb.Culprit, subscription *sub_pb.Subscription) (string, error) {
	subject, body, err := n.formatter.GetSubjectAndBody(ctx, culprit, subscription)
	if err != nil {
		return "", err
	}
	bugId, err := n.transport.SendNewCulprit(ctx, subscription, subject, body)
	if err != nil {
		return "", skerr.Wrapf(err, "sending new culprit message")
	}
	return bugId, nil
}

// Creates a bug in Buganizer about the detected anomalies.
func (n *DefaultCulpritNotifier) NotifyAnomaliesFound(ctx context.Context, anomalies []*pb.Anomaly, subscription *sub_pb.Subscription) (string, error) {
	// TODO(wenbinzhang): implement the NotifyAnomaliesFound after the notifier is updated to support multiple purposes.
	sklog.Debugf("NotifyAnomaliesFound not yet implemented: %s", subscription.Name)
	return "nil", nil
}
