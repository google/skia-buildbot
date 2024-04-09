package notify

import (
	"context"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/culprit/formatter"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	"go.skia.org/infra/perf/go/culprit/transport"
	"go.skia.org/infra/perf/go/notifytypes"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

type CulpritNotifier interface {
	// Sends out notification to users about the detected culprit.
	NotifyCulpritFound(ctx context.Context, culprit *pb.Culprit, subscription *sub_pb.Subscription) (string, error)
}

// DefaultCulpritNotifier sends notifications.
type DefaultCulpritNotifier struct {
	formatter formatter.Formatter
	transport transport.Transport
}

// newNotifier returns a newNotifier Notifier.
func GetDefaultNotifier(ctx context.Context, cfg *config.InstanceConfig, commitURLTemplate string) (CulpritNotifier, error) {
	switch cfg.NotifyConfig.Notifications {
	case notifytypes.MarkdownIssueTracker:
		transport, err := transport.NewIssueTrackerTransport(ctx, &cfg.NotifyConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		formatter, err := formatter.NewMarkdownFormatter(commitURLTemplate, &cfg.NotifyConfig)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		return &DefaultCulpritNotifier{
			formatter: formatter,
			transport: transport,
		}, nil
	default:
		return nil, skerr.Fmt("invalid Notifier type: %s, must be of type MarkdownIssueTracker", cfg.NotifyConfig.Notifications)
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
