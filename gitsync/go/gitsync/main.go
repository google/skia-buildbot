package main

import (
	"context"
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/litevcs"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo"
)

const (
	concurrentCommits = 1000
	concurrentWrites  = 1000
)

var (
	btInstanceID = flag.String("bt-instance", "production", "Big Table instance")
	btTableID    = flag.String("bt-table", "git-repos", "BigTable table ID")
	dataDir      = flag.String("data-dir", "", "Data directory where repos are cached.")
	projectID    = flag.String("project", "skia-public", "ID of the GCP project")
	repoURLs     = common.NewMultiStringFlag("repo-url", []string{}, "Repo url")
	skipLoad     = flag.Bool("skipload", false, "Skip loading the load step")
)

type commitInfo struct {
	commits []*vcsinfo.LongCommit
	indices []int
}

func main() {
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
		watcher, err := NewRepoWatcher(repoURL, useDataDir)
		if err != nil {
			sklog.Fatalf("Error initializing repo watcher: %s", err)
		}
		go watcher.Start(ctx)
	}

	select {}
}
