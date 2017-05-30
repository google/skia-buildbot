package main

/*

 */

import (
	"flag"
	"net/http"
	"path"
	"path/filepath"
	"regexp"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

const ()

var (
	local    = flag.Bool("local", false, "Use when running locally as opposed to in production.")
	host     = flag.String("host", "localhost", "HTTP server")
	port     = flag.String("port", ":8000", "HTTP service port")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	workdir  = flag.String("workdir", ".", "Working directory")
)

var (
	// hostReg matches host names that look like:
	//
	//  skia-e-win-010.rc.skia.org
	//
	// and captures the bot name 'skia-e-win-010'.
	hostReg = regexp.MustCompile("^([a-zA-Z0-9-]+).rc.skia.org$")
)

// clientHandler handles requests to connect to a bot.
type clientHandler struct {
}

func newClientHandler() (*clientHandler, error) {
	return &clientHandler{}, nil
}

func (b *clientHandler) HandleConnect(w http.ResponseWriter, r *http.Request) {
	// TODO
}

func (b *clientHandler) AddHandlers(r mux.Router) {
	r.HandleFunc("/connect/{bot}", b.HandleConnect)
	r.HandleFunc("/vncws", b.HandleConnect)
}

func runServer(serverURL string, r mux.Router) {
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"remote-control",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	// TODO
	wdAbs, err := filepath.Abs(*workdir)
	if err != nil {
		sklog.Fatal(err)
	}

	// GCS authenticated HTTP client.
	oauthCacheFile := path.Join(wdAbs, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, auth.SCOPE_READ_WRITE)
	if err != nil {
		sklog.Fatal(err)
	}

	clientHandler, err := newClientHandler()
	if err != nil {
		sklog.Fatal(err)
	}

	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}
	r := mux.NewRouter()
	clientHandler.AddHandlers(r)
	runServer(serverURL, r)
}
