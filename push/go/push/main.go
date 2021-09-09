// push is the web server for pushing debian packages.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/chatbot"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/systemd"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/push/go/trigger"
	"go.skia.org/infra/push/go/types"
	compute "google.golang.org/api/compute/v1"
	storage "google.golang.org/api/storage/v1"
)

const (
	CHAT_MSG = `%s pushed %s to %s`
)

// flags
var (
	bucketName     = flag.String("bucket_name", "skia-push", "The name of the Google Storage bucket that contains push packages and info.")
	configFilename = flag.String("config_filename", "skiapush.json5", "Config filename.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	project        = flag.String("project", "google.com:skia-buildbots", "The Google Compute Engine project.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	logging        = flag.Bool("logging", true, "If true then log to stderr.")
)

// NewTimeoutClient creates a new http.Client with both a dial timeout and a
// request timeout.
func NewFastTimeoutClient() *http.Client {
	return httputils.NewConfiguredTimeoutClient(10*time.Second, 2*time.Second)
}

// Server is the state of the application.
type Server struct {
	// config is the configuration of what servers and apps we are managing.
	config packages.PackageConfig

	// ip keeps an updated map from server name to public IP address.
	ip *Zones

	// serverNames is a list of server names (GCE DNS names) we are managing.
	// Extracted from 'config'.
	serverNames []string

	// client is an HTTP client authorized to read and write gs://skia-push.
	client *http.Client

	// fastClient is an HTTP client that is unauthorized and fails quickly.
	fastClient *http.Client

	// store is an Google Storage API client authorized to read and write gs://skia-push.
	store *storage.Service

	// comp is an Google Compute API client authorized to read compute information.
	comp *compute.Service

	// packageInfo is a cache of info about packages.
	packageInfo *packages.AllInfo

	// mutex protects currentStatus
	mutex sync.Mutex

	// The current status of all the units.
	currentStatus map[string]*systemd.UnitStatus
}

// newServer creates a new *Server object.
func newServer() *Server {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../../dist")
	}

	var err error
	config, err := packages.LoadPackageConfig(*configFilename)
	if err != nil {
		sklog.Fatalf("Failed to load PackageConfig file: %s", err)
	}

	serverNames := config.AllServerNames()

	ts, err := auth.NewDefaultTokenSource(*local, auth.ScopeFullControl, auth.ScopeCompute)
	if err != nil {
		sklog.Fatalf("Failed to create authenticated HTTP client token source: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	fastClient := NewFastTimeoutClient()

	store, err := storage.New(client)
	if err != nil {
		sklog.Fatalf("Failed to create storage service client: %s", err)
	}
	comp, err := compute.New(client)
	if err != nil {
		sklog.Fatalf("Failed to create compute service client: %s", err)
	}
	ip, err := NewZones(comp)
	if err != nil {
		sklog.Fatalf("Failed to load IP addresses at startup: %s", err)
	}

	packages.SetBucketName(*bucketName)
	packageInfo, err := packages.NewAllInfo(client, store, serverNames)
	if err != nil {
		sklog.Fatalf("Failed to create packages.AllInfo at startup: %s", err)
	}

	chatbot.Init("push.skia.org")

	return &Server{
		config:        config,
		ip:            ip,
		serverNames:   serverNames,
		client:        client,
		fastClient:    fastClient,
		store:         store,
		comp:          comp,
		packageInfo:   packageInfo,
		currentStatus: map[string]*systemd.UnitStatus{},
	}

}

// Zones keeps track of the zone of each server.
type Zones struct {
	zone  map[string]string
	comp  *compute.Service
	mutex sync.Mutex
}

func (i *Zones) load() error {
	zoneMap := map[string]string{}
	zones, err := i.comp.Zones.List(*project).Do()
	if err != nil {
		return fmt.Errorf("Failed to list zones: %s", err)
	}
	for _, zone := range zones.Items {
		sklog.Infof("Zone: %s", zone.Name)
		list, err := i.comp.Instances.List(*project, zone.Name).Do()
		if err != nil {
			return fmt.Errorf("Failed to list instances: %s", err)
		}
		for _, item := range list.Items {
			zoneMap[item.Name] = zone.Name
		}
	}
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.zone = zoneMap
	return nil
}

func (i *Zones) Zone(server string) string {
	return i.zone[server]
}

func NewZones(comp *compute.Service) (*Zones, error) {
	i := &Zones{
		comp: comp,
	}
	if err := i.load(); err != nil {
		return nil, err
	}
	go func() {
		for range time.Tick(time.Second * 60) {
			if err := i.load(); err != nil {
				sklog.Infof("Error refreshing IP address list: %s", err)
			}
		}
	}()

	return i, nil
}

// ServerUI is used in ServersUI.
type ServerUI struct {
	// Name is the name of the server.
	Name string

	// Installed is a list of package names.
	Installed []string
}

// ServersUI is the format for data sent to the UI as JSON.
// It is a list of ServerUI's.
type ServersUI []*ServerUI

// PushNewPackage is the form of the JSON requests we receive
// from the UI to push a package.
type PushNewPackage struct {
	// Name is the unique package id, such as 'pull/pull:jcgregori....'.
	Name string `json:"name"`

	// Server is the GCE name of the server.
	Server string `json:"server"`
}

// getStatus returns a populated []*systemd.UnitStatus for the given server, one for each
// push managed service, and nil if the information wasn't able to be retrieved.
func (s *Server) getStatus(server string) []*systemd.UnitStatus {
	resp, err := s.fastClient.Get(fmt.Sprintf("http://%s:10000/_/list", server))
	if err != nil {
		sklog.Infof("Failed to get status of: %s", server)
		return nil
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		sklog.Infof("Bad status code: %d %s", resp.StatusCode, server)
		return nil
	}
	dec := json.NewDecoder(resp.Body)

	var ret types.ListResponse
	if err := dec.Decode(&ret); err != nil {
		sklog.Infof("Failed to decode: %s", err)
		return nil
	}
	return ret.Units
}

// serviceStatus returns a map[string]*systemd.UnitStatus, with one entry for each service running on each
// server. The keys for the return value are "<server_name>:<service_name>", for example,
// "skia-push:logserverd.service".
func (s *Server) serviceStatus(servers ServersUI) map[string]*systemd.UnitStatus {
	var mutex sync.Mutex
	var wg sync.WaitGroup
	ret := map[string]*systemd.UnitStatus{}
	for _, server := range servers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()
			allServices := s.getStatus(server)
			mutex.Lock()
			defer mutex.Unlock()
			for _, status := range allServices {
				if status.Status != nil {
					ret[server+":"+status.Status.Name] = status
				}
			}
		}(server.Name)
	}
	wg.Wait()

	return ret
}

