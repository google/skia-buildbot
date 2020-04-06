/*
	Leasing Server for Swarming Bots.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
)

const (
	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"

	MAX_LEASE_DURATION_HRS = 23

	SWARMING_HARD_TIMEOUT = 24 * time.Hour

	LEASE_TASK_PRIORITY = 50

	MY_LEASES_URI         = "/my_leases"
	ALL_LEASES_URI        = "/all_leases"
	GET_TASK_STATUS_URI   = "/_/get_task_status"
	GET_LEASES_POST_URI   = "/_/get_leases"
	POOL_DETAILS_POST_URI = "/_/pooldetails"
	ADD_TASK_POST_URI     = "/_/add_leasing_task"
	EXTEND_TASK_POST_URI  = "/_/extend_leasing_task"
	EXPIRE_TASK_POST_URI  = "/_/expire_leasing_task"
	PROD_URI              = "https://leasing.skia.org"
)

var (
	// Flags
	host = flag.String("host", "localhost", "HTTP service host")
	//promPort                   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	//port                       = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	//local                      = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir = flag.String("workdir", ".", "Directory to use for scratch work.")
	//resourcesDir               = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files.  If blank then the directory two directories up from this source file will be used.")
	isolatesDir                = flag.String("isolates_dir", "", "The directory to find leasing server's isolates files.")
	pollInterval               = flag.Duration("poll_interval", 1*time.Minute, "How often the leasing server will check if tasks have expired.")
	emailClientSecretFile      = flag.String("email_client_secret_file", "/etc/leasing-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile        = flag.String("email_token_cache_file", "/etc/leasing-email-secrets/client_token.json", "OAuth token cache file for sending email.")
	serviceAccountFile         = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")
	poolDetailsUpdateFrequency = flag.Duration("pool_details_update_freq", 5*time.Minute, "How often to call swarming API to refresh the details of supported pools.")

	// Datastore params
	namespace   = flag.String("namespace", "leasing-server", "The Cloud Datastore namespace, such as 'leasing-server'.")
	projectName = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")

	// OAUTH params
	authWhiteList = flag.String("auth_whitelist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")

	serverURL string

	poolToDetails      map[string]*PoolDetails
	poolToDetailsMutex sync.Mutex
)

// Server is the state of the server.
type Server struct {
	templates *template.Template
	//modify    allowed.Allow // Who is allowed to modify tree status.
	//admin     allowed.Allow // Who is allowed to modify rotations.
}

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type Installed struct {
	Installed ClientConfig `json:"installed"`
}

// See baseapp.Constructor.
func New() (baseapp.App, error) {
	// Initialize mailing library.
	var cfg Installed
	err := util.WithReadFile(*emailClientSecretFile, func(f io.Reader) error {
		return json.NewDecoder(f).Decode(&cfg)
	})
	if err != nil {
		sklog.Fatalf("Failed to read client secrets from %q: %s", *emailClientSecretFile, err)
	}
	// Create a copy of the token cache file since mounted secrets are read-only
	// and the access token will need to be updated for the oauth2 flow.
	if !*baseapp.Local {
		fout, err := ioutil.TempFile("", "")
		if err != nil {
			sklog.Fatalf("Unable to create temp file %q: %s", fout.Name(), err)
		}
		err = util.WithReadFile(*emailTokenCacheFile, func(fin io.Reader) error {
			_, err := io.Copy(fout, fin)
			if err != nil {
				err = fout.Close()
			}
			return err
		})
		if err != nil {
			sklog.Fatalf("Failed to write token cache file from %q to %q: %s", *emailTokenCacheFile, fout.Name(), err)
		}
		*emailTokenCacheFile = fout.Name()
	}
	if err := MailInit(cfg.Installed.ClientID, cfg.Installed.ClientSecret, *emailTokenCacheFile); err != nil {
		sklog.Fatalf("Failed to init mail library: %s", err)
	}

	var allow allowed.Allow
	if !*baseapp.Local {
		allow = allowed.NewAllowedFromList([]string{*authWhiteList})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	// Initialize isolate and swarming.
	if err := SwarmingInit(*serviceAccountFile); err != nil {
		sklog.Fatalf("Failed to init isolate and swarming: %s", err)
	}

	// Initialize cloud datastore.
	if err := DatastoreInit(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init cloud datastore: %s", err)
	}

	poolToDetails, err = GetDetailsOfAllPools()
	if err != nil {
		sklog.Fatalf("Could not get details of all pools: %s", err)
	}
	go func() {
		for range time.Tick(*poolDetailsUpdateFrequency) {
			poolToDetailsMutex.Lock()
			poolToDetails, err = GetDetailsOfAllPools()
			poolToDetailsMutex.Unlock()
			if err != nil {
				sklog.Errorf("Could not get details of all pools: %s", err)
			}
		}
	}()

	healthyGauge := metrics2.GetInt64Metric("healthy")
	go func() {
		for range time.Tick(*pollInterval) {
			healthyGauge.Update(1)
			if err := pollSwarmingTasks(); err != nil {
				sklog.Errorf("Error when checking for expired tasks: %v", err)
			}
		}
	}()

	srv := &Server{}
	srv.loadTemplates()

	return srv, nil
}

func (srv *Server) loadTemplates() {
	srv.templates = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*baseapp.ResourcesDir, "index.html"),
		filepath.Join(*baseapp.ResourcesDir, "leases_list.html"),
	))
}

// user returns the currently logged in user, or a placeholder if running locally.
func (srv *Server) user(r *http.Request) string {
	user := "barney@example.org"
	if !*baseapp.Local {
		user = login.LoggedInAs(r)
	}
	return user
}

// See baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	//r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/", srv.indexHandler)
	r.HandleFunc(MY_LEASES_URI, srv.myLeasesHandler)
	r.HandleFunc(ALL_LEASES_URI, srv.allLeasesHandler)
	r.HandleFunc(POOL_DETAILS_POST_URI, poolDetailsHandler).Methods("POST")
	r.HandleFunc(GET_LEASES_POST_URI, srv.getLeasesHandler).Methods("POST")
	r.HandleFunc(ADD_TASK_POST_URI, addTaskHandler).Methods("POST")
	r.HandleFunc(EXTEND_TASK_POST_URI, extendTaskHandler).Methods("POST")
	r.HandleFunc(EXPIRE_TASK_POST_URI, expireTaskHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)

	h := httputils.LoggingGzipRequestResponse(r)
	h = httputils.HealthzAndHTTPS(h)

	http.Handle("/", h)
	http.HandleFunc(GET_TASK_STATUS_URI, statusHandler)

	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*baseapp.Port, nil))

	//// For login/logout.
	//r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	//r.HandleFunc("/logout/", login.LogoutHandler)
	//r.HandleFunc("/loginstatus/", login.StatusHandler)

	//// All endpoints that require authentication should be added to this router. The
	//// rest of endpoints are left unauthenticated because they are accessed from various
	//// places like: Skia infra apps, Gerrit plugin, Chrome extensions, presubmits, etc.
	//appRouter := mux.NewRouter()

	//// For tree status.
	//appRouter.HandleFunc("/", srv.treeStateHandler).Methods("GET")
	//appRouter.HandleFunc("/_/add_tree_status", srv.addStatusHandler).Methods("POST")
	//appRouter.HandleFunc("/_/get_autorollers", srv.autorollersHandler).Methods("POST")
	//appRouter.HandleFunc("/_/recent_statuses", srv.recentStatusesHandler).Methods("POST")
	//r.HandleFunc("/current", httputils.CorsHandler(srv.bannerStatusHandler)).Methods("GET")

	//// For rotations.
	//appRouter.HandleFunc("/sheriff", srv.sheriffHandler).Methods("GET")
	//appRouter.HandleFunc("/robocop", srv.robocopHandler).Methods("GET")
	//appRouter.HandleFunc("/wrangler", srv.wranglerHandler).Methods("GET")
	//appRouter.HandleFunc("/trooper", srv.trooperHandler).Methods("GET")

	//appRouter.HandleFunc("/update_sheriff_rotations", srv.updateSheriffRotationsHandler).Methods("GET")
	//appRouter.HandleFunc("/update_robocop_rotations", srv.updateRobocopRotationsHandler).Methods("GET")
	//appRouter.HandleFunc("/update_wrangler_rotations", srv.updateWranglerRotationsHandler).Methods("GET")
	//appRouter.HandleFunc("/update_trooper_rotations", srv.updateTrooperRotationsHandler).Methods("GET")

	//appRouter.HandleFunc("/_/get_rotations", srv.autorollersHandler).Methods("POST")

	//r.HandleFunc("/current-sheriff", httputils.CorsHandler(srv.currentSheriffHandler)).Methods("GET")
	//r.HandleFunc("/current-robocop", httputils.CorsHandler(srv.currentRobocopHandler)).Methods("GET")
	//r.HandleFunc("/current-wrangler", httputils.CorsHandler(srv.currentWranglerHandler)).Methods("GET")
	//r.HandleFunc("/current-trooper", httputils.CorsHandler(srv.currentTrooperHandler)).Methods("GET")

	//r.HandleFunc("/next-sheriff", httputils.CorsHandler(srv.nextSheriffHandler)).Methods("GET")
	//r.HandleFunc("/next-robocop", httputils.CorsHandler(srv.nextRobocopHandler)).Methods("GET")
	//r.HandleFunc("/next-wrangler", httputils.CorsHandler(srv.nextWranglerHandler)).Methods("GET")
	//r.HandleFunc("/next-trooper", httputils.CorsHandler(srv.nextTrooperHandler)).Methods("GET")

	//// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	//appHandler := http.Handler(appRouter)
	//if !*baseapp.Local {
	//	appHandler = login.ForceAuth(appRouter, login.DEFAULT_REDIRECT_URL)
	//}

	//r.PathPrefix("/").Handler(appHandler)
}

//func loginHandler(w http.ResponseWriter, r *http.Request) {
//	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
//	return
//}

func (srv *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	if err := srv.templates.ExecuteTemplate(w, "index.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
	return
}

type Status struct {
	TaskId  int64
	Expired bool
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	taskParam := r.FormValue("task")
	if taskParam == "" {
		httputils.ReportError(w, nil, "Missing task parameter", http.StatusInternalServerError)
		return
	}
	taskID, err := strconv.ParseInt(taskParam, 10, 64)
	if err != nil {
		httputils.ReportError(w, err, "Invalid task parameter", http.StatusInternalServerError)
		return
	}

	k, t, err := GetDSTask(taskID)
	if err != nil {
		httputils.ReportError(w, err, "Could not find task", http.StatusInternalServerError)
		return
	}

	status := Status{
		TaskId:  k.ID,
		Expired: t.Done,
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		httputils.ReportError(w, err, "Failed to encode JSON", http.StatusInternalServerError)
		return

	}

	return
}

func poolDetailsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	poolParam := r.FormValue("pool")
	if poolParam == "" {
		httputils.ReportError(w, nil, "Missing pool parameter", http.StatusInternalServerError)
		return
	}
	poolToDetailsMutex.Lock()
	defer poolToDetailsMutex.Unlock()
	poolDetails, ok := poolToDetails[poolParam]
	if !ok {
		httputils.ReportError(w, nil, "No such pool", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(poolDetails); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

type Task struct {
	Requester          string    `json:"requester"`
	OsType             string    `json:"osType"`
	DeviceType         string    `json:"deviceType"`
	InitialDurationHrs string    `json:"duration"`
	Created            time.Time `json:"created"`
	LeaseStartTime     time.Time `json:"leaseStartTime"`
	LeaseEndTime       time.Time `json:"leaseEndTime"`
	Description        string    `json:"description"`
	Done               bool      `json:"done"`
	WarningSent        bool      `json:"warningSent"`

	TaskIdForIsolates string `json:"taskIdForIsolates"`
	SwarmingPool      string `json:"pool"`
	SwarmingBotId     string `json:"botId"`
	SwarmingServer    string `json:"swarmingServer"`
	SwarmingTaskId    string `json:"swarmingTaskId"`
	SwarmingTaskState string `json:"swarmingTaskState"`

	DatastoreId int64 `json:"datastoreId"`

	// Left for backwards compatibility but no longer used.
	Architecture  string `json:"architecture"`
	SetupDebugger bool   `json:"setupDebugger"`
}

type sortTasks []*Task

func (a sortTasks) Len() int      { return len(a) }
func (a sortTasks) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortTasks) Less(i, j int) bool {
	return a[i].Created.After(a[j].Created)
}

func getLeasingTasks(filterUser string) ([]*Task, error) {
	tasks := []*Task{}
	it := GetAllDSTasks(filterUser)
	for {
		t := &Task{}
		k, err := it.Next(t)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		t.DatastoreId = k.ID
		tasks = append(tasks, t)
	}
	sort.Sort(sortTasks(tasks))

	return tasks, nil
}

func (srv *Server) getLeasesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	reqGetLeasesRequest := struct {
		filterByUser bool `json:"filter_by_user"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&reqGetLeasesRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request", http.StatusInternalServerError)
		return
	}
	filterUser := ""
	if reqGetLeasesRequest.filterByUser {
		filterUser = login.LoggedInAs(r)
	}
	tasks, err := getLeasingTasks(filterUser)
	if err != nil {
		httputils.ReportError(w, err, "Failed to expand template", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) leasesHandlerHelper(w http.ResponseWriter, r *http.Request, filterUser string) {
	w.Header().Set("Content-Type", "text/html")

	//tasks, err := getLeasingTasks(filterUser)
	//if err != nil {
	//	httputils.ReportError(w, err, "Failed to expand template", http.StatusInternalServerError)
	//	return
	//}

	//var templateTasks = struct {
	//	Tasks []*Task
	//}{
	//	Tasks: tasks,
	//}
	//if err := leasesListTemplate.Execute(w, templateTasks); err != nil {
	//	httputils.ReportError(w, err, "Failed to expand template", http.StatusInternalServerError)
	//	return
	//}
	if err := srv.templates.ExecuteTemplate(w, "leases_list.html", map[string]string{
		// Look in webpack.config.js for where the nonce templates are injected.
		"Nonce": secure.CSPNonce(r.Context()),
	}); err != nil {
		httputils.ReportError(w, err, "Failed to expand template.", http.StatusInternalServerError)
		return
	}
	return
}

func (srv *Server) myLeasesHandler(w http.ResponseWriter, r *http.Request) {
	srv.leasesHandlerHelper(w, r, login.LoggedInAs(r))
}

func (srv *Server) allLeasesHandler(w http.ResponseWriter, r *http.Request) {
	srv.leasesHandlerHelper(w, r, "" /* filterUser */)
}

func extendTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	extendRequest := struct {
		TaskID      int64 `json:"task"`
		DurationHrs int   `json:"duration"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&extendRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode extend request", http.StatusInternalServerError)
		return
	}

	k, t, err := GetDSTask(extendRequest.TaskID)
	if err != nil {
		httputils.ReportError(w, err, "Could not find task", http.StatusInternalServerError)
		return
	}

	// Add duration hours to the task's lease end time only if ends up being
	// less than 23 hours after the task's creation time.
	newLeaseEndTime := t.LeaseEndTime.Add(time.Hour * time.Duration(extendRequest.DurationHrs))
	maxPossibleLeaseEndTime := t.Created.Add(time.Hour * time.Duration(MAX_LEASE_DURATION_HRS))
	if newLeaseEndTime.After(maxPossibleLeaseEndTime) {
		httputils.ReportError(w, nil, fmt.Sprintf("Can not extend lease beyond %d hours of the task creation time", MAX_LEASE_DURATION_HRS), http.StatusInternalServerError)
		return
	}

	// Change the lease end time.
	t.LeaseEndTime = newLeaseEndTime
	// Reset the warning sent flag since the lease has been extended.
	t.WarningSent = false
	if _, err := UpdateDSTask(k, t); err != nil {
		httputils.ReportError(w, err, "Error updating task in datastore", http.StatusInternalServerError)
		return
	}
	// Inform the requester that the task has been extended by durationHrs.
	//if err := SendExtensionEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId, extendRequest.DurationHrs); err != nil {
	//	httputils.ReportError(w, err, "Error sending extension email", http.StatusInternalServerError)
	//	return
	//}

	if err := json.NewEncoder(w).Encode(t); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func expireTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	expireRequest := struct {
		TaskID int64 `json:"task"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&expireRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode expire request", http.StatusInternalServerError)
		return
	}

	k, t, err := GetDSTask(expireRequest.TaskID)
	if err != nil {
		httputils.ReportError(w, err, "Could not find task", http.StatusInternalServerError)
		return
	}

	// Change the task to Done, change the lease end time to now, and mark the
	// state as successfully completed.
	t.Done = true
	t.LeaseEndTime = time.Now()
	t.SwarmingTaskState = getCompletedStateStr(false)
	if _, err := UpdateDSTask(k, t); err != nil {
		httputils.ReportError(w, err, "Error updating task in datastore", http.StatusInternalServerError)
		return
	}
	//// Inform the requester that the task has completed.
	//if err := SendCompletionEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId); err != nil {
	//	httputils.ReportError(w, err, "Error sending completion email", http.StatusInternalServerError)
	//	return
	//}

	if err := json.NewEncoder(w).Encode(t); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := context.Background()

	task := &Task{}
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add %T task", task), http.StatusInternalServerError)
		return
	}
	defer util.Close(r.Body)

	key := GetNewDSKey()
	if task.SwarmingBotId != "" {
		// If BotId is specified then validate it so that we can fail fast if
		// necessary.
		validBotId, err := IsBotIdValid(task.SwarmingPool, task.SwarmingBotId)
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Error querying swarming for botId %s in pool %s", task.SwarmingBotId, task.SwarmingPool), http.StatusInternalServerError)
			return
		}
		if !validBotId {
			httputils.ReportError(w, err, fmt.Sprintf("Could not find botId %s in pool %s", task.SwarmingBotId, task.SwarmingPool), http.StatusInternalServerError)
			return
		}
	}
	// Populate deviceType only if Android is the osType.
	if task.OsType != "Android" {
		task.DeviceType = ""
	}
	// Add the username of the requester.
	task.Requester = login.LoggedInAs(r)
	// Add the created time.
	task.Created = time.Now()
	// Set to pending.
	task.SwarmingTaskState = swarming.TASK_STATE_PENDING

	// Isolate artifacts.
	var isolateDetails *IsolateDetails
	if task.TaskIdForIsolates != "" {
		t, err := GetSwarmingTaskMetadata(task.SwarmingPool, task.TaskIdForIsolates)
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not find taskId %s in pool %s", task.TaskIdForIsolates, task.SwarmingPool), http.StatusInternalServerError)
			return
		}
		isolateDetails, err = GetIsolateDetails(ctx, *serviceAccountFile, swarming.GetTaskRequestProperties(t))
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not get isolate details of task %s in pool %s", task.TaskIdForIsolates, task.SwarmingPool), http.StatusInternalServerError)
			return
		}
	} else {
		isolateDetails = &IsolateDetails{}
	}

	datastoreKey, err := PutDSTask(key, task)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error putting task in datastore: %v", err), http.StatusInternalServerError)
		return
	}
	isolateHash, err := GetIsolateHash(ctx, task.SwarmingPool, isolateDetails.IsolateDep)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error when getting isolate hash: %v", err), http.StatusInternalServerError)
		return
	}
	// Trigger the swarming task.
	swarmingTaskId, err := TriggerSwarmingTask(task.SwarmingPool, task.Requester, strconv.Itoa(int(datastoreKey.ID)), task.OsType, task.DeviceType, task.SwarmingBotId, serverURL, isolateHash, isolateDetails)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error when triggering swarming task: %v", err), http.StatusInternalServerError)
		return
	}

	// Update the task with swarming fields.
	swarmingInstance := GetSwarmingInstance(task.SwarmingPool)
	task.SwarmingServer = swarmingInstance.SwarmingServer
	task.SwarmingTaskId = swarmingTaskId
	if _, err = UpdateDSTask(datastoreKey, task); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error updating task with swarming fields in datastore: %v", err), http.StatusInternalServerError)
		return
	}

	sklog.Infof("Added %v task into the datastore with key %s", task, datastoreKey)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

// See baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(New, []string{"leasing.skia.org"})
}
