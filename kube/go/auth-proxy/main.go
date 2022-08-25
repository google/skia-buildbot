// auth-proxy is a reverse proxy that runs in front of applications and takes
// care of authentication.
//
// This is useful for applications like Promentheus that doesn't handle
// authentication itself, so we can run it behind auth-proxy to restrict access.
//
// The auth-proxy application also adds the X-WEBAUTH-USER header to each
// authenticated request and gives it the value of the logged in users email
// address, which can be used for audit logging. The application running behind
// auth-proxy should then use:
//
//     https://pkg.go.dev/go.skia.org/infra/go/alogin/proxylogin
//
// When using --cria_group this application should be run using work-load
// identity with a service account that as read access to CRIA, such as:
//
//     skia-auth-proxy-cria-reader@skia-public.iam.gserviceaccount.com
//
// See also:
//
//     https://chrome-infra-auth.appspot.com/auth/groups/project-skia-auth-service-access
//
//     https://grafana.com/blog/2015/12/07/grafana-authproxy-have-it-your-way/
package main

import (
	"context"
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
	"golang.org/x/oauth2/google"
)

var (
	criaGroup   = flag.String("cria_group", "", "The chrome infra auth group to use for restricting access. Example: 'google/skia-staff@google.com'")
	local       = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port        = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort    = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	targetPort  = flag.String("target_port", ":9000", "The port we are proxying to.")
	allowPost   = flag.Bool("allow_post", false, "Allow POST requests to bypass auth.")
	allowedFrom = flag.String("allowed_from", "", "A comma separated list of of domains and email addresses that are allowed to access the site. Example: 'google.com'")
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

func validateFlags() error {
	if *criaGroup != "" && *allowedFrom != "" {
		return fmt.Errorf("Only one of the flags in [--auth_group, --allowed_from] can be specified.")
	}
	if *criaGroup == "" && *allowedFrom == "" {
		return fmt.Errorf("At least one of the flags in [--auth_group, --allowed_from] must be specified.")
	}

	return nil
}

func main() {
	common.InitWithMust(
		"auth-proxy",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	if err := validateFlags(); err != nil {
		sklog.Fatal(err)
	}

	var allow allowed.Allow
	if *criaGroup != "" {
		ctx := context.Background()
		ts, err := google.DefaultTokenSource(ctx, "email")
		if err != nil {
			sklog.Fatal(err)
		}
		criaClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
		allow, err = allowed.NewAllowedFromChromeInfraAuth(criaClient, *criaGroup)
		if err != nil {
			sklog.Fatal(err)
		}
	} else {
		allow = allowed.NewAllowedFromList(strings.Split(*allowedFrom, ","))
	}

	authInstance := auth.New()
	authInstance.SimpleInitWithAllow(*port, *local, nil, nil, allow)
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
