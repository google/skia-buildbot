/*
	Swarming Bots Leasing Server.
*/

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	//"os/user"
	"path/filepath"
	"runtime"

	"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"
	swarming_api "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"google.golang.org/api/iterator"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/isolate"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/ds"
)

const (
	GMAIL_CACHED_TOKEN = "leasing_gmail_cached_token"

	// OAUTH2_CALLBACK_PATH is callback endpoint used for the Oauth2 flow.
	OAUTH2_CALLBACK_PATH = "/oauth2callback/"

	// One const for each Datastore Kind.
	TASK ds.Kind = "Task"

	MAX_LEASE_DURATION_HRS = 23

	SWARMING_HARD_TIMEOUT = 24 * time.Hour

	LEASE_TASK_PRIORITY = 50
)

var (
	// flags
	host         = flag.String("host", "localhost", "HTTP service host")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	port         = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir      = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	pollInterval = flag.Duration("poll_interval", 1*time.Minute, "How often the leasing server will check if tasks have expired.")

	// Datastore params
	namespace   = flag.String("namespace", "leasing-server", "The Cloud Datastore namespace, such as 'leasing-server'.")
	projectName = flag.String("project_name", "google.com:skia-buildbots", "The Google Cloud project name.")

	// OAUTH params
	authWhiteList = flag.String("auth_whitelist", "google.com", "White space separated list of domains and email addresses that are allowed to login.")
	redirectURL   = flag.String("redirect_url", "https://leasing.skia.org/oauth2callback/", "OAuth2 redirect url. Only used when local=false.")

	// authenticated http client
	client *http.Client

	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// leasesListTemplate is the page we serve on the my-leases and all-leases pages.
	leasesListTemplate *template.Template = nil

	isolateClient *isolate.Client

	swarmingClient swarming.ApiClient
)

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
	leasesListTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/leases_list.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

// TODO(rmistry): Make isolate server and swarming servers configurable.

func Init() {
	reloadTemplates()
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	//w.Write([]byte("Testing"))

	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")
	//var osTypes = struct {
	//	OsTypes []string
	//}{
	//	// OsTypes: mux.Vars(r)["category"],
	//	OsTypes: []string{"testing1", "testing2"},
	//}

	if err := indexTemplate.Execute(w, nil); err != nil {
		httputils.ReportError(w, r, err, "Failed to expand template")
		return
	}
	return
}

// TODO(rmistry): Move this to a new ds.go module.
func UpdateTask(k *datastore.Key, t *Task) (*datastore.Key, error) {
	return ds.DS.Put(context.Background(), k, t)
}

// TODO(rmistry): Need endpoints to extend and to expire. extend will throw an except if you try to do it 23 hours past the thingy!
// TODO(rmistry): Also need a page to view all/my runs. How do I display both.
// TODO(rmistry): Pub sub into swarming to get notified when a thingy is updated!
// TODO(rmistry): Send out email when tasks have expired. Do you need to send out an email when 15 mins are left asking them to clean stuff up? warning field in datastore maybe.
// TODO(rmistry): Need to extract everything out.

func checkForExpiredTasks() error {
	fmt.Println("Checking for expired tasks")
	keyToExpiredTask := map[*datastore.Key]*Task{}
	tasks := []*Task{}
	q := ds.NewQuery(TASK).EventualConsistency().Filter("Done =", false)
	// .Order("-Created")
	it := ds.DS.Run(context.TODO(), q)
	for {
		t := &Task{}
		k, err := it.Next(t)
		if err == iterator.Done {
			fmt.Println("Iterator is done")
			break
		} else if err != nil {
			return fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		fmt.Println("FOUND SOMETHING!")
		//t.ID = k.ID
		fmt.Printf("Found %d", k.ID)
		// if t.LeaseEndTime.Before(time.Now()) {  // This needs to be Before.
		fmt.Println(t.LeaseEndTime.Before(time.Now()))
		fmt.Println(t.LeaseEndTime.After(time.Now()))
		if t.LeaseEndTime.Before(time.Now()) {
			keyToExpiredTask[k] = t
			tasks = append(tasks, t)
			fmt.Println("It has expired!!!")
			fmt.Println(t)
			t.Done = true
		}
	}

	for k, t := range keyToExpiredTask {
		// Mark the task as expired.
		//key := ds.NewKey(TASK)
		// Mark the task as Done.
		t.Done = true
		if _, err := UpdateTask(k, t); err != nil {
			return fmt.Errorf("Error updating task in datastore: %v", err)
		}
		sklog.Infof("Marked as expired task %v in the datastore with key %d", t, k.ID)
	}

	return nil
}

func GetTask(taskID int64) (*datastore.Key, *Task, error) {
	key := ds.NewKey(TASK)
	key.ID = taskID

	task := &Task{}
	if err := ds.DS.Get(context.TODO(), key, task); err != nil {
		return nil, nil, fmt.Errorf("Error retrieving task from Datastore: %v", err)
	}
	return key, task, nil
}

type Status struct {
	TaskId  int64
	Expired bool
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

	k, t, err := GetTask(taskID)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not find task")
		return
	}

	// For debugging the retrieved task.
	fmt.Println("RETRIVED THIS TASK")
	fmt.Println(t)
	fmt.Println(t.Requester)
	fmt.Println(t.OsType)
	fmt.Println(t.DeviceType)
	fmt.Println(t.InitialDurationHrs)
	fmt.Println(t.Created)
	fmt.Println(t.LeaseEndTime)
	fmt.Println(t.Description)
	fmt.Println(t.BotId)
	fmt.Println(t.Done)

	status := Status{
		TaskId:  k.ID,
		Expired: t.Done,
	}
	if err := json.NewEncoder(w).Encode(status); err != nil {
		httputils.ReportError(w, r, err, "Failed to encode JSON")
		return

	}

	return
}

