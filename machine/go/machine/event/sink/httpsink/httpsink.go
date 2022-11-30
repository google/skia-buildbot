// Package httpsink sends event to the machine server via HTTP.
package httpsink

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/event/sink"
	"go.skia.org/infra/machine/go/machineserver/rpc"
)

// HTTPSink implements event.Sink.
//
// All the HTTP requests to the server include an Authorization: Bearer token
// header that is populated from Default Application Credentials.
type HTTPSink struct {
	targetURL        string
	client           *http.Client
	eventSentSuccess metrics2.Counter
	eventSentFailure metrics2.Counter
}

// NewFromClient returns an instance of HTTPSink.
//
// serverURL is the scheme, host, and optionally the port of the server to send
// requests to as a string, for example: "https://machines.skia.org".
//
// The client must be configured to make all the HTTP requests to the server
// include an Authorization: Bearer token header, which is authorized as an
// Editor of machineserver.
func NewFromClient(client *http.Client, serverURL string) (*HTTPSink, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, skerr.Wrapf(err, "parsing serverURL")
	}
	u.Path = rpc.MachineEventURL

	return &HTTPSink{
		client:           client,
		targetURL:        u.String(),
		eventSentSuccess: metrics2.GetCounter(sink.SendSuccessMetricName, map[string]string{"type": "http"}),
		eventSentFailure: metrics2.GetCounter(sink.SendFailureMetricName, map[string]string{"type": "http"}),
	}, nil

}

// Send implements event.Sink.
func (h *HTTPSink) Send(ctx context.Context, event machine.Event) error {
	b, err := json.Marshal(event)
	if err != nil {
		h.eventSentFailure.Inc(1)
		return skerr.Wrapf(err, "serializing event")
	}
	body := bytes.NewReader(b)
	resp, err := h.client.Post(h.targetURL, "application/json", body)
	if err != nil {
		h.eventSentFailure.Inc(1)
		return skerr.Wrapf(err, "sending event")
	}
	if resp.StatusCode != http.StatusOK {
		h.eventSentFailure.Inc(1)
		return fmt.Errorf("sending event to %q: non-200 status code %q", h.targetURL, resp.Status)
	}
	h.eventSentSuccess.Inc(1)
	return nil
}
