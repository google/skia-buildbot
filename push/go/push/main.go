// push is the web server for pushing debian packages.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/influxdb"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/systemd"
	"go.skia.org/infra/go/util"
	compute "google.golang.org/api/compute/v1"
	storage "google.golang.org/api/storage/v1"
)

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// config is the configuration of what servers and apps we are managing.
	config packages.PackageConfig

	// ip keeps an updated map from server name to public IP address.
	ip *IPAddresses

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
)

// flags
var (
	bucketName     = flag.String("bucket_name", "skia-push", "The name of the Google Storage bucket that contains push packages and info.")
	configFilename = flag.String("config_filename", "skiapush.conf", "Config filename.")
	influxDatabase = flag.String("influxdb_database", influxdb.DEFAULT_DATABASE, "The InfluxDB database.")
	influxHost     = flag.String("influxdb_host", influxdb.DEFAULT_HOST, "The InfluxDB hostname.")
	influxPassword = flag.String("influxdb_password", influxdb.DEFAULT_PASSWORD, "The InfluxDB password.")
	influxUser     = flag.String("influxdb_name", influxdb.DEFAULT_USER, "The InfluxDB username.")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	project        = flag.String("project", "google.com:skia-buildbots", "The Google Compute Engine project.")
	promPort       = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
)

func loadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

// FastDialTimeout is a dialer that sets a timeout.
func FastDialTimeout(network, addr string) (net.Conn, error) {
	return net.DialTimeout(network, addr, 10*time.Second)
}

// NewTimeoutClient creates a new http.Client with both a dial timeout and a
// request timeout.
func NewFastTimeoutClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: FastDialTimeout,
		},
		Timeout: 2 * time.Second,
	}
}

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	loadTemplates()

	var err error
	config, err = packages.LoadPackageConfig(*configFilename)
	if err != nil {
		sklog.Fatalf("Failed to load PackageConfig file: %s", err)
	}

	serverNames = config.AllServerNames()

	if client, err = auth.NewDefaultJWTServiceAccountClient(auth.SCOPE_FULL_CONTROL, auth.SCOPE_GCE); err != nil {
		sklog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}

	fastClient = NewFastTimeoutClient()

	if store, err = storage.New(client); err != nil {
		sklog.Fatalf("Failed to create storage service client: %s", err)
	}
	if comp, err = compute.New(client); err != nil {
		sklog.Fatalf("Failed to create compute service client: %s", err)
	}
	ip, err = NewIPAddresses(comp)
	if err != nil {
		sklog.Fatalf("Failed to load IP addresses at startup: %s", err)
	}

	packages.SetBucketName(*bucketName)
	packageInfo, err = packages.NewAllInfo(client, store, serverNames)
	if err != nil {
		sklog.Fatalf("Failed to create packages.AllInfo at startup: %s", err)
	}
}

// IPAddresses keeps track of the external IP addresses of each server.
type IPAddresses struct {
	ip    map[string]string
	comp  *compute.Service
	mutex sync.Mutex
}

func (i *IPAddresses) loadIPAddresses() error {
	zones, err := comp.Zones.List(*project).Do()
	if err != nil {
		return fmt.Errorf("Failed to list zones: %s", err)
	}
	ip := map[string]string{}
	for _, zone := range zones.Items {
		sklog.Infof("Zone: %s", zone.Name)
		list, err := comp.Instances.List(*project, zone.Name).Do()
		if err != nil {
			return fmt.Errorf("Failed to list instances: %s", err)
		}
		for _, item := range list.Items {
			for _, nif := range item.NetworkInterfaces {
				for _, acc := range nif.AccessConfigs {
					if strings.HasPrefix(strings.ToLower(acc.Name), "external") {
						ip[item.Name] = acc.NatIP
					}
				}
			}
		}
	}
	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.ip = ip
	return nil
}

// Get returns the current set of external IP addresses for servers.
func (i *IPAddresses) Get() map[string]string {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	return i.ip
}

// Resolve the server name to an ip address, but only if running locally.
func (i *IPAddresses) Resolve(server string) string {
	serverName := server
	if ipaddr, ok := i.Get()[server]; ok && *local {
		serverName = ipaddr
	}
	return serverName
}