type poolDetails struct {
	OsTypes     map[string]int
	DeviceTypes map[string]int
}

func getPoolDetails() *poolDetails {
	// Testing the swarming API

	// TODO(rmistry): Extract this out.
	// TODO(Rmistry): When you display it make them all clickable links. Clicking will take you to the swarming page.
	bots, err := swarmingClient.ListBotsForPool("Skia")
	if err != nil {
		sklog.Fatalf("Could not list bots in pool: %s", err)
	}
	fmt.Println("BOTS IN SKIA POOL:")
	osTypes := map[string]int{}
	deviceTypes := map[string]int{}
	total := 0
	for _, bot := range bots {
		if bot.IsDead || bot.Quarantined {
			// Do not include dead/quarantined bots in the counts below.
			continue
		}
		for _, d := range bot.Dimensions {
			if d.Key == "os" {
				val := ""
				// Use the longest string from the os values because that is what the swarming UI
				// does and it works in all cases we have (atleast as of 10/20/17).
				for _, v := range d.Value {
					if len(v) > len(val) {
						val = v
					}
				}
				osTypes[val]++
			}
			if d.Key == "device_type" {
				// There should only be one value for device type.
				val := d.Value[0]
				deviceTypes[val]++
				total++ // TODO(Rmistry): Remove this later.
			}
		}
	}
	fmt.Println(osTypes)
	fmt.Println(deviceTypes)
	fmt.Println(total)
	// ------------------------
	return &poolDetails{
		OsTypes:     osTypes,
		DeviceTypes: deviceTypes,
	}
}

func poolDetailsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	poolDetails := getPoolDetails()
	if err := json.NewEncoder(w).Encode(poolDetails); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to encode JSON: %v", err))
		return
	}
}

type Task struct {
	Requester          string    `json:"requester"`
	OsType             string    `json:"osType"`
	DeviceType         string    `json:"deviceType"`
	InitialDurationHrs string    `json:"duration"`
	Created            time.Time `json:"created"`
	LeaseEndTime       time.Time `json:"leaseEndTime"`
	Description        string    `json:"description"`
	BotId              string    `json:"botId"`
	Done               bool      `json:"done"`
	SwarmingLogs       string    `json:"swarmingLogs"`
	TaskId             int64     `json:"taskId"`
}

type sortTasks []*Task

func (a sortTasks) Len() int      { return len(a) }
func (a sortTasks) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortTasks) Less(i, j int) bool {
	return a[i].Created.After(a[j].Created)
}

func getLeasingTasks(filterUser string) ([]*Task, error) {
	tasks := []*Task{}
	q := ds.NewQuery(TASK).EventualConsistency()
	if filterUser != "" {
		q = q.Filter("Requester =", filterUser)
	}
	it := ds.DS.Run(context.TODO(), q)
	for {
		t := &Task{}
		k, err := it.Next(t)
		if err == iterator.Done {
			fmt.Println("Iterator is done")
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to retrieve list of tasks: %s", err)
		}
		t.TaskId = k.ID
		tasks = append(tasks, t)
	}
	sort.Sort(sortTasks(tasks))

	return tasks, nil
}

func leasesHandlerHelper(w http.ResponseWriter, r *http.Request, filterUser string) {
	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")

	tasks, err := getLeasingTasks(filterUser)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to expand template")
		return
	}

	var templateTasks = struct {
		Tasks []*Task
	}{
		Tasks: tasks,
	}
	if err := leasesListTemplate.Execute(w, templateTasks); err != nil {
		httputils.ReportError(w, r, err, "Failed to expand template")
		return
	}
	return
}

