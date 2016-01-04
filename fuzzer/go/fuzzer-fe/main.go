package main

/*
Runs the frontend portion of the fuzzer.  This primarily is the webserver (see DESIGN.md)
*/

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	fcommon "go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/frontend"
	"go.skia.org/infra/fuzzer/go/functionnamefinder"
	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/fuzzer/go/fuzzcache"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gs"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
	"golang.org/x/net/context"
	"google.golang.org/cloud"
	"google.golang.org/cloud/storage"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil
	// detailsTemplate is used for /details, which actually displays the stacktraces and fuzzes.
	detailsTemplate *template.Template = nil

	storageClient *storage.Client = nil

	versionWatcher *fcommon.VersionWatcher = nil
)

var (
	// web server params
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8001", "HTTP service port (e.g., ':8002')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	boltDBPath     = flag.String("bolt_db_path", "fuzzer-db", "The path to the bolt db to be used as a local cache.")

	// OAUTH params
	authWhiteList = flag.String("auth_whitelist", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://fuzzer.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")

	// Scanning params
	skiaRoot           = flag.String("skia_root", "", "[REQUIRED] The root directory of the Skia source code.")
	clangPath          = flag.String("clang_path", "", "[REQUIRED] The path to the clang executable.")
	clangPlusPlusPath  = flag.String("clang_p_p_path", "", "[REQUIRED] The path to the clang++ executable.")
	depotToolsPath     = flag.String("depot_tools_path", "", "The absolute path to depot_tools.  Can be empty if they are on your path.")
	bucket             = flag.String("bucket", "skia-fuzzer", "The GCS bucket in which to locate found fuzzes.")
	downloadProcesses  = flag.Int("download_processes", 4, "The number of download processes to be used for fetching fuzzes.")
	versionCheckPeriod = flag.Duration("version_check_period", 20*time.Second, `The period used to check the version of Skia that needs fuzzing.`)
)

var requiredFlags = []string{"skia_root", "clang_path", "clang_p_p_path", "bolt_db_path"}

func Init() {
	reloadTemplates()
}

func reloadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	detailsTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/details.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func main() {
	defer common.LogPanic()
	// Calls flag.Parse()
	common.InitWithMetrics("fuzzer", graphiteServer)

	if err := writeFlagsToConfig(); err != nil {
		glog.Fatalf("Problem with configuration: %s", err)
	}

	Init()

	if err := setupOAuth(); err != nil {
		glog.Fatal(err)
	}
	go func() {
		if err := fcommon.DownloadSkiaVersionForFuzzing(storageClient, config.FrontEnd.SkiaRoot, &config.FrontEnd); err != nil {
			glog.Fatalf("Problem downloading Skia: %s", err)
		}

		cache, err := fuzzcache.New(config.FrontEnd.BoltDBPath)
		if err != nil {
			glog.Fatalf("Could not create fuzz report cache at %s: %s", config.FrontEnd.BoltDBPath, err)
		}
		defer util.Close(&cache)

		if err := frontend.LoadFromBoltDB(cache); err != nil {
			glog.Errorf("Could not load from boltdb.  Loading from source of truth anyway. %s", err)
		}
		var finder functionnamefinder.Finder
		if !*local {
			if finder, err = functionnamefinder.New(); err != nil {
				glog.Fatalf("Error loading Skia Source: %s", err)
			}
		}
		if err := frontend.LoadFromGoogleStorage(storageClient, finder, cache); err != nil {
			glog.Fatalf("Error loading in data from GCS: %s", err)
		}
		updater := frontend.NewVersionUpdater(storageClient, cache)
		versionWatcher = fcommon.NewVersionWatcher(storageClient, config.FrontEnd.VersionCheckPeriod, updater.HandlePendingVersion, updater.HandleCurrentVersion)
		versionWatcher.Start()

		err = <-versionWatcher.Status
		glog.Fatal(err)
	}()
	runServer()
}

func writeFlagsToConfig() error {
	// Check the required ones and terminate if they are not provided
	for _, f := range requiredFlags {
		if flag.Lookup(f).Value.String() == "" {
			return fmt.Errorf("Required flag %s is empty.", f)
		}
	}
	var err error
	config.FrontEnd.SkiaRoot, err = fileutil.EnsureDirExists(*skiaRoot)
	if err != nil {
		return err
	}
	config.FrontEnd.BoltDBPath = *boltDBPath
	config.FrontEnd.VersionCheckPeriod = *versionCheckPeriod
	config.Common.ClangPath = *clangPath
	config.Common.ClangPlusPlusPath = *clangPlusPlusPath
	config.Common.DepotToolsPath = *depotToolsPath
	config.GS.Bucket = *bucket
	config.FrontEnd.NumDownloadProcesses = *downloadProcesses
	return nil
}

func setupOAuth() error {
	var useRedirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		useRedirectURL = *redirectURL
	}
	if err := login.InitFromMetadataOrJSON(useRedirectURL, login.DEFAULT_SCOPE, *authWhiteList); err != nil {
		return fmt.Errorf("Problem setting up server OAuth: %s", err)
	}

	client, err := auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_READ_ONLY)
	if err != nil {
		return fmt.Errorf("Problem setting up client OAuth: %s", err)
	}

	storageClient, err = storage.NewClient(context.Background(), cloud.WithBaseHTTP(client))
	if err != nil {
		return fmt.Errorf("Problem authenticating: %s", err)
	}
	return nil
}

