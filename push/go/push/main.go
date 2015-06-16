// push is the web server for pushing debian packages.
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
	"sync"
	"time"

	"code.google.com/p/google-api-go-client/compute/v1"
	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/BurntSushi/toml"
	"github.com/coreos/go-systemd/dbus"
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/util"
)

// Server is used in PushConfig.
type Server struct {
	AppNames []string
}

// PushConfig is the configuration of the application.
//
// It is a list of servers (by GCE domain name) and the list
// of apps that are allowed to be installed on them. It is
// loaded from *config_filename in toml format.
type PushConfig struct {
	Servers map[string]Server
}

var (
	// indexTemplate is the main index.html page we serve.
	indexTemplate *template.Template = nil

	// config is the configuration of what servers and apps we are managing.
	config PushConfig

	// ip keeps an updated map from server name to public IP address.
	ip *IPAddresses

	// serverNames is a list of server names (GCE DNS names) we are managing.
	// Extracted from 'config'.
	serverNames []string

	// client is an HTTP client authorized to read and write gs://skia-push.
	client *http.Client

	// store is an Google Storage API client authorized to read and write gs://skia-push.
	store *storage.Service

	// comp is an Google Compute API client authorized to read compute information.
	comp *compute.Service

	// packageInfo is a cache of info about packages.
	packageInfo *packages.AllInfo
)

// flags
var (
	port           = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	graphiteServer = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	doOauth        = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	oauthCacheFile = flag.String("oauth_cache_file", "google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	configFilename = flag.String("config_filename", "skiapush.conf", "Config filename.")
	resourcesDir   = flag.String("resources_dir", "", "The directory to find templates, JS, and CSS files. If blank the current directory will be used.")
	project        = flag.String("project", "google.com:skia-buildbots", "The Google Compute Engine project.")
)

func loadTemplates() {
	indexTemplate = template.Must(template.ParseFiles(
		filepath.Join(*resourcesDir, "templates/index.html"),
		filepath.Join(*resourcesDir, "templates/titlebar.html"),
		filepath.Join(*resourcesDir, "templates/header.html"),
	))
}

func Init() {
	if *resourcesDir == "" {
		_, filename, _, _ := runtime.Caller(0)
		*resourcesDir = filepath.Join(filepath.Dir(filename), "../..")
	}
	loadTemplates()

	// Read toml config file.
	if _, err := toml.DecodeFile(*configFilename, &config); err != nil {
		glog.Fatalf("Failed to decode config file: %s", err)
	}

	serverNames = make([]string, 0, len(config.Servers))
	for k, _ := range config.Servers {
		serverNames = append(serverNames, k)
	}

	var err error
	if client, err = auth.NewClient(*doOauth, *oauthCacheFile, auth.SCOPE_FULL_CONTROL, auth.SCOPE_GCE); err != nil {
		glog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}

	if store, err = storage.New(client); err != nil {
		glog.Fatalf("Failed to create storage service client: %s", err)
	}
	if comp, err = compute.New(client); err != nil {
		glog.Fatalf("Failed to create compute service client: %s", err)
	}
	ip, err = NewIPAddresses(comp)
	if err != nil {
		glog.Fatalf("Failed to load IP addresses at startup: %s", err)
	}

	packageInfo, err = packages.NewAllInfo(client, store, serverNames)
	if err != nil {
		glog.Fatalf("Failed to create packages.AllInfo at startup: %s", err)
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
		glog.Infof("Zone: %s", zone.Name)
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
				glog.Infof("Error refreshing IP address list: %s", err)
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

// UnitStatus is serialized to JSON in the return of pulld propsHandler.
type UnitStatus struct {
	// Status is the current status of the unit.
	Status *dbus.UnitStatus `json:"status"`

	// Props is the set of unit properties returned from GetUnitTypeProperties.
	Props map[string]interface{} `json:"props"`
}

// getStatus returns a populated *UnitStatus for the given server and service, and
// nil if the information wasn't able to be retrieved.
func getStatus(server, service string) *UnitStatus {
	serverName := server
	if ipaddr, ok := ip.Get()[server]; ok && *local {
		serverName = ipaddr
	}
	resp, err := client.Get(fmt.Sprintf("http://%s:10114/_/props?service=%s", serverName, service))
	if err != nil || resp.StatusCode > 400 {
		glog.Infof("Failed to get status of: %s %s", server, service)
		return nil
	}
	dec := json.NewDecoder(resp.Body)
	defer util.Close(resp.Body)

	props := &UnitStatus{
		Props: map[string]interface{}{},
	}
	if err := dec.Decode(props); err != nil {
		return nil
	}
	return props
}

// serviceStatus returns a map[string]*UnitStatus, with one entry for each service running on each
// server. The keys for the return value are "<server_name>:<service_name>", for example,
// "skia-push:logserverd.service".
func serviceStatus(servers ServersUI, allAvailable map[string][]*packages.Package) map[string]*UnitStatus {
	// First populate a quick package lookup map.
	packageLookup := map[string]*packages.Package{}
	for _, ps := range allAvailable {
		for _, p := range ps {
			packageLookup[p.Name] = p
		}
	}

	ret := map[string]*UnitStatus{}
	for _, server := range servers {
		for _, packageName := range server.Installed {
			if p, ok := packageLookup[packageName]; ok {
				for _, service := range p.Services {
					ret[server.Name+":"+service] = getStatus(server.Name, service)
				}
			}
		}
	}

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
	Status   map[string]*UnitStatus         `json:"status"`
}

// jsonHandler handles the GET of the JSON.
func jsonHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("JSON Handler: %q\n", r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

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
		if login.LoggedInAs(r) == "" {
			util.ReportError(w, r, fmt.Errorf("You must be logged on to push."), "")
			return
		}
		push := PushNewPackage{}
		dec := json.NewDecoder(r.Body)
		defer util.Close(r.Body)
		if err := dec.Decode(&push); err != nil {
			util.ReportError(w, r, fmt.Errorf("Failed to decode push request"), "Failed to decode push request")
			return
		}
		if installedPackages, ok := allInstalled[push.Server]; !ok {
			util.ReportError(w, r, fmt.Errorf("Unknown server name"), "Unknown server name")
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
			glog.Infof("Updating %s with %#v giving %#v", push.Server, push.Name, newInstalled)
			if err := packageInfo.PutInstalled(push.Server, newInstalled, installedPackages.Generation); err != nil {
				util.ReportError(w, r, err, "Failed to update server.")
				return
			}
			resp, err := client.Get(fmt.Sprintf("http://%s:10114/pullpullpull", push.Server))
			if err != nil || resp == nil {
				glog.Infof("Failed to trigger an instant pull for server %s: %v %v", push.Server, err, resp)
			}
			allInstalled[push.Server].Names = newInstalled
		}
	}

	// The response to either a GET or a POST is an up to date ServersUI.
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

	enc := json.NewEncoder(w)
	err := enc.Encode(AllUI{
		Servers:  servers,
		Packages: allAvailable,
		IP:       ip.Get(),
		Status:   serviceStatus(servers, allAvailable),
	})
	if err != nil {
		glog.Errorf("Failed to write or encode output: %s", err)
		return
	}
}

// mainHandler handles the GET of the main page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	glog.Infof("Main Handler: %q\n", r.URL.Path)
	if *local {
		loadTemplates()
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "text/html")
		if err := indexTemplate.Execute(w, struct{}{}); err != nil {
			glog.Errorln("Failed to expand template:", err)
		}
	}
}

