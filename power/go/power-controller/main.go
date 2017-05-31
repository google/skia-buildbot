package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/promalertsclient"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/power/go/decider"
	"go.skia.org/infra/power/go/gatherer"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	downBots gatherer.Gatherer = nil

	authedClient *http.Client = nil
)

var (
	// web server params
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	alertsEndpoint = flag.String("alerts_endpoint", "skia-prom:8001", "The Prometheus GCE name and port")

	// OAUTH params
	authWhiteList = flag.String("auth_whitelist", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://power.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")

	powercycleConfig = flag.String("powercycle_config", "/etc/powercycle.yaml", "YAML file with powercycle bot/device configuration. Same as used for powercycle.")
	updatePeriod     = flag.Duration("update_period", time.Minute, "How often to update the list of down bots.")
)

func main() {
	flag.Parse()
	defer common.LogPanic()

	if *local {
		common.InitWithMust(
			"power-controller",
			common.PrometheusOpt(promPort),
		)
	} else {
		common.InitWithMust(
			"power-controller",
			common.PrometheusOpt(promPort),
			common.CloudLoggingOpt(),
		)
	}

	loadTemplates()

	if err := setupAuthentication(); err != nil {
		sklog.Fatalf("Could not setup authentication: %s", err)
	}

	es, err := skswarming.NewApiClient(authedClient, "chromium-swarm.appspot.com")
	if err != nil {
		sklog.Fatalln(err)
	}
	is, err := skswarming.NewApiClient(authedClient, "chrome-swarming.appspot.com")
	if err != nil {
		sklog.Fatalln(err)
	}
	ac := promalertsclient.New(&http.Client{}, *alertsEndpoint)
	d, err := decider.New(*powercycleConfig)
	if err != nil {
		sklog.Fatalln(err)
	}

	downBots = gatherer.New(es, is, ac, d)

	go func() {
		downBots.Update()
		for {
			<-time.Tick(*updatePeriod)
			downBots.Update()
		}
	}()

	runServer() // runs forever
}

func loadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

// indexHandler displays the index page, which has no real templating. The client side JS will
// query for more information.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		loadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	if err := indexTemplate.Execute(w, nil); err != nil {
		sklog.Errorf("Failed to expand template: %v", err)
	}
}

type downBotsResponse struct {
	List []gatherer.DownBot `json:"list"`
}

func downBotsHandler(w http.ResponseWriter, r *http.Request) {
	if downBots == nil {
		http.Error(w, "The power-controller isn't finished booting up.  Try again later", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	response := downBotsResponse{List: downBots.CachedDownBots()}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

func setupAuthentication() error {
	oauthCacheFile := path.Join(*resourcesDir, "google_storage_token.data")
	var err error
	authedClient, err = auth.NewClient(*local, oauthCacheFile, skswarming.AUTH_SCOPE)
	if err != nil {
		return fmt.Errorf("Could not set up autheticated HTTP client: %s", err)
	}

	useRedirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		useRedirectURL = *redirectURL
	}
	if err := login.Init(useRedirectURL, *authWhiteList); err != nil {
		return fmt.Errorf("Problem setting up server OAuth: %s", err)
	}
	return nil
}

func runServer() {
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/down_bots", downBotsHandler)

	rootHandler := httputils.LoggingGzipRequestResponse(r)

	http.Handle("/", rootHandler)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
