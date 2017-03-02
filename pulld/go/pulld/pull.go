package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	compute "google.golang.org/api/compute/v1"
	storage "google.golang.org/api/storage/v1"
)

var (
	metadataTriggerCh = make(chan bool, 1)

	store *storage.Service

	failedInstallCounter = metrics2.GetCounter("pulld_failed_install", nil)
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
	sklog.Info("About to read package list.")
	// Read the old and new packages from their respective storage locations.
	serverList, err := packages.InstalledForServer(client, store, hostname)
	if err != nil {
		sklog.Errorf("Failed to retrieve remote package list: %s", err)
		return
	}
	localList, err := packages.FromLocalFile(*installedPackagesFile)
	if err != nil {
		sklog.Errorf("Failed to retrieve local package list: %s", err)
		return
	}

	// Install any new or updated packages.
	newPackages, installed := differences(serverList.Names, localList)
	sklog.Infof("New: %v, Installed: %v", newPackages, installed)

	for _, name := range newPackages {
		// If just an appname appears w/o a package name then that means
		// that package hasn't been selected, so just skip it for now.
		if len(strings.Split(name, "/")) == 1 {
			continue
		}
		installed = append(installed, name)
		if err := packages.ToLocalFile(installed, *installedPackagesFile); err != nil {
			sklog.Errorf("Failed to write local package list: %s", err)
			continue
		}
		if err := packages.Install(client, store, name); err != nil {
			failedInstallCounter.Inc(1)
			sklog.Errorf("Failed to install package %s: %s", name, err)
			// Pop last name from 'installed' then rewrite the file since the
			// install failed.
			installed = installed[:len(installed)-1]
			if err := packages.ToLocalFile(installed, *installedPackagesFile); err != nil {
				sklog.Errorf("Failed to rewrite local package list after install failure for %s: %s", name, err)
			}
			continue
		}

		// The pull application is special in that it's not restarted by the
		// the postinstall script of the debian package, because that might kill
		// pullg while it was updating itself. Instead pulld will just exit when
		// it notices that it has been updated and count on systemd to restart it.
		if containsPulld(newPackages) {
			sklog.Info("The pulld package has been updated, exiting to allow a restart.")
			sklog.Flush()
			os.Exit(0)
		}
	}
}

// containsPull returns true if the list of installed packages contains the 'pull' package.
func containsPulld(packages []string) bool {
	for _, s := range packages {
		if p := strings.Split(s, "/")[0]; p == "pulld" || p == "pulld-not-gce" {
			return true
		}
	}
	return false
}

// metadataWait waits for the instance level metadata 'pushrev' to change, at
// which point the server should check for new versions of apps to install.
func metadataWait() {
	for {
		req, err := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/attributes/pushrev?wait_for_change=true", nil)
		if err != nil {
			sklog.Errorf("Failed to create request: %s", err)
			continue
		}
		req.Header.Set("Metadata-Flavor", "Google")
		// We use the default client which should never timeout.
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode != 200 {
			sklog.Errorf("wait_for_change failed: %s", err)
			if resp != nil {
				sklog.Errorf("Response: %+v", *resp)
			}
			time.Sleep(time.Minute)
			continue
		}
		metadataTriggerCh <- true
		sklog.Infof("Pull triggered via metadata.")
	}
}

func pullInit(serviceAccountPath string) {
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Running with hostname: %s", hostname)

	client, err := auth.NewJWTServiceAccountClient("", serviceAccountPath, &http.Transport{Dial: httputils.DialTimeout}, storage.DevstorageFullControlScope, compute.ComputeReadonlyScope)
	if err != nil {
		sklog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}
	sklog.Info("Got authenticated client.")

	store, err = storage.New(client)
	if err != nil {
		sklog.Fatalf("Failed to create storage service client: %s", err)
	}

	if *onGCE {
		go metadataWait()
	}

	step(client, store, hostname)
	timeCh := time.Tick(*pullPeriod)
	go func() {
		for {
			select {
			case <-timeCh:
			case <-metadataTriggerCh:
			}
			step(client, store, hostname)
		}
	}()
}
