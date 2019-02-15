package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

// This server watches a list of git repos for changes and syncs the meta data of all commits
// to a BigTable backed datastore.

func main() {
	// Define the flags and parse them.
	var (
		btInstanceID    = flag.String("bt-instance", "production", "Big Table instance")
		btTableID       = flag.String("bt-table", "git-repos", "BigTable table ID")
		dataDir         = flag.String("data-dir", "", "Data directory where repos are cached.")
		httpPort        = flag.String("http_port", ":9091", "The http port where ready-ness endpoints are served.")
		projectID       = flag.String("project", "skia-public", "ID of the GCP project")
		repoURLs        = common.NewMultiStringFlag("repo-url", []string{}, "Repo url")
		runInit         = flag.Bool("init", false, "Initialize the BigTable instance and quit. This should be run with a different different user who has admin rights.")
		refreshInterval = flag.Duration("refresh", 10*time.Minute, "Interval in which to poll git and refresh the GitStore.")
	)
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
	config := &gitstore.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstanceID,
		TableID:    *btTableID,
		Shards:     32,
	}

	// Initialize bigtable if invoked with --init and quit.
	// This should be invoked with a user that has admin privileges, so that the production user that
	// wants to write to the instance does not need admin privileges.
	if *runInit {
		if err := gitstore.InitBT(config); err != nil {
			sklog.Fatalf("Error initializing BT: %s", err)
		}
		sklog.Infof("BigTable instance %s and table %s in project %s initialized.", *btInstanceID, *btTableID, *projectID)
		return
	}

	// Start all repo watchers in the background.
	ctx := context.Background()
	for _, repoURL := range *repoURLs {
		go func(repoURL string) {
			repoDir, err := gitstore.NormalizeURL(repoURL)
			if err != nil {
				sklog.Fatalf("Error getting normalized URL for %q:  %s", repoURL, err)
			}
			repoDir = filepath.Join(useDataDir, repoDir)

			watcher, err := NewRepoWatcher(config, repoURL, repoDir)
			if err != nil {
				sklog.Fatalf("Error initializing repo watcher: %s", err)
			}
			watcher.Start(ctx, *refreshInterval)
		}(repoURL)
	}

	// Set up the http handler to indicate ready-ness and start serving.
	http.HandleFunc("/healthz", httputils.ReadyHandleFunc)
	log.Fatal(http.ListenAndServe(*httpPort, nil))
}
