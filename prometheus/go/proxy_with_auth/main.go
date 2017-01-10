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
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

var (
	port       = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort   = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
	targetPort = flag.String("target_port", ":10116", "The port we are proxying to.")
	domain     = flag.String("domain", "prom.skia.org", "What is the domain name we are serving.")
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
	defer common.LogPanic()
	common.Init()
	metrics2.InitPrometheus(*promPort)

	common.StartCloudLogging(filepath.Base(os.Args[0]))

	target, err := url.Parse(fmt.Sprintf("http://localhost%s", *targetPort))
	if err != nil {
		sklog.Fatalf("Unable to parse target URL %q: %s", targetPort, err)
	}

	redirectURL := fmt.Sprintf("https://%s/oauth2callback/", *domain)
	if err := login.Init(redirectURL, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	http.Handle("/", NewProxy(target))
	http.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
