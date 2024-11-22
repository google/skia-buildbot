package notify

import (
	"context"

	"go.skia.org/infra/perf/go/alerts"
)

// NoopTransport implements Transport by doing nothing.
type NoopTransport struct {
}

// NewNoopTransport returns a new EmailService instance.
func NewNoopTransport() NoopTransport {
	return NoopTransport{}
}

// SendNewRegression implements Transport.
func (e NoopTransport) SendNewRegression(ctx context.Context, alert *alerts.Alert, body, subject string) (string, error) {
	return "", nil
}

// SendRegressionMissing implements Transport.
func (e NoopTransport) SendRegressionMissing(ctx context.Context, threadingReference string, alert *alerts.Alert, body, subject string) error {
	return nil
}

// UpdateRegressionNotification implements Transport.
func (e NoopTransport) UpdateRegressionNotification(ctx context.Context, alert *alerts.Alert, body, notificationId string) error {
	return nil
}
