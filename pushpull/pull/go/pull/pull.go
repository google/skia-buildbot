package main

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	storage "google.golang.org/api/storage/v1"
)

var (
	store *storage.Service

	allPackages map[string]*packages.Package

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

func step(ctx context.Context, client *http.Client, store *storage.Service, hostname string) {
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
		if err := packages.Install(ctx, client, store, name); err != nil {
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
	if len(newPackages) > 0 {
		allPackages, err = packages.AllAvailableByPackageName(store)
		if err != nil {
			sklog.Errorf("Failed to update the list of all packages: %s", err)
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

func pullInit(ctx context.Context, client *http.Client, metadataTriggerCh chan bool) {
	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Running with hostname: %s", hostname)

	store, err = storage.New(client)
	if err != nil {
		sklog.Fatalf("Failed to create storage service client: %s", err)
	}

	allPackages, err = packages.AllAvailableByPackageName(store)
	if err != nil {
		sklog.Fatalf("Failed to retrieve a list of all packages: %s", err)
	}

	step(ctx, client, store, hostname)
	timeCh := time.Tick(*pullPeriod)
	go func() {
		for {
			select {
			case <-timeCh:
			case <-metadataTriggerCh:
			}
			step(ctx, client, store, hostname)
		}
	}()
}
