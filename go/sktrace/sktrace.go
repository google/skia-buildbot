// sktrace is an EXPERIMENTAL package with utility functions for the go.opencensus.io/trace package.
// Subject to change.
package sktrace

import (
	"net/http"

	"go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/sklog"
)

// TODO(stephana): Re-add Jaeger as a local option when the Go package is stable
// again.

// TraceClient wraps the information relevant for tracing of this node.
// Use the Trace(...) function to register handlers where the service name will
// be injected in the traces.
type TraceClient struct {
	serviceName string
}

// NewTraceClient returns a new trace client instance.
func NewTraceClient(projectID, serviceName string, tokenSrc oauth2.TokenSource) (*TraceClient, error) {
	var exporter trace.Exporter
	var err error

	sdOptions := stackdriver.Options{
		ProjectID:     projectID,
		ClientOptions: []option.ClientOption{option.WithTokenSource(tokenSrc)},
	}
	if exporter, err = stackdriver.NewExporter(sdOptions); err != nil {
		return nil, sklog.FmtErrorf("Error creating stackdriver exporter: %s", err)
	}
	sklog.Infof("StackDriver trace exporter created.")

	trace.RegisterExporter(exporter)
	trace.SetDefaultSampler(trace.AlwaysSample())
	return &TraceClient{
		serviceName: serviceName,
	}, nil
}

// Trace wraps a given pattern and handler function and sets up tracing for it.
// A client should call it like this when they set up their route:
//
//  	router.HandleFunc(tracing.Trace("/url/path", handler)).Methods("GET")
//
// Inside the 'handler' function they can retrieve the contex via r.Context()
// where 'r' is the http.Request object and pass it around for more detailed
// tracing.
func (t *TraceClient) Trace(pattern string, handler func(http.ResponseWriter, *http.Request)) (string, func(http.ResponseWriter, *http.Request)) {
	return pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx, span := trace.StartSpan(ctx, pattern)
		span.SetAttributes(trace.StringAttribute("service", t.serviceName))
		defer span.End()

		// If the context was created in the Context() call we need to make sure
		// it's also the context the request carries from here on.
		// Note: This makes a shallow copy of the request.
		handler(w, r.WithContext(ctx))
	}
}
