package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/alerts"
)

func TestEmailTransportSendNewRegression_EmailIsMissing_ReturnsError(t *testing.T) {
	e := NewEmailTransport()
	_, err := e.SendNewRegression(context.Background(), &alerts.Alert{}, "", "")
	require.Contains(t, err.Error(), "No email address")

}

func TestEmailTransportSendRegressionMissing_EmailIsMissing_ReturnsError(t *testing.T) {
	e := NewEmailTransport()
	err := e.SendRegressionMissing(context.Background(), "", &alerts.Alert{}, "", "")
	require.Contains(t, err.Error(), "No email address")

}