func myLeasesHandler(w http.ResponseWriter, r *http.Request) {
	leasesHandlerHelper(w, r, login.LoggedInAs(r))
}

func allLeasesHandler(w http.ResponseWriter, r *http.Request) {
	leasesHandlerHelper(w, r, "" /* filterUser */)
}

func extendTaskHandler(w http.ResponseWriter, r *http.Request) {
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

	durationParam := r.FormValue("duration")
	if durationParam == "" {
		httputils.ReportError(w, r, nil, "Missing duration parameter")
		return
	}
	durationHrs, err := strconv.Atoi(durationParam)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to parse %s", durationParam))
		return
	}

	k, t, err := GetTask(taskID)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not find task")
		return
	}

	// Add duration hours to the task's lease end time only if ends up being
	// less than 23 hours after the task's creation time.
	newLeaseEndTime := t.LeaseEndTime.Add(time.Hour * time.Duration(durationHrs))
	maxPossibleLeaseEndTime := t.Created.Add(time.Hour * time.Duration(MAX_LEASE_DURATION_HRS))
	if newLeaseEndTime.After(maxPossibleLeaseEndTime) {
		httputils.ReportError(w, r, nil, fmt.Sprintf("Can not extend lease beyond %d hours of the task creation time", MAX_LEASE_DURATION_HRS))
		return
	}

	// Change the lease end time.
	t.LeaseEndTime = newLeaseEndTime
	if _, err := UpdateTask(k, t); err != nil {
		httputils.ReportError(w, r, err, "Error updating task in datastore")
		return
	}
}

func expireTaskHandler(w http.ResponseWriter, r *http.Request) {
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

	k, t, err := GetTask(taskID)
	if err != nil {
		httputils.ReportError(w, r, err, "Could not find task")
		return
	}

	// Change the task to Done and change the lease end time to now.
	t.LeaseEndTime = time.Now()
	t.Done = true
	if _, err := UpdateTask(k, t); err != nil {
		httputils.ReportError(w, r, err, "Error updating task in datastore")
		return
	}
}

func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	task := &Task{}

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add %T task", task))
		return
	}
	defer util.Close(r.Body)

	durationHrs, err := strconv.Atoi(task.InitialDurationHrs)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to parse %s", task.InitialDurationHrs))
		return
	}

	// Add the username of the requester.
	task.Requester = login.LoggedInAs(r)
	// Add the created time and calculate the lease end time.
	task.Created = time.Now()
	task.LeaseEndTime = time.Now().Add(time.Hour * time.Duration(durationHrs))
	// Populate deviceType only if Android is the osType.
	if task.OsType != "Android" {
		task.DeviceType = ""
	}

	context.Background()

	// For debugging the added task.
	fmt.Println("ADDING THIS TASK")
	fmt.Println(task)
	fmt.Println(task.Requester)
	fmt.Println(task.OsType)
	fmt.Println(task.DeviceType)
	fmt.Println(task.InitialDurationHrs)
	fmt.Println(task.Created)
	fmt.Println(task.LeaseEndTime)
	fmt.Println(task.Description)
	fmt.Println(task.BotId)
	fmt.Println(task.Done)

	key := ds.NewKey(TASK)
	datastoreKey, err := ds.DS.Put(context.Background(), key, task)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Error putting task in datastore: %v", err))

	}
	sklog.Infof("Added %v task into the datastore with key %s", task, datastoreKey)

	isolateHash, err := getIsolateHash(task.OsType)

	// TODO(rmistry): Schedule swarming task if these dimentions.

	//dimsMap := map[string]string{
	//	"os":          task.OsType,
	//	"device_type": task.DeviceType,
	//}
	// TODO(rmistry): This is just for testing.
	//dimsMap := map[string]string{
	//	"pool": "CT",
	//	"id":   "build1-m5",
	//}

	// TODO(rmistry): Change this. Fix this. Do something with this.
	rawDims := []string{"pool:Skia", "id:skia-gce-001"}

	dims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(rawDims))
	dimsMap := make(map[string]string, len(rawDims))
	for _, d := range rawDims {
		split := strings.SplitN(d, ":", 2)
		key := split[0]
		val := split[1]
		dims = append(dims, &swarming_api.SwarmingRpcsStringPair{
			Key:   key,
			Value: val,
		})
		dimsMap[key] = val
	}

	expirationSecs := int64(swarming.RECOMMENDED_EXPIRATION.Seconds())
	executionTimeoutSecs := int64(SWARMING_HARD_TIMEOUT.Seconds())
	ioTimeoutSecs := int64(swarming.RECOMMENDED_IO_TIMEOUT.Seconds())
	taskName := fmt.Sprintf("Leased by %s using leasing.skia.org", task.Requester)
	taskRequest := &swarming_api.SwarmingRpcsNewTaskRequest{
		ExpirationSecs: expirationSecs,
		Name:           taskName,
		Priority:       LEASE_TASK_PRIORITY,
		Properties: &swarming_api.SwarmingRpcsTaskProperties{
			//CipdInput:            cipdInput,
			Dimensions: dims,
			//Env:                  env,
			ExecutionTimeoutSecs: executionTimeoutSecs,
			//ExtraArgs:            extraArgs,  Will be needed later?
			InputsRef: &swarming_api.SwarmingRpcsFilesRef{
				Isolated:       isolateHash,
				Isolatedserver: isolate.ISOLATE_SERVER_URL,
				Namespace:      isolate.DEFAULT_NAMESPACE,
			},
			IoTimeoutSecs: ioTimeoutSecs,
		},
		//PubsubTopic:    fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, pubSubTopic),
		ServiceAccount: swarming.GetServiceAccountFromTaskDims(dimsMap),
		//Tags:           db.TagsForTask(c.Name, id, c.Attempt, c.TaskSpec.Priority, c.RepoState, c.RetryOf, dimsMap, c.ForcedJobId, c.ParentTaskIds),
		User: "skiabot@google.com",
	}
	//fmt.Println(taskRequest)

	resp, err := swarmingClient.TriggerTask(taskRequest)
	if err != nil {
		sklog.Fatalf("COULD NOT Trigger swarming task %s", err)
	}
	fmt.Println("This is the triggered task!")
	fmt.Println(resp.TaskId)
	fmt.Println("-----------------------------------------------")
	fmt.Println("-----------------------------------------------")
	fmt.Println("-----------------------------------------------")
	fmt.Println("-----------------------------------------------")
	fmt.Println("-----------------------------------------------")

	// TODO(rmistry): Email instructions after getting the swarming task so that you can link to it.
	// also should mention that if you need to extend you can (upto 23 hours of creation time). If you are done earlier than expected then you can you expire it earlier here.
}

