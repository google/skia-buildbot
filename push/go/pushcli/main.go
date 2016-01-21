// pushcli is a simple command-line application for pushing a package to head.
package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/packages"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/storage/v1"
)

var (
	project  = flag.String("project", "google.com:skia-buildbots", "The Google Compute Engine project.")
	rollback = flag.Bool("rollback", false, "If true roll back to the next most recent package, otherwise use the most recently pushed package.")
)

func init() {
	flag.Usage = func() {
		fmt.Printf(`Usage: pushcli [options] <package> <server>

Pushes the latest version of <package> to <server>.

  <package> - The name of the package, e.g. "pulld"
  <server> - The name of the server, e.g. "skia-monitoring"

Use the --rollback flag to force a rollback to the previous version. Note that this always picks
the next most recent package, regardless of the version of the package currently deployed.

`)
		flag.PrintDefaults()
	}
}

func main() {
	defer common.LogPanic()
	common.Init()

	// Parse out the non-flag arguments.
	args := flag.Args()
	if len(args) != 2 {
		glog.Errorf("Requires two arguments.")
	}
	appName := args[0]    // "skdebuggerd"
	serverName := args[1] // "skia-debugger"

	// Create the needed clients.
	client, err := auth.NewDefaultJWTServiceAccountClient(storage.DevstorageFullControlScope, compute.ComputeReadonlyScope)
	if err != nil {
		glog.Fatalf("Failed to create authenticated HTTP client: %s", err)
	}
	store, err := storage.New(client)
	if err != nil {
		glog.Fatalf("Failed to create storage service client: %s", err)
	}
	comp, err := compute.New(client)
	if err != nil {
		glog.Fatalf("Failed to create compute service client: %s", err)
	}

	// Get the current set of packages installed on the server.
	installed, err := packages.InstalledForServer(client, store, serverName)
	glog.Infof("%#v", *installed)
	if err != nil {
		glog.Fatalf("Failed to get the current installed packages on %s: %s", serverName, err)

	}

	// Get the sorted list of available versions of the given package.
	available, err := packages.AllAvailableApp(store, appName)
	glog.Infof("%#v", available)
	if err != nil {
		glog.Fatalf("Failed to get the list of available versions for package %s: %s", appName, err)
	}

	// By default roll to head, which is the first entry in the slice.
	latest := available[0]
	if *rollback {
		if len(available) == 1 {
			glog.Fatalf("Can't rollback a package with only one version.")
		}
		latest = available[1]
	}

	// Build a new list of packages that is the old list of packages with the new package added.
	newInstalled := []string{fmt.Sprintf("%s/%s", appName, latest.Name)}
	for _, name := range installed.Names {
		if strings.Split(name, "/")[0] == appName {
			continue
		}
		newInstalled = append(newInstalled, name)
	}

	// Write the new list of packages back to Google Storage.
	if err := packages.PutInstalled(store, serverName, newInstalled, installed.Generation); err != nil {
		glog.Fatalf("Failed to write updated package for %s: %s", appName, err)
	}

	// If we are on the right network we can ping pulld to install the new
	// package and avoid the 15s wait for pulld to poll and find the new package.
	if ip, err := findIPAddress(comp, serverName); err == nil {
		glog.Infof("findIPAddress: %q %s", ip, err)
		resp, err := client.Get(fmt.Sprintf("http://%s:10114/pullpullpull", ip))
		if err != nil || resp == nil {
			glog.Infof("Failed to trigger an instant pull for server %s: %v %v", serverName, err)
		} else {
			util.Close(resp.Body)
		}
	}
}

// findIPAddress returns the ip address of the server with the given name.
func findIPAddress(comp *compute.Service, name string) (string, error) {
	// We have to look in each zone for the server with the given name.
	zones, err := comp.Zones.List(*project).Do()
	if err != nil {
		return "", fmt.Errorf("Failed to list zones: %s", err)
	}
	for _, zone := range zones.Items {
		item, err := comp.Instances.Get(*project, zone.Name, name).Do()
		if err != nil {
			continue
		}
		for _, nif := range item.NetworkInterfaces {
			for _, acc := range nif.AccessConfigs {
				if strings.HasPrefix(strings.ToLower(acc.Name), "external") {
					return acc.NatIP, nil
				}
			}
		}
	}
	return "", fmt.Errorf("Couldn't find an instance named: %s", name)
}
