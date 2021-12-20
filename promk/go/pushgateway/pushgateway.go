package pushgateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
)

const (
	DefaultPushgatewayURL = "https://pushgateway.skia.org"
)

// Pushgateway is an object used for interacting with the prom pushgateway.
type Pushgateway struct {
	client    *http.Client
	targetURL string
}

// New returns an instantiated Pushgateway. If url is not specified then
// DefaultPushgatewayURL is used.
func New(client *http.Client, jobName, url string) *Pushgateway {
	if url == "" {
		url = DefaultPushgatewayURL
	}
	return &Pushgateway{
		client:    client,
		targetURL: fmt.Sprintf("%s/metrics/job/%s", url, jobName),
	}
}

// Push pushes the specified metric name and value to the pushgateway.
func (p *Pushgateway) Push(ctx context.Context, metricName, metricValue string) error {
	metricText := fmt.Sprintf("%s %s\n", metricName, metricValue)
	if _, err := httputils.PostWithContext(ctx, p.client, p.targetURL, "text/plain", strings.NewReader(metricText)); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