func getIsolateHash(osType string) (string, error) {
	isolateTask := &isolate.Task{
		BaseDir:     "isolates",
		Blacklist:   []string{},
		Deps:        []string{},
		IsolateFile: path.Join("isolates", "leasing.isolate"),
		OsType:      osType,
	}
	isolateTasks := []*isolate.Task{isolateTask}
	hashes, err := isolateClient.IsolateTasks(isolateTasks)
	if err != nil {
		return "", fmt.Errorf("Could not isolate leasing task: %s")
	}
	if len(hashes) != 1 {
		return "", fmt.Errorf("IsolateTasks returned incorrect number of hashes %d (expected 1)", len(hashes))
	}
	return hashes[0], nil
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/", indexHandler)
	r.HandleFunc("/my_leases", myLeasesHandler)
	r.HandleFunc("/all_leases", allLeasesHandler)
	r.HandleFunc("/_/get_task_status", statusHandler) // Should this be POST? rmistry what is easier to do in the python script
	r.HandleFunc("/_/pooldetails", poolDetailsHandler).Methods("POST")
	r.HandleFunc("/_/add_leasing_task", addTaskHandler).Methods("POST")
	r.HandleFunc("/_/extend_leasing_task", extendTaskHandler).Methods("POST")
	r.HandleFunc("/_/expire_leasing_task", expireTaskHandler).Methods("POST")

	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	r.HandleFunc(OAUTH2_CALLBACK_PATH, login.OAuth2CallbackHandler)
	r.HandleFunc("/login/", loginHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on %s", serverURL)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}

func MailInit(tokenPath string) error {
	emailTokenPath := tokenPath
	emailClientId := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_ID))
	emailClientSecret := metadata.Must(metadata.ProjectGet(metadata.GMAIL_CLIENT_SECRET))
	cachedGMailToken := metadata.Must(metadata.ProjectGet(GMAIL_CACHED_TOKEN))
	if err := ioutil.WriteFile(emailTokenPath, []byte(cachedGMailToken), os.ModePerm); err != nil {
		return fmt.Errorf("Failed to cache token: %s", err)
	}
	gmail, err := email.NewGMail(emailClientId, emailClientSecret, emailTokenPath)
	if err != nil {
		return fmt.Errorf("Could not initialize gmail object: %s", err)
	}
	//if err := gmail.Send("Cluster Telemetry", recipients, subject, body); err != nil {
	//	return fmt.Errorf("Could not send email: %s", err)
	//}
	fmt.Println(gmail)
	return nil
}

