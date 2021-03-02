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
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/unrolled/secure"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/leasing/go/types"
)

const (
	maxLeaseDurationHrs = 23

	swarmingHardTimeout = 24 * time.Hour

	leaseTaskPriority = 50

	myLeasesURI              = "/my_leases"
	allLeasesURI             = "/all_leases"
	getTaskStatusURI         = "/_/get_task_status"
	getLeasesPostURI         = "/_/get_leases"
	getSupportedPoolsPostURI = "/_/get_supported_pools"
	poolDetailsPostURI       = "/_/pooldetails"
	addTaskPostURI           = "/_/add_leasing_task"
	extendTaskPostURI        = "/_/extend_leasing_task"
	expireTaskPostURI        = "/_/expire_leasing_task"
)

var (
	// Flags
	host                       = flag.String("host", "leasing.skia.org", "HTTP service host")
	workdir                    = flag.String("workdir", ".", "Directory to use for scratch work.")
	artifactsDir               = flag.String("artifacts_dir", "", "The directory to find leasing server's artifacts.")
	pollInterval               = flag.Duration("poll_interval", 1*time.Minute, "How often the leasing server will check if tasks have expired.")
	emailClientSecretFile      = flag.String("email_client_secret_file", "/etc/leasing-email-secrets/client_secret.json", "OAuth client secret JSON file for sending email.")
	emailTokenCacheFile        = flag.String("email_token_cache_file", "/etc/leasing-email-secrets/client_token.json", "OAuth token cache file for sending email.")
	serviceAccountFile         = flag.String("service_account_file", "/var/secrets/google/key.json", "Service account JSON file.")
	poolDetailsUpdateFrequency = flag.Duration("pool_details_update_freq", 5*time.Minute, "How often to call swarming API to refresh the details of supported pools.")

	// Datastore params
	namespace   = flag.String("namespace", "leasing-server", "The Cloud Datastore namespace, such as 'leasing-server'.")
	projectName = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")

	// OAUTH params
	authAllowList = flag.String("auth_allowlist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")

	poolToDetails      map[string]*types.PoolDetails
	poolToDetailsMutex sync.Mutex
)

type ClientConfig struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type ClientSecretJSON struct {
	Installed ClientConfig `json:"installed"`
}

