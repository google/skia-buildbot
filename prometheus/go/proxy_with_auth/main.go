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

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/sklog"
)

var (
	local      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port       = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort   = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
	targetPort = flag.String("target_port", ":10116", "The port we are proxying to.")
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
	p.reverseProxy.ServeHTTP(w, r)
}

func main() {
	common.InitWithMust(
		filepath.Base(os.Args[0]),
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	login.SimpleInitMust(*port, *local)

	targetURL := fmt.Sprintf("http://localhost%s", *targetPort)
	target, err := url.Parse(targetURL)
	if err != nil {
		sklog.Fatalf("Unable to parse target URL %s: %s", targetURL, err)
	}

	http.Handle("/", NewProxy(target))
	http.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
