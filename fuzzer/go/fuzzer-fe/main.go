package main

/*
Runs the frontend portion of the fuzzer.  This primarily is the webserver (see DESIGN.md)
*/

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gorilla/mux"
	fcommon "go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/config"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/fuzzer/go/download_skia"
	"go.skia.org/infra/fuzzer/go/frontend"
	"go.skia.org/infra/fuzzer/go/frontend/fuzzcache"
	"go.skia.org/infra/fuzzer/go/frontend/fuzzpool"
	"go.skia.org/infra/fuzzer/go/frontend/gcsloader"
	"go.skia.org/infra/fuzzer/go/frontend/syncer"
	"go.skia.org/infra/fuzzer/go/issues"
	fstorage "go.skia.org/infra/fuzzer/go/storage"
	"go.skia.org/infra/fuzzer/go/version_watcher"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gcs"
	"go.skia.org/infra/go/git/gitinfo"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/go/vcsinfo"
	"google.golang.org/api/option"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil
	// rollTemplate is used for /roll, which allows a user to roll the fuzzer forward.
	rollTemplate *template.Template = nil
	// detailsTemplate is used for /category, which displays the count of fuzzes in various files
	// as well as the stacktraces.
	detailsTemplate *template.Template = nil

	storageClient *storage.Client = nil

	versionWatcher *version_watcher.VersionWatcher = nil

	fuzzSyncer *syncer.FuzzSyncer = nil

	issueManager *issues.IssuesManager = nil

	fuzzPool *fuzzpool.FuzzPool = fuzzpool.New()

	repo     *gitinfo.GitInfo = nil
	repoLock sync.Mutex
)

