// Neither Prometheus nor AlertManager handle authentication, so use a reverse
// proxy that requires login to protect both of them.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/sklog"
)

var (
	aud               = flag.String("aud", "", "The aud value, from the Identity-Aware Proxy JWT Audience for the given backend.")
	authGroup         = flag.String("auth_group", "google/skia-staff@google.com", "The chrome infra auth group to use for restricting access.")
	local             = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port              = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort          = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
	serviceAccountJWT = flag.String("service_account_jwt", "/var/secrets/skia-public-auth/key.json", "The JWT key for the service account that has access to chrome infra auth.")
	targetPort        = flag.String("target_port", ":10116", "The port we are proxying to.")
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
	sklog.Infof("Requesting: %s As: %s", r.RequestURI, r.Header.Get(iap.EMAIL_HEADER))
	p.reverseProxy.ServeHTTP(w, r)
}

func main() {
	common.InitWithMust(
		filepath.Base(os.Args[0]),
		common.PrometheusOpt(promPort),
	)
	target, err := url.Parse(fmt.Sprintf("http://localhost%s", *targetPort))
	if err != nil {
		sklog.Fatalf("Unable to parse target URL %q: %s", targetPort, err)
	}

	var h http.Handler = NewProxy(target)
	if !*local {
		client, err := auth.NewJWTServiceAccountClient("", *serviceAccountJWT, nil, auth.SCOPE_USERINFO_EMAIL)
		if err != nil {
			sklog.Fatal(err)
		}
		allow, err := allowed.NewAllowedFromChromeInfraAuth(client, *authGroup)
		if err != nil {
			sklog.Fatal(err)
		}
		h = iap.New(h, *aud, allow)
	}
	http.Handle("/", h)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
