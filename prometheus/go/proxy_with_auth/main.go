// grains is the Grafana/InfluxDB Proxy Server.
//
// InfluxDB and Grafana are not very secure, Grafana is JS only and talks to
// InfluxDB which accepts name and password as query parameters.  Grafana has a
// config.js file where you list the InfluxDB databases along with the name and
// password to access them!
//
// This is obviously unacceptable from a security standpoint, so we need a
// solution that uses both InfluxDB and Grafana, protects access to them, and
// obviates the need to put name and passwords to InfluxDB into the config.js.
//
// The solution is to create a thin proxy application, grains, that talks to
// the outside world and then proxies the requests to the local InfluxDB
// instance and when proxying adds in the InfluxDB name and password to all the
// requests. The ports for the InfluxDB Admin and API endpoints should be
// blocked from outside access.
//
// This app does the following:
//  * All requests must be via logged in user (OAuth 2.0-ish, see login.go).
//  * Adds in the name/password pairs to all requests to InfluxDB.
//  * Name/password pairs passed in via cmd line flags or via metadata.
//  * The Graphana config.js served throught this app doesn't specify passwords.
//  * Listens on one port, requests under the /db/... path are directed to
//     the local InfluxDB API port, all others go to serving Grafana static files.
//  * Since Grafana and InfluxDB are served on the same port there's no CORS issues.
//  * Presumes the same password for all InfluxDB databases.
//
package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

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
	InfluxDB http.Handler
}

func NewProxy(target *url.URL) *Proxy {
	return &Proxy{
		InfluxDB: httputil.NewSingleHostReverseProxy(target),
	}
}

func (p Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Requesting: %s", r.RequestURI)
	if login.LoggedInAs(r) == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusSeeOther)
		return
	}
	p.InfluxDB.ServeHTTP(w, r)
}

func main() {
	defer common.LogPanic()
	common.Init()
	metrics2.InitPrometheus(*promPort)

	target := fmt.Sprintf("http://localhost%s", *targetPort)
	u, err := url.Parse(target)
	if err != nil {
		sklog.Fatalf("Unable to parse target URL %q: %s", target, err)
	}

	redirectURL := fmt.Sprintf("https://%s/oauth2callback/", *domain)
	if err := login.Init(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	http.Handle("/", NewProxy(u))
	http.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
