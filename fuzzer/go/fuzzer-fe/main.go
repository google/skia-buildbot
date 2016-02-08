package main

/*
Runs the frontend portion of the fuzzer.  This primarily is the webserver (see DESIGN.md)
*/

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	fcommon "go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/frontend"
	"go.skia.org/infra/fuzzer/go/frontend/data"
	"go.skia.org/infra/fuzzer/go/frontend/gsloader"
	"go.skia.org/infra/fuzzer/go/frontend/syncer"
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
	// detailsTemplate is used for /category/foo, which displays the number of
	// fuzzes by file/function/line
	overviewTemplate *template.Template = nil
	// detailsTemplate is used for /details, which displays the information from
	// overview as well as the stacktraces and fuzzes.
	detailsTemplate *template.Template = nil

	storageClient *storage.Client = nil

	versionWatcher *fcommon.VersionWatcher = nil

	fuzzSyncer *syncer.FuzzSyncer = nil
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
	skiaRoot          = flag.String("skia_root", "", "[REQUIRED] The root directory of the Skia source code.")
	clangPath         = flag.String("clang_path", "", "[REQUIRED] The path to the clang executable.")
	clangPlusPlusPath = flag.String("clang_p_p_path", "", "[REQUIRED] The path to the clang++ executable.")
	depotToolsPath    = flag.String("depot_tools_path", "", "The absolute path to depot_tools.  Can be empty if they are on your path.")
	bucket            = flag.String("bucket", "skia-fuzzer", "The GCS bucket in which to locate found fuzzes.")
	downloadProcesses = flag.Int("download_processes", 4, "The number of download processes to be used for fetching fuzzes.")

	// Other params
	versionCheckPeriod = flag.Duration("version_check_period", 20*time.Second, `The period used to check the version of Skia that needs fuzzing.`)
	fuzzSyncPeriod     = flag.Duration("fuzz_sync_period", 2*time.Minute, `The period used to sync bad fuzzes and check the count of grey and bad fuzzes.`)
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
	overviewTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/overview.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	detailsTemplate = template.New("details.html")
	// Allows this template to have Polymer binding in it and go template markup.  The go templates
	// have been changed to be {%.Thing%} instead of {{.Thing}}
	detailsTemplate.Delims("{%", "%}")
	detailsTemplate = template.Must(detailsTemplate.ParseFiles(
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

		fuzzSyncer = syncer.New(storageClient)
		fuzzSyncer.Start()

		cache, err := fuzzcache.New(config.FrontEnd.BoltDBPath)
		if err != nil {
			glog.Fatalf("Could not create fuzz report cache at %s: %s", config.FrontEnd.BoltDBPath, err)
		}
		defer util.Close(cache)

		if err := gsloader.LoadFromBoltDB(cache); err != nil {
			glog.Errorf("Could not load from boltdb.  Loading from source of truth anyway. %s", err)
		}
		gsLoader := gsloader.New(storageClient, cache)
		if err := gsLoader.LoadFreshFromGoogleStorage(); err != nil {
			glog.Fatalf("Error loading in data from GCS: %s", err)
		}
		fuzzSyncer.SetGSLoader(gsLoader)
		updater := frontend.NewVersionUpdater(gsLoader, fuzzSyncer)
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
	config.FrontEnd.FuzzSyncPeriod = *fuzzSyncPeriod
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
	r.HandleFunc("/category/{category:[a-z_]+}", detailsPageHandler)
	r.HandleFunc("/category/{category:[a-z_]+}/name/{name}", detailsPageHandler)
	r.HandleFunc("/category/{category:[a-z_]+}/file/{file}", detailsPageHandler)
	r.HandleFunc("/category/{category:[a-z_]+}/file/{file}/func/{function}", detailsPageHandler)
	r.HandleFunc(`/category/{category:[a-z_]+}/file/{file}/func/{function}/line/{line}`, detailsPageHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/fuzz-summary", summaryJSONHandler)
	r.HandleFunc("/json/details", detailsJSONHandler)
	r.HandleFunc("/json/status", statusJSONHandler)
	r.HandleFunc(`/fuzz/{category:[a-z_]+}/{name:[0-9a-f]+}`, fuzzHandler)
	r.HandleFunc(`/metadata/{category:[a-z_]+}/{name:[0-9a-f]+_(debug|release)\.(err|dump|asan)}`, metadataHandler)
	r.HandleFunc("/fuzz_count", fuzzCountHandler)
	r.HandleFunc("/newBug", newBugHandler)

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

	var cat = struct {
		Category string
	}{
		Category: mux.Vars(r)["category"],
	}

	if err := detailsTemplate.Execute(w, cat); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

func summaryPageHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	var cat = struct {
		Category string
	}{
		Category: mux.Vars(r)["category"],
	}

	if err := overviewTemplate.Execute(w, cat); err != nil {
		glog.Errorf("Failed to expand template: %v", err)
	}
}

type countSummary struct {
	Category        string `json:"category"`
	CategoryDisplay string `json:"categoryDisplay"`
	TotalBad        int    `json:"totalBadCount"`
	TotalGrey       int    `json:"totalGreyCount"`
	// "This" means "newly introduced/fixed in this revision"
	ThisBad  int `json:"thisBadCount"`
	ThisGrey int `json:"thisGreyCount"`
}

func summaryJSONHandler(w http.ResponseWriter, r *http.Request) {
	var overview interface{}
	if cat := r.FormValue("category"); cat != "" {
		overview = data.CategoryOverview(cat)
	} else {
		overview = getOverview()
	}

	if err := json.NewEncoder(w).Encode(overview); err != nil {
		glog.Errorf("Failed to write or encode output: %v", err)
		return
	}
}

func getOverview() []countSummary {
	overviews := make([]countSummary, 0, len(fcommon.FUZZ_CATEGORIES))
	for _, cat := range fcommon.FUZZ_CATEGORIES {
		o := countSummary{
			CategoryDisplay: fcommon.PrettifyCategory(cat),
			Category:        cat,
		}
		c := syncer.FuzzCount{
			TotalBad:  -1,
			TotalGrey: -1,
			ThisBad:   -1,
			ThisGrey:  -1,
		}
		if fuzzSyncer != nil {
			c = fuzzSyncer.LastCount(cat)
		}
		o.TotalBad = c.TotalBad
		o.ThisBad = c.ThisBad
		o.TotalGrey = c.TotalGrey
		o.ThisGrey = c.ThisGrey
		overviews = append(overviews, o)
	}
	return overviews
}

func detailsJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	category := r.FormValue("category")
	name := r.FormValue("name")
	// The file names have "/" in them and the functions can have "(*&" in them.
	// We base64 encode them to prevent problems.
	file, err := decodeBase64(r.FormValue("file"))
	if err != nil {
		util.ReportError(w, r, err, "There was a problem decoding the params.")
		return
	}
	function, err := decodeBase64(r.FormValue("func"))
	if err != nil {
		util.ReportError(w, r, err, "There was a problem decoding the params.")
		return
	}
	lineStr, err := decodeBase64(r.FormValue("line"))
	if err != nil {
		util.ReportError(w, r, err, "There was a problem decoding the params.")
		return
	}

	var f data.FuzzReportTree
	if name != "" {
		var err error
		if f, err = data.FindFuzzDetailForFuzz(category, name); err != nil {
			util.ReportError(w, r, err, "There was a problem fulfilling the request.")
		}
	} else {
		line, err := strconv.ParseInt(lineStr, 10, 32)
		if err != nil {
			line = fcommon.UNKNOWN_LINE
		}

		if f, err = data.FindFuzzDetails(category, file, function, int(line)); err != nil {
			util.ReportError(w, r, err, "There was a problem fulfilling the request.")
			return
		}
	}

	if err := json.NewEncoder(w).Encode(f); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

func decodeBase64(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	b, err := base64.URLEncoding.DecodeString(s)
	return string(b), err
}

func fuzzHandler(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)
	category := v["category"]
	// Check the category to avoid someone trying to download arbitrary files from our bucket
	if !fcommon.HasCategory(category) {
		util.ReportError(w, r, nil, "Category not found")
		return
	}
	name := v["name"]
	contents, err := gs.FileContentsFromGS(storageClient, config.GS.Bucket, fmt.Sprintf("%s/%s/bad/%s/%s", category, config.FrontEnd.SkiaVersion.Hash, name, name))
	if err != nil {
		util.ReportError(w, r, err, "Fuzz not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", name)
	n, err := w.Write(contents)
	if err != nil || n != len(contents) {
		glog.Errorf("Could only serve %d bytes of fuzz %s, not %d: %s", n, name, len(contents), err)
		return
	}
}

func metadataHandler(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)
	category := v["category"]
	// Check the category to avoid someone trying to download arbitrary files from our bucket
	if !fcommon.HasCategory(category) {
		util.ReportError(w, r, nil, "Category not found")
		return
	}
	name := v["name"]
	hash := strings.Split(name, "_")[0]

	contents, err := gs.FileContentsFromGS(storageClient, config.GS.Bucket, fmt.Sprintf("%s/%s/bad/%s/%s", category, config.FrontEnd.SkiaVersion.Hash, hash, name))
	if err != nil {
		util.ReportError(w, r, err, "Fuzz metadata not found")
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

func statusJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s := status{
		Current: commit{
			Hash:   "loading",
			Author: "(Loading)",
		},
		Pending: nil,
	}

	if config.FrontEnd.SkiaVersion != nil {
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
	}

	if err := json.NewEncoder(w).Encode(s); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

func fuzzCountHandler(w http.ResponseWriter, r *http.Request) {
	c := syncer.FuzzCount{
		TotalBad:  -1,
		TotalGrey: -1,
		ThisBad:   -1,
		ThisGrey:  -1,
	}
	if fuzzSyncer != nil {
		c = fuzzSyncer.LastCount(r.FormValue("category"))
	}
	if err := json.NewEncoder(w).Encode(c); err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

type newBug struct {
	Category       string
	PrettyCategory string
	Name           string
	Revision       string
}

var newBugTemplate = template.Must(template.New("new_bug").Parse(`# Your bug description here about fuzz found in {{.PrettyCategory}}

# tracking metadata below:
fuzz_category: {{.Category}}
fuzz_commit: {{.Revision}}
related_fuzz: https://fuzzer.skia.org/category/{{.Category}}/name/{{.Name}}
fuzz_download: https://fuzzer.skia.org/fuzz/{{.Category}}/{{.Name}}
`))

func newBugHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.FormValue("name")
	category := r.FormValue("category")
	q := url.Values{
		"labels": []string{"FromSkiaFuzzer,Type-Defect,Priority-Medium,Restrict-View-Googler"},
		"status": []string{"New"},
	}
	b := newBug{
		Category:       category,
		PrettyCategory: fcommon.PrettifyCategory(category),
		Name:           name,
		Revision:       config.FrontEnd.SkiaVersion.Hash,
	}
	var t bytes.Buffer
	if err := newBugTemplate.Execute(&t, b); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Could not create template with %#v", b))
		return
	}
	q.Add("comment", t.String())
	// 303 means "make a GET request to this url"
	http.Redirect(w, r, "https://bugs.chromium.org/p/skia/issues/entry?"+q.Encode(), 303)
}
