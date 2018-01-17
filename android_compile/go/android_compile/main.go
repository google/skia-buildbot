/*
	Android Compile Server for Skia Bots.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"

	//MY_LEASES_URI         = "/my_leases"
	//ALL_LEASES_URI        = "/all_leases"
	//GET_TASK_STATUS_URI   = "/_/get_task_status"
	//POOL_DETAILS_POST_URI = "/_/pooldetails"
	//ADD_TASK_POST_URI     = "/_/add_leasing_task"
	//EXTEND_TASK_POST_URI  = "/_/extend_leasing_task"
	//EXPIRE_TASK_POST_URI  = "/_/expire_leasing_task"

	REGISTER_RUN_POST_URI = "/_/register"
	GET_TASK_STATUS_URI   = "/get_task_status"
	// Do a redirection for logs to Google Storage like CT does?
	PROD_URI = "https://android-compile.skia.org"
)

var (
	// Flags
	host           = flag.String("host", "localhost", "HTTP service host")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files.  If blank then the directory two directories up from this source file will be used.")
	updateInterval = flag.Duration("update_interval", 10*time.Minute, "How often the Android compile server will update it's local checkouts.")
	numCheckouts   = flag.Int("num_checkouts", 10, "The number of checkouts the Android compile server should maintain.")

	// Datastore params
	namespace   = flag.String("namespace", "android-compile", "The Cloud Datastore namespace, such as 'android-compile'.")
	projectName = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")

	// OAUTH params
	authWhiteList = flag.String("auth_whitelist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://leasing.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")

	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	serverURL string
)

type Request struct {
	ChangeNum int `json:"change_num"`
	PatchNum  int `json:"patch_num"`
}

func reloadTemplates() {
	if *resourcesDir == "" {
		// If resourcesDir is not specified then consider the directory two directories up from this
		// source file as the resourcesDir.
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	if err := indexTemplate.Execute(w, nil); err != nil {
		httputils.ReportError(w, r, err, "Failed to expand template")
		return
	}
	return
}

// TODO(rmistry): This is only tryjob. Need one for no tryjob as well.

type CompileTask struct {
	Issue    int `json:"issue"`
	PatchSet int `json:"patchset"`

	Hash string `json:"hash"`

	WithPatchSucceeded bool `json:"withpatch_success"`
	NoPatchSucceeded   bool `json:"nopatch_success"`

	WithPatchLog string `json:"withpatch_log"`
	NoPatchLog   string `json:"nopatch_log"`

	Done         bool `json:"done"`
	InfraFailure bool `json:"infra_failure"`
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	taskParam := r.FormValue("task")
	if taskParam == "" {
		httputils.ReportError(w, r, nil, "Missing task parameter")
		return
	}
	taskID, err := strconv.ParseInt(taskParam, 10, 64)
	if err != nil {
		httputils.ReportError(w, r, err, "Invalid task parameter")
		return
	}

	_, t, err := GetDSTask(taskID)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not find task")
		return
	}

	//status := Status{
	//	TaskId:  k.ID,
	//	Expired: t.Done,
	//}
	if err := json.NewEncoder(w).Encode(t); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return

	}

	return
}

func registerRunHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	issueParam := r.FormValue("issue")
	if issueParam == "" {
		httputils.ReportError(w, r, nil, "Missing issue")
		return
	}
	issue, err := strconv.Atoi(issueParam)
	if err != nil {
		httputils.ReportError(w, r, nil, "Issue must be a number")
		return
	}

	patchsetParam := r.FormValue("patchset")
	if patchsetParam == "" {
		httputils.ReportError(w, r, nil, "Missing patchset")
		return
	}
	patchset, err := strconv.Atoi(patchsetParam)
	if err != nil {
		httputils.ReportError(w, r, nil, "Patchset must be a number")
		return
	}

	// Add to datastore and send the key to ApplyAndCompilePatch to update the datastore as well!
	key := GetNewDSKey()
	task := CompileTask{}
	task.Issue = issue
	task.PatchSet = patchset
	datastoreKey, err := PutDSTask(key, &task)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Error putting task in datastore: %v", err))
		return
	}
	fmt.Println("ADDED IT!!!!!!!!!!!!!!!!")
	fmt.Println(datastoreKey.ID)

	// Kick off the task.
	go func() {
		if err := ApplyAndCompilePatch(issue, patchset, &task, datastoreKey); err != nil {
			task.InfraFailure = true
			sklog.Errorf("Error when applying and compiling issue %d and patchset %d %s", issue, patchset, err)
		}
		task.Done = true
		if _, err := UpdateDSTask(datastoreKey, &task); err != nil {
			sklog.Errorf("Could not update compile task with ID %d: %s", datastoreKey.ID, err)
		}

		// Add something for infra error in the task? and something for done in the task as well?
	}()

	// What do I return here?
	jsonResponse := map[string]interface{}{
		"taskID": datastoreKey.ID,
	}
	if err := json.NewEncoder(w).Encode(jsonResponse); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return
	}
}

func runServer() {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/", indexHandler)
	// Change below to POST (to play with from a python script!)
	r.HandleFunc(REGISTER_RUN_POST_URI, registerRunHandler).Methods("GET")
	r.HandleFunc(GET_TASK_STATUS_URI, statusHandler)

	//r.HandleFunc(MY_LEASES_URI, myLeasesHandler)
	//r.HandleFunc(ALL_LEASES_URI, allLeasesHandler)
	//r.HandleFunc(GET_TASK_STATUS_URI, statusHandler)
	//r.HandleFunc(POOL_DETAILS_POST_URI, poolDetailsHandler).Methods("POST")
	//r.HandleFunc(ADD_TASK_POST_URI, addTaskHandler).Methods("POST")
	//r.HandleFunc(EXTEND_TASK_POST_URI, extendTaskHandler).Methods("POST")
	//r.HandleFunc(EXPIRE_TASK_POST_URI, expireTaskHandler).Methods("POST")

	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

// Move this whole section to checkouts.go in it's Init it will make sure the directories exist!

//func updateCheckouts() error {
//	fmt.Println("Going to update all checkouts now!")
//	return nil
//}

func main() {
	flag.Parse()
	defer common.LogPanic()

	if *local {
		// Dont log to cloud in local mode.
		common.InitWithMust(
			"android_compile",
			common.PrometheusOpt(promPort),
		)
		reloadTemplates()
	} else {
		common.InitWithMust(
			"android_compile",
			common.PrometheusOpt(promPort),
			common.CloudLoggingOpt(),
		)
	}

	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	reloadTemplates()
	serverURL = "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	useRedirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		useRedirectURL = *redirectURL
	}
	if err := login.Init(useRedirectURL, *authWhiteList); err != nil {
		sklog.Fatal(fmt.Errorf("Problem setting up server OAuth: %s", err))
	}

	// Initialize cloud datastore.
	if err := DatastoreInit(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init cloud datastore: %s", err)
	}

	// Initialize checkouts.
	if err := CheckoutsInit(*numCheckouts, *workdir); err != nil {
		sklog.Fatalf("Failed to init checkouts: %s", err)
	}
	// Start periodically updating checkouts when they are idle so that they do not
	// fall too far behind HEAD and waste time syncing.
	// UNCOMMENT ME
	//StartCheckoutUpdaters(*updateInterval)
	// TODO(rmistry): Just testing below!
	//if err := ApplyPatch("/usr/local/google/home/rmistry/android-compile-workdir/checkouts/checkout_1", 90521, 1); err != nil {
	//	sklog.Fatal(err)
	//}
	//if err := ApplyAndCompilePatch(90521, 1); err != nil {
	//	sklog.Fatal(err)
	//}

	healthyGauge := metrics2.GetInt64Metric("healthy")
	go func() {
		for range time.Tick(*updateInterval) { // use some other interval? how do other servers do it?
			healthyGauge.Update(1)
		}
	}()

	runServer()
}