func runServer() {
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))

	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/details", detailsPageHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/fuzz-list", fuzzListHandler)
	r.HandleFunc("/json/details", detailedReportsHandler)
	r.HandleFunc("/status", statusHandler)
	r.HandleFunc(`/fuzz/{kind:(binary|api)}/{name:[0-9a-f]+\.(skp)}`, fuzzHandler)
	r.HandleFunc(`/metadata/{kind:(binary|api)}/{type:(skp)}/{name:[0-9a-f]+_(debug|release)\.(err|dump)}`, metadataHandler)

	rootHandler := login.ForceAuth(util.LoggingGzipRequestResponse(r), OAUTH2_CALLBACK_PATH)

	http.Handle("/", rootHandler)
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}

	w.Header().Set("Content-Type", "text/html")

	if err := indexTemplate.Execute(w, nil); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

func detailsPageHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}

	w.Header().Set("Content-Type", "text/html")

	if err := detailsTemplate.Execute(w, nil); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

func fuzzListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	f := fuzz.FuzzSummary()

	if err := json.NewEncoder(w).Encode(f); err != nil {
		glog.Errorf("Failed to write or encode output: %v", err)
		return
	}
}

func detailedReportsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var f fuzz.FileFuzzReport
	if name := r.FormValue("name"); name != "" {
		var err error
		if f, err = fuzz.FindFuzzDetailForFuzz(name); err != nil {
			util.ReportError(w, r, err, "There was a problem fulfilling the request.")
		}
	} else {
		line, err := strconv.ParseInt(r.FormValue("line"), 10, 32)
		if err != nil {
			line = fcommon.UNKNOWN_LINE
		}

		if f, err = fuzz.FindFuzzDetails(r.FormValue("file"), r.FormValue("func"), int(line), r.FormValue("fuzz-type") == "binary"); err != nil {
			util.ReportError(w, r, err, "There was a problem fulfilling the request.")
			return
		}
	}

	if err := json.NewEncoder(w).Encode(f); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

func fuzzHandler(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)
	kind := v["kind"]
	name := v["name"]
	xs := strings.Split(name, ".")
	hash, ftype := xs[0], xs[1]
	contents, err := gs.FileContentsFromGS(storageClient, config.GS.Bucket, fmt.Sprintf("%s_fuzzes/%s/bad/%s/%s/%s", kind, config.FrontEnd.SkiaVersion.Hash, ftype, hash, hash))
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Fuzz with name %v not found", v["name"]))
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", name)
	n, err := w.Write(contents)
	if err != nil || n != len(contents) {
		glog.Errorf("Could only serve %d bytes of fuzz %s, not %d: %s", n, hash, len(contents), err)
		return
	}
}

func metadataHandler(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)
	ftype := v["type"]
	kind := v["kind"]
	name := v["name"]
	hash := strings.Split(name, "_")[0]

	contents, err := gs.FileContentsFromGS(storageClient, config.GS.Bucket, fmt.Sprintf("%s_fuzzes/%s/bad/%s/%s/%s", kind, config.FrontEnd.SkiaVersion.Hash, ftype, hash, name))
	if err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Fuzz with name %v not found", v["name"]))
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", name)
	n, err := w.Write(contents)
	if err != nil || n != len(contents) {
		glog.Errorf("Could only serve %d bytes of metadata %s, not %d: %s", n, name, len(contents), err)
		return
	}
}

type commit struct {
	Hash   string `json:"hash"`
	Author string `json:"author"`
}

type status struct {
	Current commit  `json:"current"`
	Pending *commit `json:"pending"`
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s := status{
		Current: commit{
			Hash:   "loading",
			Author: "(Loading)",
		},
		Pending: nil,
	}

	s.Current.Hash = config.FrontEnd.SkiaVersion.Hash
	s.Current.Author = config.FrontEnd.SkiaVersion.Author
	if versionWatcher != nil {
		if pending := versionWatcher.PendingVersion; pending != nil {
			s.Pending = &commit{
				Hash:   pending.Hash,
				Author: pending.Author,
			}
		}
	}

	if err := json.NewEncoder(w).Encode(s); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}
