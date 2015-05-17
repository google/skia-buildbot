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
	port           = flag.String("port", ":10116", "HTTP service address (e.g., ':8000')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

type UnitSlice []dbus.UnitStatus

func (p UnitSlice) Len() int           { return len(p) }
func (p UnitSlice) Less(i, j int) bool { return p[i].Name < p[j].Name }
func (p UnitSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

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
func serviceOnly(units []dbus.UnitStatus) []dbus.UnitStatus {
	ret := []dbus.UnitStatus{}
	for _, u := range units {
		if strings.HasSuffix(u.Name, ".service") {
			ret = append(ret, u)
		}
	}
	return ret
}

// listHandler returns the list of units.
func listHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("List Handler: %q\n", r.URL.Path)
	units, err := dbc.ListUnits()
	if err != nil {
		if *local {
			// If running locally the above will fail because we aren't on systemd
			// yet, so return some dummy data.
			units = []dbus.UnitStatus{
				dbus.UnitStatus{
					Name:     "test.service",
					SubState: "running",
				},
				dbus.UnitStatus{
					Name:     "something.service",
					SubState: "halted",
				},
			}
		} else {
			util.ReportError(w, r, err, "Failed to list units.")
			return
		}
	}
	units = serviceOnly(units)
	sort.Sort(UnitSlice(units))
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(units); err != nil {
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
	common.InitWithMetrics("sksysmon", graphiteServer)
	Init()

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler).Methods("GET")
	r.HandleFunc("/_/list", listHandler).Methods("GET")
	r.HandleFunc("/_/change", changeHandler).Methods("POST")
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
