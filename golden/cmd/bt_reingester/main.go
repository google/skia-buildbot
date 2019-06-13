package main

// bt_reingester will scan through all the files in a GCS bucket and ingest
// them into the bt_tracestore.
import (
	"context"
	"flag"
	"fmt"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/bt"
	"go.skia.org/infra/go/gitstore/bt_gitstore"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vcsinfo/bt_vcs"
	"go.skia.org/infra/golden/go/goldingestion"
	"go.skia.org/infra/golden/go/tracestore/bt_tracestore"
)

func main() {
	var (
		btInstance = flag.String("bt_instance", "production", "BigTable instance to use in the project identified by 'project_id'")
		projectID  = flag.String("project_id", "skia-public", "GCP project ID.")
		btTableID  = flag.String("bt_table_id", "", "BigTable table ID for the traces.")

		gitBTTableID = flag.String("git_table_id", "git-repos", "BigTable table ID that has the git data.")
		gitRepoURL   = flag.String("git_repo_url", "", "The URL of the git repo to look up in BigTable.")

		srcBucket  = flag.String("src_bucket", "", "Source bucket to ingest files from.")
		srcRootDir = flag.String("src_root_dir", "", "Source root directory to ingest files in.")
	)
	flag.Parse()

	bt.EnsureNotEmulator()

	tokenSrc, err := auth.NewDefaultTokenSource(true, storage.ScopeFullControl, pubsub.ScopePubSub, pubsub.ScopeCloudPlatform)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(tokenSrc).With2xxOnly().WithDialTimeout(time.Second * 10).Client()

	if *srcBucket == "" {
		sklog.Fatalf("You must supply --src_bucket")
	}

	gss, err := ingestion.NewGoogleStorageSource("", *srcBucket, *srcRootDir, client, nil)
	if err != nil {
		sklog.Fatalf("Could not set up GCS ingester for gs://%s/%s: %s", *srcBucket, *srcRootDir, err)
	}

	gtc := &bt_gitstore.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstance,
		TableID:    *gitBTTableID,
	}

	gitStore, err := bt_gitstore.New(context.Background(), gtc, *gitRepoURL)
	if err != nil {
		sklog.Fatalf("Error instantiating gitstore: %s", err)
	}

	// gittiles is nil because we don't need to fetch files
	// eventbus is nil because we don't want it to send events when new commits come in.
	vcs, err := bt_vcs.New(gitStore, "master", nil, nil, 0)
	if err != nil {
		sklog.Fatalf("Could not create VCS: %s", err)
	}

	btc := bt_tracestore.BTConfig{
		ProjectID:  *projectID,
		InstanceID: *btInstance,
		TableID:    *btTableID,
		VCS:        vcs,
	}

	processor, err := goldingestion.NewTraceStoreProcessor(btc)
	if err != nil {
		sklog.Fatalf("Cannot create bt trace store: %s", err)
	}

	sklog.Infof("Beginning ingestion at %s", beginning)
	results := gss.Poll(beginning.Unix(), time.Now().Unix())
	for r := range results {
		fmt.Print(".")
		err := processor.Process(context.Background(), r)
		if err != nil && err != ingestion.IgnoreResultsFileErr {
			sklog.Errorf("could not process %s: %s", r.Name(), err)
		}
	}

	sklog.Infof("done")
}

var beginning = time.Date(2019, time.January, 1, 0, 0, 0, 0, time.UTC)
