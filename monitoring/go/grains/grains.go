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
	"strings"

	"skia.googlesource.com/buildbot.git/go/login"
	"skia.googlesource.com/buildbot.git/go/metadata"
	"skia.googlesource.com/buildbot.git/perf/go/flags"

	"github.com/golang/glog"
)

const (
	COOKIESALT_METADATA_KEY        = "cookiesalt"
	INFLUXDB_NAME_METADATA_KEY     = "influxdb_name"
	INFLUXDB_PASSWORD_METADATA_KEY = "influxdb_password"
	CLIENT_ID_METADATA_KEY         = "client_id"
	CLIENT_SECRET_METADATA_KEY     = "client_secret"
)

var (
	port             = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	useMetadata      = flag.Bool("useMetadata", true, "Load sensitive values from metadata not from flags.")
	influxDbApiPort  = flag.Int("influxdb_api_port", 8086, "The local port of the InfluxDB API.")
	grafanaDir       = flag.String("grafana_dir", "", "The directory of the grafana files.")
	influxDbName     = flag.String("influxdb_name", "admin", "The InfluxDB username.")
	influxDbPassword = flag.String("influxdb_password", "admin", "The InfluxDB password.")
	cookieSalt       = flag.String("cookiesalt", "notverysecret", "Entropy for securing cookies.")
	clientID         = flag.String("client_id", "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com", "OAuth 2.0 Client ID")
	clientSecret     = flag.String("client_secret", "cw0IosPu4yjaG2KWmppj2guj", "OAuth 2.0 Client Secret")
	redirectURL      = flag.String("redirect_url", "http://localhost:8000/oauth2callback/", "URL to use for OAuth 2.0 redirects.")
)

type Proxy struct {
	InfluxDB http.Handler
	Grafana  http.Handler
}

func NewProxy(influxDbApiPort int, grafanaDir string) *Proxy {
	u, err := url.Parse(fmt.Sprintf("http://localhost:%d", influxDbApiPort))
	if err != nil {
		glog.Fatalf("Failed to parse redirect URL: %s", err)
	}
	return &Proxy{
		InfluxDB: httputil.NewSingleHostReverseProxy(u),
		Grafana:  http.FileServer(http.Dir(grafanaDir)),
	}
}

func (p Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Requesting: %s", r.RequestURI)
	if login.LoggedInAs(r) == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusSeeOther)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/db/") {
		glog.Infof("Forwarding to InfluxDB")
		values := r.URL.Query()
		values.Set("u", *influxDbName)
		values.Set("p", *influxDbPassword)
		r.URL.RawQuery = values.Encode()
		p.InfluxDB.ServeHTTP(w, r)
	} else {
		glog.Infof("Serving static files.")
		p.Grafana.ServeHTTP(w, r)
	}
}

func main() {
	flag.Parse()
	flags.Log()
	if *useMetadata {
		*clientID = metadata.MustGet(CLIENT_ID_METADATA_KEY)
		*clientSecret = metadata.MustGet(CLIENT_SECRET_METADATA_KEY)
		*cookieSalt = metadata.MustGet(COOKIESALT_METADATA_KEY)
		*influxDbName = metadata.MustGet(INFLUXDB_NAME_METADATA_KEY)
		*influxDbPassword = metadata.MustGet(INFLUXDB_PASSWORD_METADATA_KEY)
	}
	login.Init(*clientID, *clientSecret, *redirectURL, *cookieSalt)
	http.Handle("/", NewProxy(*influxDbApiPort, *grafanaDir))
	http.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	glog.Fatal(http.ListenAndServe(*port, nil))
}
