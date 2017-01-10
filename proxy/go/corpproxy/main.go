package main

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
)

var (
	local    = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
)

var (
	// hostReg matches host names that look like:
	//
	//  skia-perf-10110-proxy.skia.org
	//
	// and captures the internal host name 'skia-perf', and the port to connect
	// to '10110'.
	hostReg = regexp.MustCompile("^([a-zA-Z0-9-]+)-([0-9]+)-proxy.skia.org$")
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	sklog.Infof("Requesting: %s", r.RequestURI)
	if login.LoggedInAs(r) == "" {
		http.Redirect(w, r, login.LoginURL(w, r), http.StatusSeeOther)
		return
	}
	if !login.IsAdmin(r) {
		sklog.Info("User is not an admin.")
		http.NotFound(w, r)
		return
	}
	parts := hostReg.FindAllStringSubmatch(r.Host, -1)
	if len(parts) != 1 || len(parts[0]) != 3 {
		sklog.Infof("Failed to parse r.Host: %q", r.Host)
		http.NotFound(w, r)
		return
	}
	rawTarget := fmt.Sprintf("http://%s:%s", parts[0][1], parts[0][2])
	sklog.Infof("Proxying to: %q", rawTarget)
	target, err := url.Parse(rawTarget)
	if err != nil {
		http.NotFound(w, r)
		sklog.Warningf("Failed to parse %q: %s", rawTarget, err)
		return
	}
	// TODO(jcgregorio) Maybe cache these is they are slow to build, or if they
	// cache open connections.
	reverseProxy := httputil.NewSingleHostReverseProxy(target)
	reverseProxy.ServeHTTP(w, r)
}

func main() {
	defer common.LogPanic()
	common.Init()
	metrics2.InitPrometheus(*promPort)
	common.StartCloudLogging(filepath.Base(os.Args[0]))

	redirectURL := "https://auth-proxy.skia.org/oauth2callback/"
	if *local {
		redirectURL = fmt.Sprintf("https://localhost%s/oauth2callback/", *port)
	}
	if err := login.Init(redirectURL, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST); err != nil {
		sklog.Fatalf("Failed to initialize the login system: %s", err)
	}

	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
