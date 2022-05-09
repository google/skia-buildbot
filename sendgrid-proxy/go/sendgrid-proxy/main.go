// A reverse proxy for SendGrid that attaches the SendGrid API Key to every
// request.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/secret"
	"go.skia.org/infra/go/sklog"
)

const (
	secretName = "sendgrid-proxy"

	requestsMetricName = "sendgrid_proxy_requests"
	errorsMetricName   = "sendgrid_proxy_errors"
)

var (
	port      = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	project   = flag.String("project", "skia-public", "The GCP project that contains the API Key in GCP Secret manager.")
	promPort  = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	targetURL = flag.String("target_url", "https://api.sendgrid.com", "The URL we are proxying to.")
)

func newProxy(target *url.URL, sendGridAPIKey string) *httputil.ReverseProxy {
	numRequests := metrics2.GetCounter(requestsMetricName)
	numErrors := metrics2.GetCounter(errorsMetricName)
	authHeaderValue := []string{fmt.Sprintf("Bearer: %s", sendGridAPIKey)}

	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	reverseProxy.Director = func(r *http.Request) {
		r.Header["Authorization"] = authHeaderValue
		numRequests.Inc(1)
	}
	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		numErrors.Inc(1)
		httputils.ReportError(w, err, "failed to proxy request", http.StatusInternalServerError)
	}

	return reverseProxy
}

func main() {
	common.InitWithMust(
		"sendmail-proxy",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	ctx := context.Background()
	secretClient, err := secret.NewClient(context.Background())
	if err != nil {
		sklog.Fatal(err)
	}
	sendGridAPIKey, err := secretClient.Get(ctx, *project, secretName, secret.VersionLatest)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("SendGrid API Key retrieved.")

	u, err := url.Parse(*targetURL)
	if err != nil {
		sklog.Fatal(err)
	}

	var h http.Handler = newProxy(u, sendGridAPIKey)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)

	sklog.Info("Start listening.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
