package main

// android_hashlookup is a test tool to verify in different environments
// (locally and on GCE) that looking up skia githashes via the android
// build servers works.

import (
	"flag"
	"net/http"
	"os"

	storage "code.google.com/p/google-api-go-client/storage/v1"
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/androidbuild"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/util"
)

// TODO(stephana): Factor to take the target information (builID etc.)
// and the secrets files via flags.

const (
	OAUTH_CACHE_FILEPATH   = "oauth-token.cache"
	CLIENT_SECRET_FILEPATH = "client_secret.json"
)

var (
	local = flag.Bool("local", false, "Running locally if true, as opposed to running in GCE.")
)

func main() {
	common.Init()

	args := flag.Args()
	if len(args) != 3 {
		glog.Errorf("Expected arguments: branch target buildID")
		glog.Errorf("i.e.:  git_master-skia razor-userdebug 1772442")
		os.Exit(1)
	}

	// Set the arguments necessary to lookup the git hash.
	branch := args[0]
	target := args[1]
	buildID := args[2]
	glog.Infof("Branch, target, buildID: %s, %s, %s", branch, target, buildID)

	// Set up the oauth client.
	var client *http.Client
	var err error

	transport := util.NewBackOffTransport()

	if *local {
		// Use a local client secret file to load data.
		client, err = auth.InstalledAppClient(OAUTH_CACHE_FILEPATH, CLIENT_SECRET_FILEPATH,
			transport,
			androidbuildinternal.AndroidbuildInternalScope,
			storage.CloudPlatformScope)
		if err != nil {
			glog.Fatalf("Unable to create installed app oauth client:%s", err)
		}
	} else {
		// Use compute engine service account.
		client = auth.GCEServiceAccountClient(transport)
	}

	// Make sure storage access works.
	glog.Info("Starting simple storage test.")
	service, err := storage.New(client)
	if err != nil {
		glog.Fatalf("Storage error: %s", err)
	}

	buckets, err := service.Buckets.List("google.com:skia-buildbots").Do()
	if err != nil {
		glog.Fatalf("Storage error: %s", err)
	}

	for _, item := range buckets.Items {
		glog.Infof("Bucket: %v", item)
	}

	glog.Info("Storage test successful.")

	// Make sure we can look up a git hash from android builds.
	glog.Info("Starting githash lookup.")

	lookup, err := androidbuild.New(client)
	if err != nil {
		glog.Fatalf("Unable to create lookup client: %s", err)
	}

	// Do the lookup a few times, so we can see if subsequent calls are faster.
	for i := 0; i < 3; i++ {
		commit, err := lookup.FindCommit(branch, target, buildID, true)
		if err != nil {
			glog.Fatalf("Unable to find commit: %s", err)
		}
		glog.Infof("Commit:  %v", commit)
	}

	glog.Info("Successfully tested oauth and githash lookup.")
}
