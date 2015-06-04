package main

import (
	"encoding/json"
	"flag"
	"fmt"
	htemplate "html/template"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/gorilla/mux"
	metrics "github.com/rcrowley/go-metrics"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/util"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *htemplate.Template = nil

	requestsCounter                  = metrics.NewRegisteredCounter("requests", metrics.DefaultRegistry)
	router          *mux.Router      = mux.NewRouter()
	client          *http.Client     = nil
	store           *storage.Service = nil
)

// Command line flags.
var (
	configFilename = flag.String("config", "fuzzer.toml", "Configuration filename")
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

func Init() {
	rand.Seed(time.Now().UnixNano())

	common.InitWithMetricsCB("fuzzer", func() string {
		common.DecodeTomlFile(*configFilename, &config.Config)
		return config.Config.FrontEnd.GraphiteServer
	})

	if config.Config.Common.ResourcePath == "" {
		_, filename, _, _ := runtime.Caller(0)
		config.Config.Common.ResourcePath = filepath.Join(filepath.Dir(filename), "../..")
	}

	path, err := filepath.Abs(config.Config.Common.ResourcePath)
	if err != nil {
		glog.Fatalf("Couldn't get absolute path to fuzzer resources: %s", err)
	}
	if err := os.Chdir(path); err != nil {
		glog.Fatal(err)
	}

	indexTemplate = htemplate.Must(htemplate.ParseFiles(
		filepath.Join(path, "templates/index.html"),
		filepath.Join(path, "templates/header.html"),
		filepath.Join(path, "templates/titlebar.html"),
		filepath.Join(path, "templates/footer.html"),
	))

	if client, err = auth.NewClient(config.Config.Common.DoOAuth, config.Config.Common.OAuthCacheFile, storage.DevstorageFull_controlScope); err != nil {
		glog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}

	if store, err = storage.New(client); err != nil {
		glog.Fatalf("Failed to create storage service client: %s", err)
	}
}

type IndexContext struct {
	LoadFuzzListURL string
}

func getURL(router *mux.Router, name string, pairs ...string) string {
	route := router.Get(name)
	if route == nil {
		glog.Fatalf("Couldn't find any route named %s", name)
	}

	routeURL, err := route.URL(pairs...)
	if err != nil {
		glog.Fatalf("Couldn't resolve route %s into a URL", routeURL)
	}

	return routeURL.String()
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Main Handler: %q\n", r.URL.Path)
	requestsCounter.Inc(1)
	if r.Method == "GET" {
		// Expand the template.
		w.Header().Set("Content-Type", "text/html")
		fuzzListURL := getURL(router, "fuzzListHandler")
		context := IndexContext{
			fuzzListURL,
		}
		if err := indexTemplate.Execute(w, context); err != nil {
			glog.Errorf("Failed to expand template: %q\n", err)
		}
	}
}

// makeResourceHandler creates a static file handler that sets a caching policy.
func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(config.Config.Common.ResourcePath))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

func getFuzzes(baseDir string) []string {
	results := []string{}
	glog.Infof("Opening bucket/directory: %s/%s", config.Config.Common.FuzzOutputGSBucket, baseDir)

	req := store.Objects.List(config.Config.Common.FuzzOutputGSBucket).Prefix(baseDir + "/").Delimiter("/")
	for req != nil {
		resp, err := req.Do()
		if err != nil {
			return results
		}
		for _, result := range resp.Prefixes {
			results = append(results, result[len(baseDir)+1:len(result)-1])
		}
		if len(resp.NextPageToken) > 0 {
			req.PageToken(resp.NextPageToken)
		} else {
			req = nil
		}
	}
	return results
}

func fuzzListHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse form data.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)

	fuzzes := []string{}

	failed := r.FormValue("failed")
	passed := r.FormValue("passed")

	if failed == "true" {
		fuzzes = append(fuzzes, getFuzzes("failed")...)
	}
	if passed == "true" {
		fuzzes = append(fuzzes, getFuzzes("working")...)
	}

	if err := enc.Encode(fuzzes); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
	}
}

func main() {
	flag.Parse()
	Init()

	// Set up login
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-ubjke2f3staq6ouas64r31h8f8tcbiqp.apps.googleusercontent.com"
	var clientSecret = "rK-kRY71CXmcg0v9I9KIgWci"
	var useRedirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", config.Config.FrontEnd.Port)
	if !config.Config.Common.Local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
		useRedirectURL = config.Config.FrontEnd.RedirectURL
	}

	login.Init(clientID, clientSecret, useRedirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, config.Config.Common.Local)

	// Set up the login related resources.
	router.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	router.HandleFunc("/loginstatus/", login.StatusHandler)
	router.HandleFunc("/logout/", login.LogoutHandler)

	router.HandleFunc("/", mainHandler)
	router.HandleFunc("/failed", mainHandler)
	router.HandleFunc("/passed", mainHandler)
	router.PathPrefix("/res/").HandlerFunc(makeResourceHandler())

	jsonRouter := router.PathPrefix("/_").Subrouter()
	jsonRouter.HandleFunc("/list", fuzzListHandler).Name("fuzzListHandler")

	rootHandler := util.LoggingGzipRequestResponse(router)
	if config.Config.FrontEnd.ForceLogin {
		rootHandler = login.ForceAuth(rootHandler, OAUTH2_CALLBACK_PATH)
	}

	http.Handle("/", rootHandler)

	glog.Fatal(http.ListenAndServe(config.Config.FrontEnd.Port, nil))
}