func NewIPAddresses(comp *compute.Service) (*IPAddresses, error) {
	i := &IPAddresses{
		ip:   map[string]string{},
		comp: comp,
	}
	if err := i.loadIPAddresses(); err != nil {
		return nil, err
	}
	go func() {
		for _ = range time.Tick(time.Second * 60) {
			if err := i.loadIPAddresses(); err != nil {
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
func getStatus(server string) []*systemd.UnitStatus {
	resp, err := fastClient.Get(fmt.Sprintf("http://%s:10114/_/list", ip.Resolve(server)))
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

	ret := []*systemd.UnitStatus{}
	if err := dec.Decode(&ret); err != nil {
		sklog.Infof("Failed to decode: %s", err)
		return nil
	}
	sklog.Infof("%s - %#v", server, ret)
	return ret
}

// serviceStatus returns a map[string]*systemd.UnitStatus, with one entry for each service running on each
// server. The keys for the return value are "<server_name>:<service_name>", for example,
// "skia-push:logserverd.service".
func serviceStatus(servers ServersUI) map[string]*systemd.UnitStatus {
	var mutex sync.Mutex
	var wg sync.WaitGroup
	ret := map[string]*systemd.UnitStatus{}
	for _, server := range servers {
		wg.Add(1)
		go func(server string) {
			defer wg.Done()
			allServices := getStatus(server)
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
	IP       map[string]string              `json:"ip"`
	Status   map[string]*systemd.UnitStatus `json:"status"`
}

// stateHandler handles the GET of the JSON.
func stateHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.FormValue("refresh") == "true" {
		if err := packageInfo.ForceRefresh(); err != nil {
			httputils.ReportError(w, r, err, "Failed to refresh.")
		}
	}
	allAvailable := packageInfo.AllAvailable()
	allInstalled := packageInfo.AllInstalled()

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
		for _, expected := range config.Servers[name].AppNames {
			if !util.In(expected, installedNames) {
				installed.Names = append(installed.Names, expected+"/")
			}
		}
		allInstalled[name] = installed
		serversSeen[name] = true
	}

	// Now loop over config.Servers and find servers that don't have
	// any installed applications. Add them to allInstalled.
	for name, expected := range config.Servers {
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
			httputils.ReportError(w, r, nil, "You must be logged on as an admin to push.")
			return
		}
		push := PushNewPackage{}
		dec := json.NewDecoder(r.Body)
		defer util.Close(r.Body)
		if err := dec.Decode(&push); err != nil {
			httputils.ReportError(w, r, fmt.Errorf("Failed to decode push request"), "Failed to decode push request")
			return
		}
		if installedPackages, ok := allInstalled[push.Server]; !ok {
			httputils.ReportError(w, r, fmt.Errorf("Unknown server name"), "Unknown server name")
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
			if err := packageInfo.PutInstalled(push.Server, newInstalled, installedPackages.Generation); err != nil {
				httputils.ReportError(w, r, err, "Failed to update server.")
				return
			}
			resp, err := fastClient.Get(fmt.Sprintf("http://%s:10114/pullpullpull", push.Server))
			if err != nil || resp == nil {
				sklog.Infof("Failed to trigger an instant pull for server %s: %v %v", push.Server, err, resp)
			} else {
				util.Close(resp.Body)
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
		IP:       ip.Get(),
		Status:   serviceStatus(servers),
	})
	if err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

func serversFromAllInstalled(allInstalled map[string]*packages.Installed) ServersUI {
	servers := ServersUI{}
	names := make([]string, 0, len(allInstalled))
	for name, _ := range allInstalled {
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
func statusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	servers := serversFromAllInstalled(packageInfo.AllInstalled())
	enc := json.NewEncoder(w)
	err := enc.Encode(serviceStatus(servers))
	if err != nil {
		sklog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// changeHandler handles actions on individual services.
//
// The actions are forwarded off to the pulld service
// running on the machine hosting that service.
func changeHandler(w http.ResponseWriter, r *http.Request) {
	if !login.IsAdmin(r) {
		httputils.ReportError(w, r, nil, "You must be logged on as an admin to push.")
		return
	}
	if err := r.ParseForm(); err != nil {
		httputils.ReportError(w, r, err, "Failed to parse form.")
		return
	}
	action := r.Form.Get("action")
	name := r.Form.Get("name")
	machine := ip.Resolve(r.Form.Get("machine"))
	url := fmt.Sprintf("http://%s:10114/_/change?name=%s&action=%s", machine, name, action)
	resp, err := fastClient.Post(url, "", nil)
	if err != nil {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to reach %s: %v %s", machine, resp, err))
		return
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		httputils.ReportError(w, r, err, fmt.Sprintf("Failed to reach %s: %v %s", machine, resp, err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := io.Copy(w, resp.Body); err != nil {
		sklog.Errorf("Failed to copy JSON error out: %s", err)
	}
}

// mainHandler handles the GET of the main page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	if *local {
		loadTemplates()
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		if err := indexTemplate.Execute(w, struct{}{}); err != nil {
			sklog.Errorln("Failed to expand template:", err)
		}
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
func oneStep() {
	count := int64(0)
	allInstalled := packageInfo.AllInstalled()
	allAvailable := packageInfo.AllAvailable()
	for _, installed := range allInstalled {
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
	metrics2.GetInt64Metric("dirty-packages", nil).Update(count)
}

// startDirtyMonitoring periodically checks the number of dirty packages being
// used in prod and reports that number to Influx.
//
// This function doesn't return and should be launched as a Go routine.
func startDirtyMonitoring() {
	oneStep()
	for _ = range time.Tick(time.Minute) {
		oneStep()
	}
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"push",
		common.InfluxOpt(influxHost, influxUser, influxPassword, influxDatabase, local),
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)
	login.SimpleInitMust(*port, *local)

	Init()

	go startDirtyMonitoring()
	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(httputils.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/_/change", changeHandler)
	r.HandleFunc("/_/state", stateHandler)
	r.HandleFunc("/_/status", statusHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
