package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/go-systemd/dbus"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/systemd"
	"go.skia.org/infra/go/util"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// dbc is the dbus connection we use to talk to systemd.
	dbc *dbus.Conn

	hostname = ""

	ACTIONS = []string{"start", "stop", "restart"}
)

// flags
var (
	installedPackagesFile = flag.String("installed_packages_file", "installed_packages.json", "Path to the file where to cache the list of installed debs.")
	local                 = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	onGCE                 = flag.Bool("on_gce", true, "Running on GCE.  Could be running on some external machine, e.g. in the Skolo.")
	port                  = flag.String("port", ":10114", "HTTP service address (e.g., ':8000')")
	resourcesDir          = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	serviceAccountPath    = flag.String("service_account_path", "", "Path to the service account.  Can be empty string to use defaults or project metadata")
	bucketName            = flag.String("bucket_name", "skia-push", "The name of the Google Storage bucket that contains push packages and info.")

	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")

	cloudLoggingGroup = flag.String("cloud_logging_group", "rpi-master", "The log grouping to be used if cloud logging is configured (i.e. on_gce is false)")
)

type UnitStatusSlice []*systemd.UnitStatus

func (p UnitStatusSlice) Len() int           { return len(p) }
func (p UnitStatusSlice) Less(i, j int) bool { return p[i].Status.Name < p[j].Status.Name }
func (p UnitStatusSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func loadResouces() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	indexTemplate = template.Must(template.New("").Delims("{%", "%}").ParseFiles(
		filepath.Join(*resourcesDir, "templates", "index.html"),
		filepath.Join(*resourcesDir, "templates", "titlebar.html"),
		filepath.Join(*resourcesDir, "templates", "header.html"),
	))
}

// ChangeResult is the serialized JSON response from changeHandler.
type ChangeResult struct {
	Result string `json:"result"`
}

func initLogging() {
	if !*onGCE {
		client, err := auth.NewJWTServiceAccountClient("", *serviceAccountPath, nil, sklog.CLOUD_LOGGING_WRITE_SCOPE)
		if err != nil {
			sklog.Fatalf("Could not setup credentials: %s", err)
		}

		err = sklog.InitCloudLogging(client, *cloudLoggingGroup, "pulld-not-gce")
		if err != nil {
			sklog.Fatalf("Could not setup cloud logging: %s", err)
		}
	}
}

func Init() {
	var err error
	dbc, err = dbus.New()
	if err != nil {
		sklog.Fatalf("Failed to initialize dbus: %s", err)
	}

	hostname, err = os.Hostname()
	if err != nil {
		sklog.Fatalf("Unable to retrieve hostname: %s", err)
	}
	packages.SetBucketName(*bucketName)
	loadResouces()
}

// changeHandler changes the status of a service.
//
// Takes the following query parameters:
//
//   name - The name of the service.
//   action - The action to perform. One of ["start", "stop", "restart"].
//
// The response is of the form:
//
//   {
//     "result": "started"
//   }
//
func changeHandler(w http.ResponseWriter, r *http.Request) {
	var err error

	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form.")
		return
	}
	action := r.Form.Get("action")
	if !util.In(action, ACTIONS) {
		httputils.ReportError(w, r, fmt.Errorf("Not a valid action: %s", action), "Invalid action.")
		return
	}
	name := r.Form.Get("name")
	if name == "" {
		httputils.ReportError(w, r, fmt.Errorf("Not a valid service name: %s", name), "Invalid service name.")
		return
	}
	if *local {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ChangeResult{"started"}); err != nil {
			sklog.Errorf("Failed to write or encode output: %s", err)
		}
		return
	}
	ch := make(chan string)
	switch action {
	case "start":
		_, err = dbc.StartUnit(name, "replace", ch)
	case "stop":
		_, err = dbc.StopUnit(name, "replace", ch)
	case "restart":
		_, err = dbc.RestartUnit(name, "replace", ch)
	}
	if err != nil {
		httputils.ReportError(w, r, err, "Action failed.")
		return
	}
	res := ChangeResult{}
	res.Result = <-ch
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// serviceOnly returns only units that are services.
func serviceOnly(units []*systemd.UnitStatus) []*systemd.UnitStatus {
	ret := []*systemd.UnitStatus{}
	for _, u := range units {
		if strings.HasSuffix(u.Status.Name, ".service") {
			ret = append(ret, u)
		}
	}
	return ret
}

