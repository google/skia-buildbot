package formatter

import (
	"context"

	ag "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

// NoopFormatter implements Formatter by doing nothing.
type NoopFormatter struct {
}

// NewNoopFormatter returns a new EmailService instance.
func NewNoopFormatter() *NoopFormatter {
	return &NoopFormatter{}
}

// GetCulpritSubjectAndBody implements Formatter.
func (f *NoopFormatter) GetCulpritSubjectAndBody(ctx context.Context, culprit *pb.Culprit,
	subscription *sub_pb.Subscription) (string, string, error) {
	return "", "", nil
}

// GetReportSubjectAndBody implements Formatter.
func (f *NoopFormatter) GetReportSubjectAndBody(ctx context.Context, anomalyGroup *ag.AnomalyGroup,
	subscription *sub_pb.Subscription, anomalyList []*pb.Anomaly) (string, string, error) {
	return "", "", nil
}
