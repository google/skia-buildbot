package main

import (
	"context"
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

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/systemd"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/push/go/types"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// dbc is the dbus connection we use to talk to systemd.
	dbc *dbus.Conn

	hostname = ""

	ACTIONS = []string{string(types.Start), string(types.Stop), string(types.Restart)}

	// triggerPullCh triggers a pull when sent a boolean value.
	triggerPullCh = make(chan bool, 1)

	// PROCESS_ENDING_UNITS include those that are likely to cause the
	// current process to end.
	PROCESS_ENDING_UNITS = []string{"reboot.target", "pulld.service"}
)

const (
	EXEC_MAIN_START_TS_PROP = "ExecMainStartTimestamp"
)

// flags
var (
	bucketName            = flag.String("bucket_name", "skia-push", "The name of the Google Storage bucket that contains push packages and info.")
	installedPackagesFile = flag.String("installed_packages_file", "installed_packages.json", "Path to the file where to cache the list of installed debs.")
	local                 = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	onGCE                 = flag.Bool("on_gce", true, "Running on GCE.  Could be running on some external machine, e.g. in the Skolo.")
	port                  = flag.String("port", ":10000", "HTTP service address (e.g., ':8000')")
	promPort              = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir          = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	pullPeriod            = flag.Duration("pull_period", 15*time.Second, "How often to check the configuration. On GCE, the metadata update will likely happen first")
)

type UnitStatusSlice []*systemd.UnitStatus

func (p UnitStatusSlice) Len() int           { return len(p) }
func (p UnitStatusSlice) Less(i, j int) bool { return p[i].Status.Name < p[j].Status.Name }
func (p UnitStatusSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func loadResouces() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}
}

// ChangeResult is the serialized JSON response from changeHandler.
type ChangeResult struct {
	Result string `json:"result"`
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

// getFunctionForAction returns StartUnit, StopUnit, or RestartUnit based on action.
func getFunctionForAction(action types.Action) func(name string, mode string, ch chan<- string) (int, error) {
	switch action {
	case types.Start:
		return dbc.StartUnit
	case types.Stop:
		return dbc.StopUnit
	case types.Restart:
		return dbc.RestartUnit
	default:
		sklog.Fatalf("%q in ACTIONS but not handled by getFunctionForAction", action)
		return nil
	}
}

// changeHandler changes the status of a service.
//
// Takes the following query parameters:
//
//   name - The name of the service.
//   action - The action to perform.
//
// The response is of the form:
//
//   {
//     "result": "started"
//   }
//
func changeHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Failed to parse form.", http.StatusInternalServerError)
		return
	}
	action := r.Form.Get("action")
	if !util.In(action, ACTIONS) {
		httputils.ReportError(w, fmt.Errorf("Not a valid action: %s", action), "Invalid action.", http.StatusInternalServerError)
		return
	}
	name := r.Form.Get("name")
	if name == "" {
		httputils.ReportError(w, fmt.Errorf("Not a valid service name: %s", name), "Invalid service name.", http.StatusInternalServerError)
		return
	}
	if *local {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(ChangeResult{"started"}); err != nil {
			sklog.Errorf("Failed to write or encode output: %s", err)
		}
		return
	}
	cmd := &types.Command{
		Action:  types.Action(action),
		Service: name,
	}
	res := ChangeResult{}
	res.Result = executeCommand(cmd)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

// executeCommand executes the given Command and returns a status message.
func executeCommand(cmd *types.Command) string {
	if cmd.Action == types.Pull {
		triggerPullCh <- true
		return "pull enqueued"
	} else {
		f := getFunctionForAction(cmd.Action)
		if util.In(cmd.Service, PROCESS_ENDING_UNITS) {
			go func() {
				<-time.After(1 * time.Second)
				if _, err := f(cmd.Service, "replace", nil); err != nil {
					sklog.Error(err)
				}
			}()
			return "enqueued"
		} else {
			ch := make(chan string)
			if _, err := f(cmd.Service, "replace", ch); err != nil {
				sklog.Errorf("Action failed: %s", err)
				return "action failed"
			}
			return <-ch
		}
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
		return nil, fmt.Errorf("Failed to list units: %s", err)
	}
	if !*local {
		units = filterUnits(units)
	}
	// Now fill in the Props for each unit.
	for _, unit := range units {
		props, err := dbc.GetUnitTypeProperties(unit.Status.Name, "Service")
		if err != nil {
			sklog.Errorf("Failed to get props for the unit %s: %s", unit.Status.Name, err)
			continue
		}
		// Props are huge, only pass along the value(s) we use.
		unit.Props = map[string]interface{}{
			EXEC_MAIN_START_TS_PROP: props[EXEC_MAIN_START_TS_PROP],
		}
	}
	return units, nil
}

// listHandler returns the list of units.
func listHandler(w http.ResponseWriter, r *http.Request) {
	units, err := listUnits()
	if err != nil {
		httputils.ReportError(w, err, "Failed to list units.", http.StatusInternalServerError)
		return
	}
	resp := &types.ListResponse{
		Hostname: hostname,
		Units:    units,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
	}
}

func main() {
	flag.Parse()
	common.InitWithMust(
		"pulld",
		common.PrometheusOpt(promPort),
		common.CloudLoggingDefaultAuthOpt(local),
	)
	tokenSource, err := auth.NewDefaultTokenSource(*local)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client()

	Init()
	ctx := context.Background()
	pullInit(ctx, client, triggerPullCh)
	rebootMonitoringInit()

	r := mux.NewRouter()
	r.HandleFunc("/_/list", listHandler).Methods("GET")
	r.HandleFunc("/_/change", changeHandler).Methods("POST")
	r.PathPrefix("/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
