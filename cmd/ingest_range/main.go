package main

// skia_ingestion is the server process that runs an arbitary number of
// ingesters and stores them in traceDB backends.

import (
	"flag"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/gcs"

	gstorage "google.golang.org/api/storage/v1"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/gevent"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

// Command line flags.
var (
	eventTopic         = flag.String("event_topic", "", "The pubsub topic to use for distributed events.")
	local              = flag.Bool("local", false, "Running locally if true.")
	projectID          = flag.String("project_id", common.PROJECT_ID, "GCP project ID.")
	targetGCSPath      = flag.String("gs_path", "", "GS path, where the files to be ingested are located. Format: <bucket>/<path>.")
	serviceAccountFile = flag.String("service_account_file", "", "Credentials file for service account.")
)

func main() {
	common.Init()

	// serviceName uniquely identifies this host and app and is used as ID for other services.
	_, appName := filepath.Split(os.Args[0])
	nodeName, err := gevent.GetNodeName(appName, *local)
	if err != nil {
		sklog.Fatalf("Error getting unique service name: %s", err)
	}

	// Initialize oauth client and start the ingesters.
	client, err := auth.NewJWTServiceAccountClient("", *serviceAccountFile, nil, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}

	// Get the token source from the same service account. Needed to access cloud pubsub and datastore.
	tokenSource, err := auth.NewJWTServiceAccountTokenSource("", *serviceAccountFile, gstorage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Failed to authenticate service account to get token source: %s", err)
	}

	eventBus, err := gevent.New(*projectID, *eventTopic, nodeName, option.WithTokenSource(tokenSource))
	if err != nil {
		sklog.Fatalf("Unable to create global event client. Got error: %s", err)
	}

	// Set up the source, but don't plug in the eventbus since we don't want to listen to
	// storage events.
	bucketID, objectPrefix := gcs.SplitGSPath(*targetGCSPath)
	source, err := ingestion.NewGoogleStorageSource("polling-bucket", bucketID, objectPrefix, client, nil)
	if err != nil {
		sklog.Fatalf("Unable to open storage source: %s", err)
	}

	startTS := int64(0)
	endTS := int64(0)
	for rf := range source.Poll(startTS, endTS) {
		bucketID, objectID := rf.StorageIDs()
		eventBus.PublishStorageEvent(eventbus.NewStorageEvent(bucketID, objectID, rf.TimeStamp(), rf.MD5()))
		sklog.Infof("Triggered: %s", rf.Name())
	}
}