var (
	// web server params
	host         = flag.String("host", "localhost", "HTTP service host")
	port         = flag.String("port", ":8001", "HTTP service port (e.g., ':8002')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	boltDBPath   = flag.String("bolt_db_path", "fuzzer-db", "The path to the bolt db to be used as a local cache.")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	// OAUTH params
	authWhiteList = flag.String("auth_whitelist", login.DEFAULT_DOMAIN_WHITELIST, "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://fuzzer.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")

	// Scanning params
	// At the moment, the front end does not actually build Skia.  It checks out Skia to get
	// commit information only.  However, it is still not a good idea to share SkiaRoot dirs.
	skiaRoot          = flag.String("skia_root", "", "[REQUIRED] The root directory of the Skia source code.  Cannot be safely shared with backend.")
	depotToolsPath    = flag.String("depot_tools_path", "", "The absolute path to depot_tools.  Can be empty if they are on your path.")
	bucket            = flag.String("bucket", "skia-fuzzer", "The GCS bucket in which to locate found fuzzes.")
	downloadProcesses = flag.Int("download_processes", 4, "The number of download processes to be used for fetching fuzzes.")

	// Other params
	versionCheckPeriod = flag.Duration("version_check_period", 20*time.Second, `The period used to check the version of Skia that needs fuzzing.`)
	fuzzSyncPeriod     = flag.Duration("fuzz_sync_period", 2*time.Minute, `The period used to sync bad fuzzes and check the count of grey and bad fuzzes.`)
	backendNames       = common.NewMultiStringFlag("backend_names", nil, "The names of all backend gce instances, e.g. skia-fuzzer-be-1")
)

var requiredFlags = []string{"skia_root", "bolt_db_path"}

func Init() {
	reloadTemplates()
}

func reloadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
	rollTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/roll.html"),
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
	flag.Parse()
	if *local {
		common.InitWithMust(
			"fuzzer-fe-local",
		)
	} else {
		common.InitWithMust(
			"fuzzer-fe",
			common.PrometheusOpt(promPort),
			common.MetricsLoggingOpt(),
		)
	}

	ctx := context.Background()

	if err := writeFlagsToConfig(); err != nil {
		sklog.Fatalf("Problem with configuration: %s", err)
	}

	Init()

	if err := setupOAuth(ctx); err != nil {
		sklog.Fatal(err)
	}
	client := fstorage.NewFuzzerGCSClient(storageClient, config.GCS.Bucket)

	go func() {
		if err := download_skia.AtGCSRevision(ctx, client, config.Common.SkiaRoot, &config.Common, !*local); err != nil {
			sklog.Fatalf("Problem downloading Skia: %s", err)
		}

		fuzzSyncer = syncer.New(storageClient)
		fuzzSyncer.Start()

		cache, err := fuzzcache.New(config.FrontEnd.BoltDBPath)
		if err != nil {
			sklog.Fatalf("Could not create fuzz report cache at %s: %s", config.FrontEnd.BoltDBPath, err)
		}
		defer util.Close(cache)

		if err := gcsloader.LoadFromBoltDB(fuzzPool, cache); err != nil {
			sklog.Errorf("Could not load from boltdb.  Loading from source of truth anyway. %s", err)
		}
		gsLoader := gcsloader.New(storageClient, cache, fuzzPool)
		if err := gsLoader.LoadFreshFromGoogleStorage(); err != nil {
			sklog.Fatalf("Error loading in data from GCS: %s", err)
		}
		fuzzSyncer.SetGCSLoader(gsLoader)
		updater := frontend.NewVersionUpdater(gsLoader, fuzzSyncer)
		versionWatcher = version_watcher.New(client, config.Common.VersionCheckPeriod, nil, updater.HandleCurrentVersion)
		versionWatcher.Start(ctx)

		err = <-versionWatcher.Status
		sklog.Fatal(err)
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
	config.Common.SkiaRoot, err = fileutil.EnsureDirExists(*skiaRoot)
	if err != nil {
		return err
	}
	config.FrontEnd.BoltDBPath = *boltDBPath
	config.Common.VersionCheckPeriod = *versionCheckPeriod
	config.Common.DepotToolsPath = *depotToolsPath

	config.GCS.Bucket = *bucket
	config.FrontEnd.NumDownloadProcesses = *downloadProcesses
	config.FrontEnd.FuzzSyncPeriod = *fuzzSyncPeriod
	config.FrontEnd.BackendNames = *backendNames
	return nil
}

func setupOAuth(ctx context.Context) error {
	login.InitWithAllow(*port, *local, nil, nil, allowed.Googlers())

	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_READ_WRITE)
	if err != nil {
		return fmt.Errorf("Problem setting up client OAuth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	storageClient, err = storage.NewClient(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("Problem authenticating: %s", err)
	}

	issueManager = issues.NewManager(client)
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
	r.HandleFunc("/category/{category:[0-9a-z_]+}", detailsPageHandler)
	r.HandleFunc("/category/{category:[0-9a-z_]+}/name/{name}", detailsPageHandler)
	r.HandleFunc("/category/{category:[0-9a-z_]+}/file/{file}", detailsPageHandler)
	r.HandleFunc("/category/{category:[0-9a-z_]+}/file/{file}/func/{function}", detailsPageHandler)
	r.HandleFunc(`/category/{category:[0-9a-z_]+}/file/{file}/func/{function}/line/{line}`, detailsPageHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/json/fuzz-summary", httputils.CorsCredentialsHandler(summaryJSONHandler, ".skia.org"))
	r.HandleFunc("/json/details", detailsJSONHandler)
	r.HandleFunc("/json/status", statusJSONHandler)
	r.HandleFunc(`/fuzz/{name:[0-9a-f]+}`, fuzzHandler)
	r.HandleFunc(`/metadata/{name:[0-9a-f]+_(?:debug|release)\.(?:err|dump|asan)}`, metadataHandler)
	r.HandleFunc("/newBug", newBugHandler)
	r.HandleFunc("/roll", rollHandler)
	r.HandleFunc("/roll/revision", updateRevision)

	rootHandler := login.ForceAuth(httputils.LoggingGzipRequestResponse(r), OAUTH2_CALLBACK_PATH)

	rootHandler = httputils.HealthzAndHTTPS(rootHandler)
	http.Handle("/", rootHandler)
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// indexHandler displays the index page, which has no real templating. The client side JS will
// query for more information.
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	if err := indexTemplate.Execute(w, nil); err != nil {
		sklog.Errorf("Failed to expand template: %v", err)
	}
}

// detailsPageHandler displays the details page customized with the category requrested. The client
// side JS will query for more information.
func detailsPageHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	c := mux.Vars(r)["category"]
	context := struct {
		Category      string
		HumanCategory string
		ReproString   string
	}{
		Category:      c,
		HumanCategory: fcommon.PrettifyCategory(c),
		ReproString:   fcommon.ReplicationArgs(c),
	}

	if err := detailsTemplate.Execute(w, context); err != nil {
		sklog.Errorf("Failed to expand template: %v", err)
	}
}

// rollHandler displays the roll page, which has no real templating. The client side JS will
// query for more information and post the roll.
func rollHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	if err := rollTemplate.Execute(w, nil); err != nil {
		sklog.Errorf("Failed to expand template: %v", err)
	}
}

