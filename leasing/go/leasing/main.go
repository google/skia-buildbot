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
	"time"
	//"os/user"
	"path/filepath"
	"runtime"

	//"cloud.google.com/go/datastore"
	"github.com/gorilla/mux"

	"go.skia.org/infra/ct/go/db"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/email"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
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
)

var (
	// flags
	host         = flag.String("host", "localhost", "HTTP service host")
	promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
	port         = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	workdir      = flag.String("workdir", ".", "Directory to use for scratch work.")
	resourcesDir = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")

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
}

func Init() {
	reloadTemplates()
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, login.LoginURL(w, r), http.StatusFound)
	return
}

// TODO(rmistry): Use templates...
func testHandler(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	//w.Write([]byte("Testing"))

	if *local {
		reloadTemplates()
	}
	w.Header().Set("Content-Type", "text/html")
	var osTypes = struct {
		OsTypes []string
	}{
		// OsTypes: mux.Vars(r)["category"],
		OsTypes: []string{"testing1", "testing2"},
	}

	if err := indexTemplate.Execute(w, osTypes); err != nil {
		sklog.Errorf("Failed to expand template: %v", err)
	}
	return
}

type poolDetails struct {
	OsTypes     map[string]int
	DeviceTypes map[string]int
}

func getPoolDetails() *poolDetails {
	// Testing the swarming API

	// Authenticated HTTP client.
	oauthCacheFile := path.Join(*workdir, "google_storage_token.data")
	httpClient, err := auth.NewClient(*local, oauthCacheFile, swarming.AUTH_SCOPE)
	if err != nil {
		sklog.Fatal(err)
	}
	// Swarming API client.
	swarmingApi, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER)
	if err != nil {
		sklog.Fatal(err)
	}

	// TODO(rmistry): Extract this out.
	// TODO(Rmistry): When you display it make them all clickable links. Clicking will take you to the swarming page.
	bots, err := swarmingApi.ListBotsForPool("Skia")
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
	Requester    string
	OsType       string `json:"osType"`
	DeviceType   string `json:"deviceType"`
	DurationHrs  string `json:"duration"`
	Created      time.Time
	LeaseEndTime time.Time
	Description  string `json:"description"`
	BotId        string
	Done         bool
}

// Handle max end time somehow. Maybe in the update handler.
func addTaskHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	task := &Task{}

	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to add %T task", task))
		return
	}
	defer util.Close(r.Body)

	durationHrs, err := strconv.Atoi(task.DurationHrs)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to parse %s", task.DurationHrs))
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
	fmt.Println(task)
	fmt.Println(task.Requester)
	fmt.Println(task.OsType)
	fmt.Println(task.DeviceType)
	fmt.Println(task.DurationHrs)
	fmt.Println(task.Created)
	fmt.Println(task.LeaseEndTime)
	fmt.Println(task.Description)
	fmt.Println(task.BotId)
	fmt.Println(task.Done)

	key := ds.NewKey("Task")
	datastoreKey, err := ds.DS.Put(context.Background(), key, task)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Error putting task in datastore: %v", err))

	}
	sklog.Infof("Added %v task into the datastore with key %s", task, datastoreKey)
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))

	r.HandleFunc("/test", testHandler)
	r.HandleFunc("/_/pooldetails", poolDetailsHandler).Methods("POST")
	r.HandleFunc("/_/add_leasing_task", addTaskHandler).Methods("POST")

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
	// Setup flags.
	dbConf := db.DBConfigFromFlags()

	// Don't use cached templates in local mode.
	if *local {
		reloadTemplates()
	}

	common.InitWithMust("leasing")
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

	// Initialize the ctfe database.
	fmt.Println(dbConf)
	//if !*local {
	//	if err := dbConf.GetPasswordFromMetadata(); err != nil {
	//		sklog.Fatal(err)
	//	}
	//}
	//if err := dbConf.InitDB(); err != nil {
	//	sklog.Fatal(err)
	//}

	if err := ds.Init(*projectName, *namespace); err != nil {
		sklog.Fatalf("Failed to init Cloud Datastore: %s", err)
	}

	runServer(serverURL)
}
