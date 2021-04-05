// Prometheus doesn't handle authentication, so use a reverse
// proxy that requires login to protect it.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/auth-proxy/auth"
)

var (
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port        = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	targetPort  = flag.String("target_port", ":9000", "The port we are proxying to.")
	allowPost   = flag.Bool("allow_post", false, "Allow POST requests to bypass auth.")
	allowedFrom = flag.String("allowed_from", "google.com", "A comma separated list of of domains and email addresses that are allowed to access the site.")
	passive     = flag.Bool("passive", false, "If true then allow unauthenticated requests to go through, while still adding logged in users emails in via the webAuthHeaderName.")
)

// Send the logged in user email in the following header. This allows decoupling
// of authentication from the core of the app. See
// https://grafana.com/blog/2015/12/07/grafana-authproxy-have-it-your-way/ for
// how Grafana uses this to support almost any authentication handler.
const webAuthHeaderName = "X-WEBAUTH-USER"

type proxy struct {
	reverseProxy http.Handler
	authProvider auth.Auth
}

func newProxy(target *url.URL, authProvider auth.Auth) *proxy {
	return &proxy{
		reverseProxy: httputil.NewSingleHostReverseProxy(target),
		authProvider: authProvider,
	}
}

func (p proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Requesting: %s", r.RequestURI)
	email := p.authProvider.LoggedInAs(r)
	r.Header.Del(webAuthHeaderName)
	r.Header.Add(webAuthHeaderName, email)
	if r.Method == "POST" && *allowPost {
		p.reverseProxy.ServeHTTP(w, r)
		return
	}
	if !*passive {
		if email == "" {
			http.Redirect(w, r, p.authProvider.LoginURL(w, r), http.StatusSeeOther)
			return
		}
		if !p.authProvider.IsViewer(r) {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
	}
	p.reverseProxy.ServeHTTP(w, r)
}

func main() {
	common.InitWithMust(
		"auth-proxy",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	allowedList := strings.Split(*allowedFrom, ",")
	authInstance := auth.New()
	authInstance.SimpleInitWithAllow(*port, *local, nil, nil, allowed.NewAllowedFromList(allowedList))
	targetURL := fmt.Sprintf("http://localhost%s", *targetPort)
	target, err := url.Parse(targetURL)
	if err != nil {
		sklog.Fatalf("Unable to parse target URL %s: %s", targetURL, err)
	}

	var h http.Handler = newProxy(target, authInstance)
	h = httputils.HealthzAndHTTPS(h)
	http.Handle("/", h)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
