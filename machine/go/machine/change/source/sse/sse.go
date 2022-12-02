package sse

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/r3labs/sse/v2"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/machine/go/machine/change/source"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	"golang.org/x/oauth2/google"
)

// SSE implements Source.
type SSE struct {
	ch chan interface{}
}

// New returns a new *SEE, which implements Source.
//
// serverURL is the scheme, host, and optionally the port of the server to send
// requests to as a string, for example: "https://machines.skia.org".
func New(ctx context.Context, serverURL string, machineID string) (*SSE, error) {
	// Get an authorized client.
	client, err := google.DefaultClient(ctx, "email")
	if err != nil {
		return nil, skerr.Wrapf(err, "getting authorized http client")
	}

	return NewFromClient(ctx, client, serverURL, machineID)
}

// NewFromClient returns a new *SEE, which implements Source.
func NewFromClient(ctx context.Context, client *http.Client, serverURL string, machineID string) (*SSE, error) {
	// Construct URL for subscribing to Server-Sent Events.
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing serverURL")
	}
	u.Path = rpc.SSEMachineDescriptionUpdatedURL

	// Create SSE client.
	sseClient := sse.NewClient(u.String())
	sseClient.Connection = client
	sseClient.ReconnectStrategy = backoff.NewConstantBackOff(time.Second)

	receive := metrics2.GetCounter(source.MetricName, map[string]string{"type": "http"})

	ch := make(chan interface{})

	// Subscribe to all events for this machine.
	go func() {
		for {
			// SubscribeWithContext should not return except on error.
			err := sseClient.SubscribeWithContext(ctx, machineID, func(msg *sse.Event) {
				receive.Inc(1)
				ch <- nil
			})
			if err != nil {
				sklog.Errorf("sse connect: %s", err)
			}
		}
	}()

	return &SSE{
		ch: ch,
	}, nil

}

// Start implements Source.
func (s *SSE) Start(ctx context.Context) <-chan interface{} {
	return s.ch
}