// countSummary represents the data needed to summarize the results for a fuzzer, which is mostly
// counts of what has been found.
type countSummary struct {
	Category        string `json:"category"`
	CategoryDisplay string `json:"categoryDisplay"`
	HighPriority    int    `json:"highPriorityCount"`
	MedPriority     int    `json:"mediumPriorityCount"`
	LowPriority     int    `json:"lowPriorityCount"`
	Status          string `json:"status"`
	Groomer         string `json:"groomer"`
}

// summaryJSONHandler returns a countSummary, representing the results for all fuzzers.
func summaryJSONHandler(w http.ResponseWriter, r *http.Request) {
	summary := getSummary()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		sklog.Errorf("Failed to write or encode output: %v", err)
		return
	}
}

// getSummary() creates a countSummary for every fuzzer.
func getSummary() []countSummary {
	counts := make([]countSummary, 0, len(fcommon.FUZZ_CATEGORIES))
	for _, cat := range fcommon.FUZZ_CATEGORIES {
		o := countSummary{
			CategoryDisplay: fcommon.PrettifyCategory(cat),
			Category:        cat,
		}
		c := syncer.FuzzCount{
			HighPriority: -1,
			MedPriority:  -1,
			LowPriority:  -1,
		}
		if fuzzSyncer != nil {
			c = fuzzSyncer.LastCount(cat)
		}
		o.HighPriority = c.HighPriority
		o.MedPriority = c.MedPriority
		o.LowPriority = c.LowPriority
		o.Status = fcommon.Status(cat)
		o.Groomer = fcommon.Groomer(cat)
		counts = append(counts, o)
	}
	return counts
}

// detailsJSONHandler returns the "details" for a given fuzzer, optionally filtered by file name,
// function name and line number.
func detailsJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	category := r.FormValue("category")
	architecture := r.FormValue("architecture")
	name := r.FormValue("name")
	badOrGrey := r.FormValue("badOrGrey")

	// The file names have "/" in them and the functions can have "(*&" in them.
	// We base64 encode them to prevent problems.
	file, err := decodeBase64(r.FormValue("file"))
	if err != nil {
		httputils.ReportError(w, r, err, "There was a problem decoding the params.")
		return
	}
	function, err := decodeBase64(r.FormValue("func"))
	if err != nil {
		httputils.ReportError(w, r, err, "There was a problem decoding the params.")
		return
	}
	lineStr, err := decodeBase64(r.FormValue("line"))
	if err != nil {
		httputils.ReportError(w, r, err, "There was a problem decoding the params.")
		return
	}

	var reports []data.FuzzReport
	if name != "" {
		if report, err := fuzzPool.FindFuzzDetailForFuzz(name); err != nil {
			httputils.ReportError(w, r, err, "There was a problem fulfilling the request.")
		} else {
			reports = append(reports, report)
		}
	} else {
		line, err := strconv.ParseInt(lineStr, 10, 32)
		if err != nil {
			line = fcommon.UNKNOWN_LINE
		}
		if badOrGrey != "grey" && badOrGrey != "bad" {
			badOrGrey = ""
		}

		if reports, err = fuzzPool.FindFuzzDetails(category, architecture, badOrGrey, file, function, int(line)); err != nil {
			httputils.ReportError(w, r, err, "There was a problem fulfilling the request.")
			return
		}
	}

	if err := json.NewEncoder(w).Encode(reports); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
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