func getSortedStringFromValues(values []string) string {
	fmt.Println("Unsorted:")
	fmt.Println(values)
	sort.Strings(values)
	fmt.Println(values)
	return ""
}

// TODO(rmistry):
// * Call swarming API.
// * Write to datastore.
// * Email.
func main() {
	defer common.LogPanic()

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	common.InitWithMust("leasing", common.PrometheusOpt(promPort))
	// common.InitWithMust("leasing", common.PrometheusOpt(promPort), common.CloudLoggingOpt())
	v, err := skiaversion.GetVersion()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Version %s, built at %s", v.Commit, v.Date)

	Init()
	serverURL := "https://" + *host
	if *local {
		serverURL = "http://" + *host + *port
	}

	// TODO(rmistry): Unable this.
	//usr, err := user.Current()
	//if err != nil {
	//	sklog.Fatal(err)
	//}
	//MailInit(filepath.Join(usr.HomeDir, "email.data"))

	useRedirectURL := fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		useRedirectURL = *redirectURL
	}
	if err := login.Init(useRedirectURL, *authWhiteList); err != nil {
		sklog.Fatal(fmt.Errorf("Problem setting up server OAuth: %s", err))
	}

	isolateClient, err = isolate.NewClient(*workdir, isolate.ISOLATE_SERVER_URL)
	if err != nil {
		sklog.Fatalf("Failed to create isolate client: %s", err)
	}

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(*workdir, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	// Swarming API client.
	swarmingClient, err = swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}

	// Initialize cloud datastore.
	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init cloud datastore: %s", err)
	}

	healthyGauge := metrics2.GetInt64Metric("healthy")
	go func() {
		for range time.Tick(*pollInterval) {
			healthyGauge.Update(1)
			if err := checkForExpiredTasks(); err != nil {
				sklog.Errorf("Error when checking for expired tasks: %v", err)
			}
		}
	}()

	// rmistry experiment is here:

	//rawDims := []string{"pool:Skia", "id:skia-gce-001"}

	//dims := make([]*swarming_api.SwarmingRpcsStringPair, 0, len(rawDims))
	//dimsMap := make(map[string]string, len(rawDims))
	//for _, d := range rawDims {
	//	split := strings.SplitN(d, ":", 2)
	//	key := split[0]
	//	val := split[1]
	//	dims = append(dims, &swarming_api.SwarmingRpcsStringPair{
	//		Key:   key,
	//		Value: val,
	//	})
	//	dimsMap[key] = val
	//}

	//expirationSecs := int64(swarming.RECOMMENDED_EXPIRATION.Seconds())
	//executionTimeoutSecs := int64(SWARMING_HARD_TIMEOUT.Seconds())
	//ioTimeoutSecs := int64(swarming.RECOMMENDED_IO_TIMEOUT.Seconds())
	//taskName := fmt.Sprintf("Leased by %s using leasing.skia.org", "rmistry") // TODO(rmistry): Change to task requester.
	//taskRequest := &swarming_api.SwarmingRpcsNewTaskRequest{
	//	ExpirationSecs: expirationSecs,
	//	Name:           taskName,
	//	Priority:       LEASE_TASK_PRIORITY,
	//	Properties: &swarming_api.SwarmingRpcsTaskProperties{
	//		//CipdInput:            cipdInput,
	//		Dimensions: dims,
	//		//Env:                  env,
	//		ExecutionTimeoutSecs: executionTimeoutSecs,
	//		//ExtraArgs:            extraArgs,  Will be needed later?
	//		InputsRef: &swarming_api.SwarmingRpcsFilesRef{
	//			Isolated:       isolateHash,
	//			Isolatedserver: isolate.ISOLATE_SERVER_URL,
	//			Namespace:      isolate.DEFAULT_NAMESPACE,
	//		},
	//		IoTimeoutSecs: ioTimeoutSecs,
	//	},
	//	ServiceAccount: swarming.GetServiceAccountFromTaskDims(dimsMap),
	//	User:           "skiabot@google.com",
	//}
	//fmt.Println(taskRequest)
	//resp, err := swarmingClient.TriggerTask(taskRequest)
	//if err != nil {
	//	sklog.Fatalf("COULD NOT Trigger swarming task %s", err)
	//}
	//fmt.Println("This is the triggered task!")
	//fmt.Println(resp.TaskId)

	//// rmistry experiment is done.

	//fmt.Println(serverURL)
	runServer(serverURL)
}
