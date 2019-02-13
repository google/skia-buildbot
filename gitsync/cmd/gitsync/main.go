package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"path/filepath"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/litevcs"
	"go.skia.org/infra/go/sklog"
)

func main() {
	// Define the flags and parse them.
	btInstanceID := flag.String("bt-instance", "production", "Big Table instance")
	btTableID := flag.String("bt-table", "git-repos", "BigTable table ID")
	dataDir := flag.String("data-dir", "", "Data directory where repos are cached.")
	httpPort := flag.String("http_port", ":9091", "The http port where ready-ness endpoints are served.")
	projectID := flag.String("project", "skia-public", "ID of the GCP project")
	repoURLs := common.NewMultiStringFlag("repo-url", []string{}, "Repo url")
	common.Init()

	// Make sure we have a data directory and it exists or can be created.
	if *dataDir == "" {
		sklog.Fatal("No data dir specified.")
	}
	useDataDir := fileutil.Must(fileutil.EnsureDirExists(*dataDir))

	// Make sure we have at least one repo configured.
	if len(*repoURLs) == 0 {
		sklog.Fatalf("At least one repository URL must be configured.")
	}

	// Configure the bigtable instance.
	config := &litevcs.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstanceID,
		TableID:    *btTableID,
		Shards:     32,
	}

	// Initialize bigtable.
	if err := litevcs.InitBT(config); err != nil {
		sklog.Fatalf("Error initializing BT: %s", err)
	}

	ctx := context.Background()
	for _, repoURL := range *repoURLs {
		go func(repoURL string) {
			repoDir, err := litevcs.NormalizeURL(repoURL)
			if err != nil {
				sklog.Fatalf("Error getting normalized URL for %q:  %s", repoURL, err)
			}
			repoDir = filepath.Join(useDataDir, repoDir)

			watcher, err := NewRepoWatcher(config, repoURL, repoDir)
			if err != nil {
				sklog.Fatalf("Error initializing repo watcher: %s", err)
			}
			go watcher.Start(ctx)
		}(repoURL)
	}

	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	log.Fatal(http.ListenAndServe(*httpPort, nil))
}