// fuzzHandler serves the contents of the fuzz as application/octet-stream.  It looks up the fuzz
// by name in the fuzzPool and uses the category/architecture/badness from the returned FuzzReport
// to fetch it from Google Storage and return it to the user.  This primarily allows users to
// download grey fuzzes if they want to and simplifies the client side request.
func fuzzHandler(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)

	hash := v["name"]
	if fuzzPool == nil {
		httputils.ReportError(w, r, nil, "Fuzzes not loaded yet")
		return
	}
	fuzz, err := fuzzPool.FindFuzzDetailForFuzz(hash)
	if err != nil {
		httputils.ReportError(w, r, err, "Fuzz not found")
		return
	}
	badOrGrey := "bad"
	if fuzz.IsGrey {
		badOrGrey = "grey"
	}

	contents, err := gcs.FileContentsFromGCS(storageClient, config.GCS.Bucket, fmt.Sprintf("%s/%s/%s/%s/%s/%s", fuzz.FuzzCategory, config.Common.SkiaVersion.Hash, fuzz.FuzzArchitecture, badOrGrey, fuzz.FuzzName, fuzz.FuzzName))
	if err != nil {
		httputils.ReportError(w, r, err, "Fuzz not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	humanName := fcommon.CategoryReminder(fuzz.FuzzCategory)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`filename="%s-%s"`, humanName, hash))
	n, err := w.Write(contents)
	if err != nil || n != len(contents) {
		sklog.Errorf("Could only serve %d bytes of fuzz %s, not %d: %s", n, hash, len(contents), err)
		return
	}
}

// metadataHandler serves the contents of a fuzz's metadata (e.g. stacktrace) as text/plain
//  It looks up the fuzz by name in the fuzzPool and uses the category/architecture/badness from
// the returned FuzzReport to fetch the metadata from Google Storage and return it to the user.
// This primarily allows users to download grey fuzz metadata if they want to and simplifies
//  the client side request.
func metadataHandler(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)
	name := v["name"]
	hash := strings.Split(name, "_")[0]
	if fuzzPool == nil {
		httputils.ReportError(w, r, nil, "Fuzzes not loaded yet")
		return
	}
	fuzz, err := fuzzPool.FindFuzzDetailForFuzz(hash)
	if err != nil {
		httputils.ReportError(w, r, err, "Fuzz not found")
		return
	}
	badOrGrey := "bad"
	if fuzz.IsGrey {
		badOrGrey = "grey"
	}

	contents, err := gcs.FileContentsFromGCS(storageClient, config.GCS.Bucket, fmt.Sprintf("%s/%s/%s/%s/%s/%s", fuzz.FuzzCategory, config.Common.SkiaVersion.Hash, fuzz.FuzzArchitecture, badOrGrey, fuzz.FuzzName, name))
	if err != nil {
		httputils.ReportError(w, r, err, "Fuzz metadata not found")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", name)
	n, err := w.Write(contents)
	if err != nil || n != len(contents) {
		sklog.Errorf("Could only serve %d bytes of metadata %s, not %d: %s", n, name, len(contents), err)
		return
	}
}

type commit struct {
	Hash   string `json:"hash"`
	Author string `json:"author"`
}

// The status struct indicates what Skia revision the fuzzer is currently working on and if it
// is in the middle of rolling to a new revision.
type status struct {
	Current     commit    `json:"current"`
	Pending     *commit   `json:"pending"`
	LastUpdated time.Time `json:"lastUpdated"`
}

// statusJSONHandler returns the current status of the fuzzer using information from the config
// and versionwatcher.  TODO(kjlubick): should it use the config or just versionWatcher?
func statusJSONHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s := status{
		Current: commit{
			Hash:   "loading",
			Author: "(Loading)",
		},
		Pending: nil,
	}

	if config.Common.SkiaVersion != nil {
		s.Current.Hash = config.Common.SkiaVersion.Hash
		s.Current.Author = config.Common.SkiaVersion.Author
		s.LastUpdated = config.Common.SkiaVersion.Timestamp
		if versionWatcher != nil {
			if pending := versionWatcher.LastPendingHash; pending != "" {
				if ci, err := getCommitInfo(context.Background(), pending); err != nil {
					sklog.Errorf("Problem getting git info about pending revision %s: %s", pending, err)
				} else {
					s.Pending = &commit{
						Hash:   ci.Hash,
						Author: ci.Author,
					}
				}
			}
		}
	}

	if err := json.NewEncoder(w).Encode(s); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// newBugHandler redirects the request to a pre-filled bug report at Monorail.
func newBugHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	name := r.FormValue("name")
	category := r.FormValue("category")

	p := issues.IssueReportingPackage{
		Category:       category,
		FuzzName:       name,
		CommitRevision: config.Common.SkiaVersion.Hash,
	}
	if u, err := issueManager.CreateBadBugURL(p); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Problem creating issue link %#v", p))
	} else {
		// 303 means "make a GET request to this url"
		http.Redirect(w, r, u, 303)
	}
}

