// query_buidapi is a simple command-line application to test the androidbuildinternal API.
package main

import (
	"flag"
	"fmt"

	"go.skia.org/infra/android_ingest/go/buildapi"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

var (
	buildid = flag.Int64("buildid", 3529135, "Return all buildids newer than this.")
)

func main() {
	common.Init()
	// Create a new auth'd client.
	ts, err := auth.NewJWTServiceAccountTokenSource("", "", androidbuildinternal.AndroidbuildInternalScope)
	if err != nil {
		sklog.Fatalf("Unable to create authenticated token source: %s", err)
	}
	client := httputils.DefaultClientConfig().WithoutRetries().WithTokenSource(ts).Client()
	// Create a new API.
	api, err := buildapi.NewAPI(client)
	if err != nil {
		sklog.Fatalf("Failed to create client: %s", err)
	}

	buildid, timestamp, err := api.GetMostRecentBuildID()
	if err != nil {
		sklog.Fatalf("Failed to retrieve builds: %s", err)
	}

	branch, err := api.GetBranchFromBuildID(buildid)
	if err != nil {
		sklog.Fatalf("Failed to retrieve branch: %s", err)
	}
	fmt.Printf("%d %d %s\n", buildid, timestamp, branch)

}
