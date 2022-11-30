// Package httpsource implements event.Source by accepting incoming HTTP
// requests that contain a machine.Event serialized as JSON.
package httpsource

import (
	"context"
	"encoding/json"
	"net/http"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/machine/go/machine"
	"go.skia.org/infra/machine/go/machine/event/source"
)

// HTTPSource implements event.Source and http.Handler.
type HTTPSource struct {
	outgoing            chan machine.Event
	eventReceiveSuccess metrics2.Counter
	eventReceiveFailed  metrics2.Counter
}

// New returns an instance of HTTPSource.
func New() (*HTTPSource, error) {
	return &HTTPSource{
		outgoing:            make(chan machine.Event, 1000),
		eventReceiveSuccess: metrics2.GetCounter(source.ReceiveSuccessMetricName, map[string]string{"type": "http"}),
		eventReceiveFailed:  metrics2.GetCounter(source.ReceiveFailureMetricName, map[string]string{"type": "http"}),
	}, nil
}

// ServeHTTP implements http.Handler.
//
// Must be hooked up to the frontend to handle incoming HTTP sent Events. See
// rpc.go for the URL to use.
func (h *HTTPSource) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var event machine.Event
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		h.eventReceiveFailed.Inc(1)
		httputils.ReportError(w, err, "decoding event from machine", http.StatusBadRequest)
		return
	}
	h.outgoing <- event
	h.eventReceiveSuccess.Inc(1)
}

// Start implements Source.
func (h *HTTPSource) Start(ctx context.Context) (<-chan machine.Event, error) {
	return h.outgoing, nil
}
