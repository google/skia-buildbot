/*
	Android Compile Server for Skia Bots.
*/

package main

import (
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
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

	DatastoreId int64 `json:"datastoreId"`
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

func runServer() {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/", indexHandler)
	// Change below to POST
	// r.HandleFunc(POOL_DETAILS_POST_URI, poolDetailsHandler).Methods("POST")

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

	healthyGauge := metrics2.GetInt64Metric("healthy")
	go func() {
		for range time.Tick(*updateInterval) { // use some other interval? how do other servers do it?
			healthyGauge.Update(1)
		}
	}()

	runServer()
}
