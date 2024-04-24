package formatter

import (
	"context"

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

// SendNewCulprit implements Formatter.
func (f *NoopFormatter) GetSubjectAndBody(ctx context.Context, culprit *pb.Culprit,
	subscription *sub_pb.Subscription) (string, string, error) {
	return "", "", nil
}
