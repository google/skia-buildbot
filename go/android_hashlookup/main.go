package main

// android_hashlookup is a test tool to verify in different environments
// (locally and on GCE) that looking up skia githashes via the android
// build API works.

import (
	"flag"
	"os"
	"time"

	"go.skia.org/infra/go/androidbuild"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	storage "google.golang.org/api/storage/v1"
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
		sklog.Errorf("Expected arguments: branch target buildID")
		sklog.Errorf("i.e.:  git_master-skia razor-userdebug 1772442")
		os.Exit(1)
	}

	// Set the arguments necessary to lookup the git hash.
	branch := args[0]
	target := args[1]
	buildID := args[2]
	sklog.Infof("Branch, target, buildID: %s, %s, %s", branch, target, buildID)

	ts, err := auth.NewDefaultTokenSource(*local, OAUTH_CACHE_FILEPATH, CLIENT_SECRET_FILEPATH, androidbuildinternal.AndroidbuildInternalScope, storage.CloudPlatformScope)
	if err != nil {
		sklog.Fatalf("Unable to create installed app oauth token source: %s", err)
	}
	// In this case we don't want a backoff transport since the Apiary backend
	// seems to fail a lot, so we basically want to fall back to polling if a
	// call fails.
	client := httputils.DefaultClientConfig().WithoutRetries().WithTokenSource(ts).Client()

	f, err := androidbuild.New("/tmp/android-gold-ingest", client)
	if err != nil {
		sklog.Fatalf("Failed to construct client: %s", err)
	}
	for {
		r, err := f.Get(branch, target, buildID)
		if err != nil {
			sklog.Errorf("Failed to get requested info: %s", err)
			time.Sleep(1 * time.Minute)
			continue
		}
		if r != nil {
			sklog.Infof("Successfully found: %#v", *r)
		}

		time.Sleep(1 * time.Minute)
	}
}