// New implements baseapp.Constructor.
func New() (baseapp.App, error) {
	// Create workdir if it does not exist.
	if err := os.MkdirAll(*workdir, 0755); err != nil {
		sklog.Fatalf("Could not create %s: %s", *workdir, err)
	}

	// Initialize mailing library.
	var cfg ClientSecretJSON
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
		allow = allowed.NewAllowedFromList([]string{*authAllowList})
	} else {
		allow = allowed.NewAllowedFromList([]string{"fred@example.org", "barney@example.org", "wilma@example.org"})
	}
	login.SimpleInitWithAllow(*baseapp.Port, *baseapp.Local, nil, nil, allow)

	// Initialize swarming.
	if err := SwarmingInit(*serviceAccountFile); err != nil {
		sklog.Fatalf("Failed to init swarming: %s", err)
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

// Server is the state of the server.
type Server struct {
	templates *template.Template
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

// AddHandlers implements baseapp.App.
func (srv *Server) AddHandlers(r *mux.Router) {
	// For login/logout.
	r.HandleFunc(login.DEFAULT_OAUTH2_CALLBACK, login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	// Get task status will be used from swarming bots.
	r.HandleFunc(getTaskStatusURI, srv.statusHandler).Methods("GET")

	// All endpoints that require authentication should be added to this router.
	appRouter := mux.NewRouter()
	appRouter.HandleFunc("/", srv.indexHandler)
	appRouter.HandleFunc(myLeasesURI, srv.myLeasesHandler)
	appRouter.HandleFunc(allLeasesURI, srv.allLeasesHandler)
	appRouter.HandleFunc(poolDetailsPostURI, srv.poolDetailsHandler).Methods("POST")
	appRouter.HandleFunc(getSupportedPoolsPostURI, srv.supportedPoolsHandler).Methods("POST")
	appRouter.HandleFunc(getLeasesPostURI, srv.getLeasesHandler).Methods("POST")
	appRouter.HandleFunc(addTaskPostURI, srv.addTaskHandler).Methods("POST")
	appRouter.HandleFunc(extendTaskPostURI, srv.extendTaskHandler).Methods("POST")
	appRouter.HandleFunc(expireTaskPostURI, srv.expireTaskHandler).Methods("POST")

	// Use the appRouter as a handler and wrap it into middleware that enforces authentication.
	appHandler := http.Handler(appRouter)
	if !*baseapp.Local {
		appHandler = login.ForceAuth(appRouter, login.DEFAULT_REDIRECT_URL)
	}

	r.PathPrefix("/").Handler(appHandler)
}

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

// Status represents the status of a Swarming task.
type Status struct {
	TaskId  int64
	Expired bool
}

func (srv *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
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

func (srv *Server) poolDetailsHandler(w http.ResponseWriter, r *http.Request) {
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

func (srv *Server) supportedPoolsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	supportedPools := []string{}
	poolToDetailsMutex.Lock()
	defer poolToDetailsMutex.Unlock()
	for p := range poolToDetails {
		supportedPools = append(supportedPools, p)
	}
	sort.Strings(supportedPools)
	if err := json.NewEncoder(w).Encode(supportedPools); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to encode JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

type sortTasks []*types.Task

func (a sortTasks) Len() int      { return len(a) }
func (a sortTasks) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortTasks) Less(i, j int) bool {
	return a[i].Created.After(a[j].Created)
}

func getLeasingTasks(filterUser string) ([]*types.Task, error) {
	tasks := []*types.Task{}
	it := GetAllDSTasks(filterUser)
	for {
		t := &types.Task{}
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
		FilterByUser string `json:"filter_by_user"`
	}{}
	if err := json.NewDecoder(r.Body).Decode(&reqGetLeasesRequest); err != nil {
		httputils.ReportError(w, err, "Failed to decode add note request", http.StatusInternalServerError)
		return
	}
	tasks, err := getLeasingTasks(reqGetLeasesRequest.FilterByUser)
	if err != nil {
		httputils.ReportError(w, err, "Failed to expand template", http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(tasks); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) leasesHandlerHelper(w http.ResponseWriter, r *http.Request, filterByUser string) {
	w.Header().Set("Content-Type", "text/html")

	if err := srv.templates.ExecuteTemplate(w, "leases_list.html", map[string]string{
		"FilterByUser": filterByUser,
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
	srv.leasesHandlerHelper(w, r, "" /* filterByUser */)
}

func (srv *Server) extendTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	extendRequest := types.ExtendTaskRequest{}
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
	maxPossibleLeaseEndTime := t.Created.Add(time.Hour * time.Duration(maxLeaseDurationHrs))
	if newLeaseEndTime.After(maxPossibleLeaseEndTime) {
		httputils.ReportError(w, nil, fmt.Sprintf("Can not extend lease beyond %d hours of the task creation time", maxLeaseDurationHrs), http.StatusInternalServerError)
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
	if err := SendExtensionEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId, t.EmailThreadingReference, extendRequest.DurationHrs); err != nil {
		httputils.ReportError(w, err, "Error sending extension email", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(t); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) expireTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	expireRequest := types.ExpireTaskRequest{}
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
	// Inform the requester that the task has completed.
	if err := SendCompletionEmail(t.Requester, t.SwarmingServer, t.SwarmingTaskId, t.SwarmingBotId, t.EmailThreadingReference); err != nil {
		httputils.ReportError(w, err, "Error sending completion email", http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(t); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

func (srv *Server) addTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	ctx := context.Background()

	task := &types.Task{}
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to add %T task", task), http.StatusInternalServerError)
		return
	}
	defer util.Close(r.Body)

	key := GetNewDSKey()
	if task.SwarmingBotId != "" {
		// If BotId is specified then validate it so that we can fail fast if
		// necessary.
		validBotID, err := IsBotIDValid(task.SwarmingPool, task.SwarmingBotId)
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Error querying swarming for botId %s in pool %s", task.SwarmingBotId, task.SwarmingPool), http.StatusInternalServerError)
			return
		}
		if !validBotID {
			httputils.ReportError(w, err, fmt.Sprintf("Could not find botId %s in pool %s", task.SwarmingBotId, task.SwarmingPool), http.StatusInternalServerError)
			return
		}
	}
	// Populate deviceType only if Android  or iOS is the osType.
	if task.OsType != "Android" && !strings.HasPrefix(task.OsType, "iOS") {
		task.DeviceType = ""
	}
	// Add the username of the requester.
	task.Requester = login.LoggedInAs(r)
	// Add the created time.
	task.Created = time.Now()
	// Set to pending.
	task.SwarmingTaskState = swarming.TASK_STATE_PENDING

	// Upload artifacts.
	var swarmingProps *swarming_api.SwarmingRpcsTaskProperties
	if task.TaskIdForIsolates != "" {
		t, err := GetSwarmingTaskMetadata(task.SwarmingPool, task.TaskIdForIsolates)
		if err != nil {
			httputils.ReportError(w, err, fmt.Sprintf("Could not find taskId %s in pool %s", task.TaskIdForIsolates, task.SwarmingPool), http.StatusInternalServerError)
			return
		}
		swarmingProps = swarming.GetTaskRequestProperties(t)
	} else {
		swarmingProps = &swarming_api.SwarmingRpcsTaskProperties{}
	}

	datastoreKey, err := PutDSTask(key, task)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error putting task in datastore: %v", err), http.StatusInternalServerError)
		return
	}
	casDigest, err := AddLeasingArtifactsToCAS(ctx, task.SwarmingPool, swarmingProps.CasInputRoot)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error merging CAS inputs: %s", err), http.StatusInternalServerError)
		return
	}

	// Trigger the swarming task.
	swarmingTaskID, err := TriggerSwarmingTask(task.SwarmingPool, task.Requester, strconv.Itoa(int(datastoreKey.ID)), task.OsType, task.DeviceType, task.SwarmingBotId, *host, casDigest, swarmingProps.RelativeCwd, swarmingProps.CipdInput, swarmingProps.Command)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error when triggering swarming task: %v", err), http.StatusInternalServerError)
		return
	}

	// Update the task with swarming fields.
	swarmingInstance := GetSwarmingInstance(task.SwarmingPool)
	task.SwarmingServer = swarmingInstance.SwarmingServer
	task.SwarmingTaskId = swarmingTaskID
	if _, err = UpdateDSTask(datastoreKey, task); err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Error updating task with swarming fields in datastore: %v", err), http.StatusInternalServerError)
		return
	}

	sklog.Infof("Added %v task into the datastore with key %s", task, datastoreKey)
	if err := json.NewEncoder(w).Encode(task); err != nil {
		sklog.Errorf("Failed to send response: %s", err)
	}
}

// AddMiddleware implements baseapp.App.
func (srv *Server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(New, []string{*host})
}
