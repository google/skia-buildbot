// Prometheus doesn't handle authentication, so use a reverse
// proxy that requires login to protect it.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
)

var (
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port       = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	targetPort = flag.String("target_port", ":9000", "The port we are proxying to.")
)

type Proxy struct {
	reverseProxy http.Handler
}

func NewProxy(target *url.URL) *Proxy {
	return &Proxy{
		reverseProxy: httputil.NewSingleHostReverseProxy(target),
	}
}

func (p Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Requesting: %s", r.RequestURI)
	if login.LoggedInAs(r) == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusSeeOther)
		return
	}
	if !login.IsViewer(r) {
		http.Error(w, "403 Forbidden", http.StatusForbidden)
		return
	}
	p.reverseProxy.ServeHTTP(w, r)
}

func main() {
	common.InitWithMust(
		"auth-proxy",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)
	login.InitWithAllow(*port, *local, nil, nil, allowed.NewAllowedFromList([]string{"google.com"}))
	target, err := url.Parse(fmt.Sprintf("http://localhost%s", *targetPort))
	if err != nil {
		sklog.Fatalf("Unable to parse target URL %q: %s", targetPort, err)
	}

	var h http.Handler = NewProxy(target)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
