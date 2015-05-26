// pull is an application that monitors and pulls down new Debian packages and installs them.
package main

import (
	"bytes"
	"flag"

	"code.google.com/p/google-api-go-client/compute/v1"
	"code.google.com/p/google-api-go-client/storage/v1"

	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/skia-dev/glog"

	"strings"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/push/go/packages"
)

var (
	graphiteServer        = flag.String("graphite_server", "skia-monitoring:2003", "Where is Graphite metrics ingestion server running.")
	doOauth               = flag.Bool("oauth", true, "Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.")
	oauthCacheFile        = flag.String("oauth_cache_file", "google_storage_token.data", "Path to the file where to cache cache the oauth credentials.")
	installedPackagesFile = flag.String("installed_packages_file", "installed_packages.json", "Path to the file where to cache the list of installed debs.")
	hostname              = flag.String("hostname", "", "The hostname to use, will use os.Hostname() if not provided.")
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

// containsPull returns true if the list of installed packages contains the 'pull' package.
func containsPull(packages []string) bool {
	for _, s := range packages {
		if strings.Split(s, "/")[0] == "pull" {
			return true
		}
	}
	return false
}

func main() {
	if *hostname == "" {
		var err error
		*hostname, err = os.Hostname()
		if err != nil {
			// Never call glog before common.Init*.
			os.Exit(1)
		}
	}
	common.InitWithMetrics("pull."+*hostname, graphiteServer)
	glog.Infof("Running with hostname: %s", *hostname)

	client, err := auth.NewClient(*doOauth, *oauthCacheFile,
		storage.DevstorageFull_controlScope,
		compute.ComputeReadonlyScope)

	if err != nil {
		glog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}
	glog.Info("Got authenticated client.")

	store, err := storage.New(client)
	if err != nil {
		glog.Fatalf("Failed to create storage service client: %s", err)
	}

	for _ = range time.Tick(time.Second * 15) {
		before, err := filepath.Glob("/etc/monit/conf.d/*")
		if err != nil {
			glog.Errorf("Failed to list all monit config files: %s", err)
			continue
		}

		glog.Info("About to read package list.")
		// Read the old and new packages from their respective storage locations.
		serverList, err := packages.InstalledForServer(client, store, *hostname)
		if err != nil {
			glog.Errorf("Failed to retrieve remote package list: %s", err)
			continue
		}
		localList, err := packages.FromLocalFile(*installedPackagesFile)
		if err != nil {
			glog.Errorf("Failed to retrieve local package list: %s", err)
			continue
		}

		glog.Info("Comparing against currently installed packages.")
		// Install any new or updated packages.
		newPackages, installedPackages := differences(serverList.Names, localList)
		for _, p := range newPackages {
			glog.Infof("New Package:%s", p)
		}
		for _, p := range installedPackages {
			glog.Infof("Installed Package:%s", p)
		}

		save := false
		for _, name := range newPackages {
			// If just an appname appears w/o a packge name then that means
			// that package hasn't been selected, so just skip it for now.
			if len(strings.Split(name, "/")) == 1 {
				continue
			}
			if err := packages.Install(client, store, name); err != nil {
				glog.Errorf("Failed to install package %s: %s", name, err)
				continue
			}
			installedPackages = append(installedPackages, name)
			save = true
		}
		// Only write out the file if any new packages were installed.
		if !save {
			continue
		}
		if err := packages.ToLocalFile(installedPackages, *installedPackagesFile); err != nil {
			glog.Errorf("Failed to write local package list: %s", err)
		}

		after, err := filepath.Glob("/etc/monit/conf.d/*")
		if err != nil {
			glog.Errorf("Failed to list all monit config files: %s", err)
			continue
		}

		// Tell monit to reload if the name or number of files under /etc/monit/conf.d have changed.
		if !util.SSliceEqual(before, after) {
			cmd := exec.Command("sudo", "monit", "reload")
			var out bytes.Buffer
			cmd.Stdout = &out
			if err := cmd.Run(); err != nil {
				glog.Errorf("Failed to reload monit: %s", err)
				glog.Errorf("Failed to reload monit (stdout): %s", out.String())
				break
			}
		}

		// The pull application is special and not monitored by monit to restart on
		// timestamp changes because that might kill pull while it was updating
		// itself. Instead pull will just exit when it notices that it has been
		// updated and count on monit to restart pull.
		if containsPull(newPackages) {
			glog.Info("The pull package has been updated, exiting to allow a restart.")
			os.Exit(0)
		}
	}
}
