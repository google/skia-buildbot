package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/storage/v1"
)

var (
	httpTriggerCh = make(chan bool, 1)

	store *storage.Service
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

		// The pull application is special in that it's not restarted by the
		// the postinstall script of the debian package, because that might kill
		// pullg while it was updating itself. Instead pulld will just exit when
		// it notices that it has been updated and count on systemd to restart it.
		if containsPulld(newPackages) {
			glog.Info("The pulld package has been updated, exiting to allow a restart.")
			glog.Flush()
			os.Exit(0)
		}
	}
}

// containsPull returns true if the list of installed packages contains the 'pull' package.
func containsPulld(packages []string) bool {
	for _, s := range packages {
		if strings.Split(s, "/")[0] == "pulld" {
			return true
		}
	}
	return false
}

// pullHandler triggers a pull when /pullpullpull is requested.
func pullHandler(w http.ResponseWriter, r *http.Request) {
	httpTriggerCh <- true
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "Pull triggered.")
}

func pullInit() {
	hostname, err := os.Hostname()
	if err != nil {
		// Never call glog before common.Init*.
		os.Exit(1)
	}
	common.InitWithMetrics("pulld."+hostname, graphiteServer)
	glog.Infof("Running with hostname: %s", hostname)

	client, err := auth.NewClient(*doOauth, *oauthCacheFile,
		storage.DevstorageFullControlScope,

		compute.ComputeReadonlyScope)
	if err != nil {
		glog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}
	glog.Info("Got authenticated client.")

	store, err = storage.New(client)
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
}
