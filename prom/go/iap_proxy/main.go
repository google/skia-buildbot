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
	"go.skia.org/infra/go/iap"
	"go.skia.org/infra/go/sklog"
)

var (
	local            = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port             = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort         = flag.String("prom_port", ":10110", "Metrics service address (e.g., ':10110')")
	targetPort       = flag.String("target_port", ":10116", "The port we are proxying to.")
	projectNumber    = flag.String("project_number", "", "The number id of the GCE project.")
	backendServiceId = flag.String("backend_service_id", "", "The id of the GCE backend service.")
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
	sklog.Infof("Requesting: %s As: %s", r.RequestURI, r.Header.Get("x-user-email"))
	p.reverseProxy.ServeHTTP(w, r)
}

func main() {
	defer common.LogPanic()
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
		var err error
		h, err = iap.New([]string{}, *projectNumber, *backendServiceId, h)
		if err != nil {
			sklog.Fatalf("Failed to init iap: %s", err)
		}
	}
	http.Handle("/", h)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