// getCommitInfo updates the front end's checkout of Skia and then queries it for information about the given revision.
func getCommitInfo(ctx context.Context, revision string) (*vcsinfo.LongCommit, error) {
	repoLock.Lock()
	defer repoLock.Unlock()
	var err error
	repo, err = gitinfo.NewGitInfo(ctx, filepath.Join(config.Common.SkiaRoot, "skia"), false, false)
	if err != nil {
		return nil, fmt.Errorf("Could not create Skia repo: %s", err)
	}

	if err = repo.Checkout(ctx, "master"); err != nil {
		return nil, fmt.Errorf("Could not checkout master: %s", err)
	}

	if err = repo.Update(ctx, true, false); err != nil {
		return nil, fmt.Errorf("Could not update master branch: %s", err)
	}

	currInfo, err := repo.Details(ctx, revision, false)
	if err != nil || currInfo == nil {
		return nil, fmt.Errorf("Could not get info for %s: %s", revision, err)
	}
	return currInfo, nil
}

// updateRevision handles the POST request to roll the revision under fuzz forward.  It checks for
// authentication, verifies the revision is legit, that we are not already rolling forward,
// and then updates GCS with a pending version.
func updateRevision(w http.ResponseWriter, r *http.Request) {
	if !login.IsGoogler(r) {
		http.Error(w, "You do not have permission to push.  You must be a Googler.", http.StatusForbidden)
		return
	}
	ctx := context.Background()
	var msg struct {
		Revision string `json:"revision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to decode request body: %s", err))
		return
	}
	msg.Revision = strings.TrimSpace(msg.Revision)
	if msg.Revision == "" {
		http.Error(w, "Revision cannot be blank", http.StatusBadRequest)
		return
	}
	user := login.LoggedInAs(r)

	sklog.Infof("User %s is trying to roll the fuzzer to revision %q", user, msg.Revision)

	if config.Common.SkiaVersion == nil || versionWatcher == nil || versionWatcher.LastCurrentHash == "" {
		http.Error(w, "The fuzzer isn't finished booting up.  Try again later.", http.StatusServiceUnavailable)
		return
	}
	if versionWatcher.LastPendingHash != "" {
		http.Error(w, "There is already a pending version.", http.StatusBadRequest)
		return
	}

	currInfo, err := getCommitInfo(ctx, versionWatcher.LastCurrentHash)
	if err != nil || currInfo == nil {
		httputils.ReportError(w, r, err, "Could not get information about current revision.  Please try again later")
		return
	}
	newInfo, err := getCommitInfo(ctx, msg.Revision)
	if err != nil || newInfo == nil {
		httputils.ReportError(w, r, err, "Could not get information about revision.  Are you sure it exists?")
		return
	}

	// We can only assume this to be the case because Skia has no branches that would
	// cause commits of a later time to actually be merged in before other commits.
	if newInfo.Timestamp.Before(currInfo.Timestamp) {
		http.Error(w, fmt.Sprintf("Revision cannot be before current revision %s at %s", currInfo.Hash, currInfo.Timestamp), http.StatusBadRequest)
		return
	}

	client := fstorage.NewFuzzerGCSClient(storageClient, config.GCS.Bucket)

	sklog.Infof("Turning the crank to revision %q", newInfo.Hash)
	if err := frontend.UpdateVersionToFuzz(client, config.FrontEnd.BackendNames, newInfo.Hash); err != nil {
		sklog.Errorf("Could not turn the crank: %s", err)
	} else {
		versionWatcher.Recheck()
	}
}
