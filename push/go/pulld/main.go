// pulld is an application that monitors and pulls down new Debian packages and installs them.
//
// It presumes each package is monitored via systemd and not monit, so there's
// no monitoring of monit config files is done.
//
// Note that it is safe for pulld to install pulld since the list of installed
// packages is written out before 'dpkg -i' is called.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"code.google.com/p/google-api-go-client/storage/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/push/go/gsauth"
	"go.skia.org/infra/push/go/packages"
)

// flags
var (
	graphiteServer        = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	doOauth               = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	oauthCacheFile        = flag.String("oauth_cache_file", "google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	installedPackagesFile = flag.String("installed_packages_file", "installed_packages.json", "Path to the file where to cache the list of installed debs.")
	port                  = flag.String("port", ":10116", "HTTP service address (e.g., ':8000')")
)

var (
	httpTriggerCh = make(chan bool, 1)
)

// differences returns all strings that appear in server but not local.
func differences(server, local []string) ([]string, []string) {
	newPackages := []string{}
	installedPackages := []string{}
	for _, s := range server {
		if util.In(s, local) {
			installedPackages = append(installedPackages, s)
		} else {
			newPackages = append(newPackages, s)
		}
	}
	return newPackages, installedPackages
}

func step(client *http.Client, store *storage.Service, hostname string) {
	glog.Info("About to read package list.")
	// Read the old and new packages from their respective storage locations.
	serverList, err := packages.InstalledForServer(client, store, hostname)
	if err != nil {
		glog.Errorf("Failed to retrieve remote package list: %s", err)
		return
	}
	localList, err := packages.FromLocalFile(*installedPackagesFile)
	if err != nil {
		glog.Errorf("Failed to retrieve local package list: %s", err)
		return
	}

	// Install any new or updated packages.
	newPackages, installed := differences(serverList.Names, localList)
	glog.Infof("New: %v, Installed: %v", newPackages, installed)

	for _, name := range newPackages {
		// If just an appname appears w/o a package name then that means
		// that package hasn't been selected, so just skip it for now.
		if len(strings.Split(name, "/")) == 1 {
			continue
		}
		installed = append(installed, name)
		if err := packages.ToLocalFile(installed, *installedPackagesFile); err != nil {
			glog.Errorf("Failed to write local package list: %s", err)
			continue
		}
		if err := packages.Install(client, store, name); err != nil {
			glog.Errorf("Failed to install package %s: %s", name, err)
			// Pop last name from 'installed' then rewrite the file since the
			// install failed.
			installed = installed[:len(installed)-1]
			if err := packages.ToLocalFile(installed, *installedPackagesFile); err != nil {
				glog.Errorf("Failed to rewrite local package list after install failure for %s: %s", name, err)
			}
			continue
		}
	}
}

// pullHandler triggers a pull when /pullpullpull is requested.
func pullHandler(w http.ResponseWriter, r *http.Request) {
	httpTriggerCh <- true
	fmt.Fprintf(w, "Pull triggered.")
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		// Never call glog before common.Init*.
		os.Exit(1)
	}
	common.InitWithMetrics("pulld."+hostname, graphiteServer)
	glog.Infof("Running with hostname: %s", hostname)

	client, err := gsauth.NewClient(*doOauth, *oauthCacheFile)
	if err != nil {
		glog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}
	glog.Info("Got authenticated client.")

	store, err := storage.New(client)
	if err != nil {
		glog.Fatalf("Failed to create storage service client: %s", err)
	}

	step(client, store, hostname)
	timeCh := time.Tick(time.Second * 60)
	go func() {
		for {
			select {
			case <-timeCh:
			case <-httpTriggerCh:
			}
			step(client, store, hostname)
		}
	}()
	http.HandleFunc("/pullpullpull", pullHandler)
	glog.Fatal(http.ListenAndServe(*port, nil))
}
