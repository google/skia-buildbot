package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/coreos/go-systemd/dbus"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/util"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// dbc is the dbus connection we use to talk to systemd.
	dbc *dbus.Conn

	ACTIONS = []string{"start", "stop", "restart"}
)

// flags
var (
	doOauth               = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	graphiteServer        = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	installedPackagesFile = flag.String("installed_packages_file", "installed_packages.json", "Path to the file where to cache the list of installed debs.")
	local                 = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	oauthCacheFile        = flag.String("oauth_cache_file", "google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	port                  = flag.String("port", ":10114", "HTTP service address (e.g., ':8000')")
	resourcesDir          = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

// UnitStatus is serialized to JSON in the return of propsHandler.
type UnitStatus struct {
	// Status is the current status of the unit.
	Status *dbus.UnitStatus `json:"status"`

	// Props is the set of unit properties returned from GetUnitTypeProperties.
	Props map[string]interface{} `json:"props"`
}

type UnitStatusSlice []*UnitStatus

func (p UnitStatusSlice) Len() int           { return len(p) }
func (p UnitStatusSlice) Less(i, j int) bool { return p[i].Status.Name < p[j].Status.Name }
func (p UnitStatusSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func loadResouces() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func Init() {
	var err error
	dbc, err = dbus.New()
	if err != nil {
		glog.Fatalf("Failed to initialize dbus: %s", err)
	}
	loadResouces()
}

// ChangeResult is the serialized JSON response from changeHandler.
type ChangeResult struct {
	Result string `json:"result"`
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
	glog.Infof("Change Handler: %q\n", r.URL.Path)
	var err error

	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse form.")
		return
	}
	action := r.Form.Get("action")
	if !util.In(action, ACTIONS) {
		util.ReportError(w, r, fmt.Errorf("Not a valid action: %s", action), "Invalid action.")
		return
	}
	name := r.Form.Get("name")
	if name == "" {
		util.ReportError(w, r, fmt.Errorf("Not a valid service name: %s", name), "Invalid service name.")
		return
	}
	if *local {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ChangeResult{"started"}); err != nil {
			util.ReportError(w, r, err, "Failed to encode response.")
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
		util.ReportError(w, r, err, "Action failed.")
		return
	}
	res := ChangeResult{}
	res.Result = <-ch
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		util.ReportError(w, r, err, "Failed to encode response.")
	}
}

// serviceOnly returns only units that are services.
func serviceOnly(units []*UnitStatus) []*UnitStatus {
	ret := []*UnitStatus{}
	for _, u := range units {
		if strings.HasSuffix(u.Status.Name, ".service") {
			ret = append(ret, u)
		}
	}
	return ret
}

// filterService returns only units with names in filter.
func filterService(units []*UnitStatus, filter map[string]bool) []*UnitStatus {
	ret := []*UnitStatus{}
	for _, u := range units {
		if filter[u.Status.Name] {
			ret = append(ret, u)
		}
	}
	return ret
}

// filterUnits fitlers down the units to only the interesting ones.
func filterUnits(units []*UnitStatus) []*UnitStatus {
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

// serviceByName returns the status for the named unit.
func serviceByName(units []dbus.UnitStatus, name string) *dbus.UnitStatus {
	for _, u := range units {
		if u.Name == name {
			return &u
		}
	}
	return nil
}

// listHandler returns the list of units.
func listHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("List Handler: %q\n", r.URL.Path)
	unitStatus, err := dbc.ListUnits()
	units := make([]*UnitStatus, 0, len(unitStatus))
	for _, st := range unitStatus {
		units = append(units, &UnitStatus{
			Status: &st,
		})
	}

	if err != nil {
		if *local {
			// If running locally the above will fail because we aren't on systemd
			// yet, so return some dummy data.
			units = []*UnitStatus{
				&UnitStatus{
					Status: &dbus.UnitStatus{
						Name:     "test.service",
						SubState: "running",
					},
				},
				&UnitStatus{
					Status: &dbus.UnitStatus{
						Name:     "something.service",
						SubState: "halted",
					},
				},
			}
		} else {
			util.ReportError(w, r, err, "Failed to list units.")
			return
		}
	}
	if !*local {
		units = filterUnits(units)
		// Now fill in the Props for each unit.
		var err error
		for _, unit := range units {
			unit.Props, err = dbc.GetUnitTypeProperties(unit.Status.Name, "Service")
			if err != nil {
				glog.Errorf("Failed to get props for the unit %s: %s", unit.Status.Name, err)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(units); err != nil {
		util.ReportError(w, r, err, "Failed to encode response.")
	}
}

// propsHandler returns the properties of the requested service unit.
//
// Query parameters:
//
//    service - The name of the service, such as "pulld.service".
//
func propsHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Properties Handler: %q\n", r.URL.Path)
	var err error

	if err := r.ParseForm(); err != nil {
		util.ReportError(w, r, err, "Failed to parse form.")
		return
	}
	service := r.Form.Get("service")
	props, err := dbc.GetUnitTypeProperties(service, "Service")
	if err != nil {
		util.ReportError(w, r, err, "Failed to list properties.")
	}
	units, err := dbc.ListUnits()
	if err != nil {
		util.ReportError(w, r, err, "Failed to list unit status.")
	}
	status := serviceByName(units, service)

	ret := UnitStatus{
		Status: status,
		Props:  props,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(ret); err != nil {
		util.ReportError(w, r, err, "Failed to encode response.")
	}
}

// mainHandler handles the GET of the main page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Main Handler: %q\n", r.URL.Path)
	if r.Method == "GET" {
		if *local {
			loadResouces()
		}
		w.Header().Set("Content-Type", "text/html")
		if err := indexTemplate.Execute(w, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

func main() {
	common.InitWithMetrics("pulld", graphiteServer)
	Init()
	pullInit()

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler).Methods("GET")
	r.HandleFunc("/_/list", listHandler).Methods("GET")
	r.HandleFunc("/_/props", propsHandler).Methods("GET")
	r.HandleFunc("/_/change", changeHandler).Methods("POST")
	r.HandleFunc("/pullpullpull", pullHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
