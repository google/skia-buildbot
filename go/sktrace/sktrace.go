// sktrace is an EXPERIMENTAL package with utility functions for the go.opencensus.io/trace package.
// Subject to change.
package sktrace

import (
	"net/http"

	"go.opencensus.io/exporter/jaeger"
	"go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

const (
	// Endpoint of the locally running Jaeger instance.
	LOCAL_JAEGER_ENDPOINT = "http://localhost:14268"
)

// Init initializes traceing support. If local is true, tokenSrc can be nil and
// tracing will point to a local instance of Jaeger.
// See https://jaeger.readthedocs.io/en/latest/
func Init(serviceName string, tokenSrc oauth2.TokenSource, local bool) error {
	var exporter trace.Exporter
	var err error

	// if running local we write to the local Jaeger endpoint.
	if !local {
		jaegerOpt := jaeger.Options{
			Endpoint:    LOCAL_JAEGER_ENDPOINT,
			ServiceName: serviceName,
		}
		if exporter, err = jaeger.NewExporter(jaegerOpt); err != nil {
			return sklog.FmtErrorf("Error creating Jaeger exporter: %s", err)
		}
	} else {
		sdOptions := stackdriver.Options{
			ProjectID:     common.PROJECT_ID,
			ClientOptions: []option.ClientOption{option.WithTokenSource(tokenSrc)},
		}
		if exporter, err = stackdriver.NewExporter(sdOptions); err != nil {
			return sklog.FmtErrorf("Error creating stackdriver exporter: %s", err)
		}
	}

	trace.RegisterExporter(exporter)
	trace.SetDefaultSampler(trace.AlwaysSample())
	return nil
}

// Trace wraps a given pattern and handler function and sets up tracing for it.
// A client should call it like this when they set up their route:
//
//  	router.HandleFunc(tracing.Trace("/url/path", handler)).Methods("GET")
//
// Inside the 'handler' function they can retrieve the contex via r.Context()
// where 'r' is the http.Request object and pass it around for more detailed
// tracing.
func Trace(pattern string, handler func(http.ResponseWriter, *http.Request)) (string, func(http.ResponseWriter, *http.Request)) {
	return pattern, func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx, span := trace.StartSpan(ctx, pattern)
		defer span.End()

		// If the context was created in the Context() call we need to make sure
		// it's also the context the request carries from here on.
		// Note: This makes a shallow copy of the request.
		handler(w, r.WithContext(ctx))
	}
}