// filterService returns only units with names in filter.
func filterService(units []*systemd.UnitStatus, filter map[string]bool) []*systemd.UnitStatus {
	ret := []*systemd.UnitStatus{}
	for _, u := range units {
		if filter[u.Status.Name] {
			ret = append(ret, u)
		}
	}
	return ret
}

// filterUnits fitlers down the units to only the interesting ones.
func filterUnits(units []*systemd.UnitStatus) []*systemd.UnitStatus {
	units = serviceOnly(units)
	sort.Sort(UnitStatusSlice(units))

	// Filter the list down to just services installed by push packages.
	installedPackages, err := packages.FromLocalFile(*installedPackagesFile)
	if err != nil {
		return units
	}
	allPackages, err := packages.AllAvailableByPackageName(store)
	if err != nil {
		return units
	}
	allServices := map[string]bool{}
	for _, p := range installedPackages {
		for _, name := range allPackages[p].Services {
			allServices[name] = true
		}
	}
	return filterService(units, allServices)
}

func listUnits() ([]*systemd.UnitStatus, error) {
	unitStatus, err := dbc.ListUnits()
	units := make([]*systemd.UnitStatus, 0, len(unitStatus))
	if err == nil {
		for _, st := range unitStatus {
			cpst := st
			units = append(units, &systemd.UnitStatus{
				Status: &cpst,
			})
		}
	} else {
		if *local {
			// If running locally the above will fail because we aren't on systemd
			// yet, so return some dummy data.
			units = []*systemd.UnitStatus{
				&systemd.UnitStatus{
					Status: &dbus.UnitStatus{
						Name:     "test.service",
						SubState: "running",
					},
					Props: map[string]interface{}{
						"ExecMainStartTimestamp": time.Now().Add(-5*time.Minute).Unix() * 1000000,
					},
				},
				&systemd.UnitStatus{
					Status: &dbus.UnitStatus{
						Name:     "something.service",
						SubState: "halted",
					},
					Props: map[string]interface{}{
						"ExecMainStartTimestamp": time.Now().Add(-2*time.Hour).Unix() * 1000000,
					},
				},
			}
		} else {
			return nil, fmt.Errorf("Failed to list units: %s", err)
		}
	}
	if !*local {
		units = filterUnits(units)
		// Now fill in the Props for each unit.
		var err error
		for _, unit := range units {
			unit.Props, err = dbc.GetUnitTypeProperties(unit.Status.Name, "Service")
			if err != nil {
				sklog.Errorf("Failed to get props for the unit %s: %s", unit.Status.Name, err)
			}
		}
	}
	return units, nil
}

// listHandler returns the list of units.
func listHandler(w http.ResponseWriter, r *http.Request) {
	units, err := listUnits()
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to list units.")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(units); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// IndexBody is the context for evaluating the index.html template.
type IndexBody struct {
	Hostname string
	Units    []*systemd.UnitStatus
}

// mainHandler handles the GET of the main page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if *local {
			loadResouces()
		}
		units, err := listUnits()
		if err != nil {
			httputils.ReportError(w, r, err, "Failed to list units.")
			return
		}
		context := &IndexBody{
			Hostname: hostname,
			Units:    units,
		}
		w.Header().Set("Content-Type", "text/html")
		if err := indexTemplate.ExecuteTemplate(w, "index.html", context); err != nil {
			sklog.Errorln("Failed to expand template:", err)
		}
	}
}

func main() {
	defer common.LogPanic()
	flag.Parse()
	initLogging()
	if *onGCE {
		common.InitWithMetrics2("pulld", influxHost, influxUser, influxPassword, influxDatabase, local)
	} else {
		common.Init()
	}
	Init()
	pullInit(*serviceAccountPath)
	rebootMonitoringInit()

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler).Methods("GET")
	r.HandleFunc("/_/list", listHandler).Methods("GET")
	r.HandleFunc("/_/change", changeHandler).Methods("POST")
	r.HandleFunc("/pullpullpull", pullHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