// appNames returns a list of application names from a list of packages.
//
// For example:
//
//    appNames(["pull/pull:jcgregorio...", "push/push:someone@..."]
//
// will return
//
//    ["pull", "push"]
//
func appNames(installed []string) []string {
	ret := make([]string, len(installed))
	for i, s := range installed {
		ret[i] = strings.Split(s, "/")[0]
	}
	return ret
}

// AllUI contains all the information we know about the system.
type AllUI struct {
	Servers  ServersUI                      `json:"servers"`
	Packages map[string][]*packages.Package `json:"packages"`
	Status   map[string]*systemd.UnitStatus `json:"status"`
}

// stateHandler handles the GET of the JSON.
func (s *Server) stateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.FormValue("refresh") == "true" {
		if err := s.packageInfo.ForceRefresh(); err != nil {
			httputils.ReportError(w, err, "Failed to refresh.", http.StatusInternalServerError)
		}
	}
	allAvailable := s.packageInfo.AllAvailable()
	allInstalled := s.packageInfo.AllInstalled()

	// Update allInstalled to add in missing applications.
	//
	// Loop over 'config' and make sure each server and application is
	// represented, adding in "appName/" placeholders as package names where
	// appropriate. This is to bootstrap the case where an app is configured to
	// be available for a server, but no package for that application has been
	// installed yet.
	serversSeen := map[string]bool{}
	for name, installed := range allInstalled {
		installedNames := appNames(installed.Names)
		for _, expected := range s.config.Servers[name].AppNames {
			if !util.In(expected, installedNames) {
				installed.Names = append(installed.Names, expected+"/")
			}
		}
		allInstalled[name] = installed
		serversSeen[name] = true
	}

	// Now loop over config.Servers and find servers that don't have
	// any installed applications. Add them to allInstalled.
	for name, expected := range s.config.Servers {
		if _, ok := serversSeen[name]; ok {
			continue
		}
		installed := []string{}
		for _, appName := range expected.AppNames {
			installed = append(installed, appName+"/")
		}
		allInstalled[name].Names = installed
	}

	if r.Method == "POST" {
		if !login.IsAdmin(r) {
			httputils.ReportError(w, nil, "You must be logged on as an admin to push.", http.StatusInternalServerError)
			return
		}
		push := PushNewPackage{}
		dec := json.NewDecoder(r.Body)
		defer util.Close(r.Body)
		if err := dec.Decode(&push); err != nil {
			httputils.ReportError(w, fmt.Errorf("Failed to decode push request"), "Failed to decode push request", http.StatusInternalServerError)
			return
		}
		if installedPackages, ok := allInstalled[push.Server]; !ok {
			httputils.ReportError(w, fmt.Errorf("Unknown server name"), "Unknown server name", http.StatusInternalServerError)
			return
		} else {
			// Find a string starting with the same appname, replace it with
			// push.Name. Leave all other package names unchanged.
			appName := strings.Split(push.Name, "/")[0]
			newInstalled := []string{}
			for _, oldName := range installedPackages.Names {
				goodName := oldName
				if strings.Split(oldName, "/")[0] == appName {
					goodName = push.Name
				}
				newInstalled = append(newInstalled, goodName)
			}
			sklog.Infof("Updating %s with %#v giving %#v", push.Server, push.Name, newInstalled)
			if err := s.packageInfo.PutInstalled(push.Server, newInstalled, installedPackages.Generation); err != nil {
				httputils.ReportError(w, err, "Failed to update server.", http.StatusInternalServerError)
				return
			}
			body := fmt.Sprintf(CHAT_MSG, login.LoggedInAs(r), appName, push.Server)
			if err := chatbot.Send(body, "push", ""); err != nil {
				sklog.Warningf("Failed to send chat notification: %s", err)
			}

			if err := trigger.ByMetadata(s.comp, *project, push.Name, push.Server, s.ip.Zone(push.Server)); err != nil {
				sklog.Warningf("Could not trigger package load via metadata: %s", err)
			}
			allInstalled[push.Server].Names = newInstalled
		}
	}

	// The response to either a GET or a POST is an up to date ServersUI.
	servers := serversFromAllInstalled(allInstalled)
	enc := json.NewEncoder(w)
	err := enc.Encode(AllUI{
		Servers:  servers,
		Packages: allAvailable,
		Status:   s.serviceStatus(servers),
	})
	if err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

