package transport

import (
	"context"

	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

// NoopTransport implements Transport by doing nothing.
type NoopTransport struct {
}

// NewNoopTransport returns a new EmailService instance.
func NewNoopTransport() *NoopTransport {
	return &NoopTransport{}
}

// SendNewCulprit implements Transport.
func (t *NoopTransport) SendNewCulprit(ctx context.Context, subscription *sub_pb.Subscription, subject string, body string) (string, error) {
	return "", nil
}