func makeResourceHandler() func(http.ResponseWriter, *http.Request) {
	fileServer := http.FileServer(http.Dir(*resourcesDir))
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Cache-Control", string(300))
		fileServer.ServeHTTP(w, r)
	}
}

func main() {
	common.InitWithMetrics("push", graphiteServer)
	Init()

	// By default use a set of credentials setup for localhost access.
	var cookieSalt = "notverysecret"
	var clientID = "31977622648-1873k0c1e5edaka4adpv1ppvhr5id3qm.apps.googleusercontent.com"
	var clientSecret = "cw0IosPu4yjaG2KWmppj2guj"
	var redirectURL = fmt.Sprintf("http://localhost%s/oauth2callback/", *port)
	if !*local {
		cookieSalt = metadata.Must(metadata.ProjectGet(metadata.COOKIESALT))
		clientID = metadata.Must(metadata.ProjectGet(metadata.CLIENT_ID))
		clientSecret = metadata.Must(metadata.ProjectGet(metadata.CLIENT_SECRET))
		redirectURL = "https://push.skia.org/oauth2callback/"
	}
	login.Init(clientID, clientSecret, redirectURL, cookieSalt, login.DEFAULT_SCOPE, login.DEFAULT_DOMAIN_WHITELIST, *local)

	r := mux.NewRouter()
	r.PathPrefix("/res/").HandlerFunc(util.MakeResourceHandler(*resourcesDir))
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/json/", jsonHandler)
	r.HandleFunc("/oauth2callback/", login.OAuth2CallbackHandler)
	r.HandleFunc("/logout/", login.LogoutHandler)
	r.HandleFunc("/loginstatus/", login.StatusHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Infoln("Ready to serve.")
	glog.Fatal(http.ListenAndServe(*port, nil))
}