func serversFromAllInstalled(allInstalled map[string]*packages.Installed) ServersUI {
	servers := ServersUI{}
	names := make([]string, 0, len(allInstalled))
	for name := range allInstalled {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		servers = append(servers, &ServerUI{
			Name:      name,
			Installed: allInstalled[name].Names,
		})
	}

	return servers
}

// statusHandler handles the GET of the JSON for each service's status.
func (s *Server) statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	err := enc.Encode(s.getCurrentStatus())
	if err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

func (s *Server) getCurrentStatus() map[string]*systemd.UnitStatus {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.currentStatus
}

func (s *Server) stepStatus() {
	servers := serversFromAllInstalled(s.packageInfo.AllInstalled())
	updatedStatus := s.serviceStatus(servers)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.currentStatus = updatedStatus
}

func (s *Server) startStatusUpdate() {
	s.stepStatus()
	for range time.Tick(5 * time.Second) {
		s.stepStatus()
	}
}

// changeHandler handles actions on individual services.
//
// The actions are forwarded off to the pulld service
// running on the machine hosting that service.
func (s *Server) changeHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsAdmin(r) {
		httputils.ReportError(w, nil, "You must be logged on as an admin to push.", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, err, "Failed to parse form.", http.StatusInternalServerError)
		return
	}
	action := r.Form.Get("action")
	name := r.Form.Get("name")
	machine := r.Form.Get("machine")
	if action == "start" && name == "reboot.target" {
		body := fmt.Sprintf("%s rebooted %s", login.LoggedInAs(r), machine)
		if err := chatbot.Send(body, "push", ""); err != nil {
			sklog.Warningf("Failed to send chat notification: %s", err)
		}
	}
	url := fmt.Sprintf("http://%s:10000/_/change?name=%s&action=%s", machine, name, action)
	resp, err := s.fastClient.Post(url, "", nil)
	if err != nil {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to reach %s: %v %s", machine, resp, err), http.StatusInternalServerError)
		return
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		httputils.ReportError(w, err, fmt.Sprintf("Failed to reach %s: %v %s", machine, resp, err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, resp.Body); err != nil {
		sklog.Errorf("Failed to copy JSON error out: %s", err)
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", "max-age=300")
		fileServer.ServeHTTP(w, r)
	}
}

// oneStep does a single step of startDirtyMonitoring().
func (s *Server) oneStep() {
	count := int64(0)
	allInstalled := s.packageInfo.AllInstalled()
	allAvailable := s.packageInfo.AllAvailable()
	for serverName, installed := range allInstalled {
		// Don't warn about dirty packages on staging instances.
		if strings.HasSuffix(serverName, "-stage") {
			sklog.Infof("Skipping %s", serverName)
			continue
		}
		for _, app := range installed.Names {
			// app is the full versioned name of the installed app, we can find just
			// the package name of the app by splitting off the stuff before the
			// first "/".
			parts := strings.Split(app, "/")
			packageName := parts[0]
			for _, version := range allAvailable[packageName] {
				if version.Name == app {
					if version.Dirty {
						count++
						break
					}
				}
			}
		}
	}
	sklog.Infof("Finished oneStep: Found %d dirty packages running.", count)
	metrics2.GetInt64Metric("dirty_packages", nil).Update(count)
}

// startDirtyMonitoring periodically checks the number of dirty packages being
// used in prod and reports that number to metrics.
//
// This function doesn't return and should be launched as a Go routine.
func (s *Server) startDirtyMonitoring() {
	s.oneStep()
	for range time.Tick(time.Minute) {
		s.oneStep()
	}
}

func main() {
	common.InitWithMust(
		"push",
		common.PrometheusOpt(promPort),
	)
	if !*local {
		login.SimpleInitWithAllow(*port, *local, allowed.Googlers(), allowed.Googlers(), allowed.Googlers())
	}
	s := newServer()

	go s.startDirtyMonitoring()
	go s.startStatusUpdate()
	r := mux.NewRouter()
	r.HandleFunc("/_/change", s.changeHandler)
	r.HandleFunc("/_/state", s.stateHandler)
	r.HandleFunc("/_/status", s.statusHandler)
	if !*local {
		r.HandleFunc("/loginstatus/", login.StatusHandler)
		r.HandleFunc("/logout/", login.LogoutHandler)
		r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	}
	r.PathPrefix("/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	http.Handle("/", httputils.HealthzAndHTTPS(httputils.LoggingGzipRequestResponse(r)))
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
