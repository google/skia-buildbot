// query_buidapi is a simple command-line application to test the androidbuildinternal API.
package main

import (
	"fmt"
	"net/http"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/android_ingest/go/buildapi"
	androidbuildinternal "go.skia.org/infra/go/androidbuildinternal/v2beta1"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
)

func main() {
	common.Init()
	// Create a new auth'd client.
	client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, androidbuildinternal.AndroidbuildInternalScope)
	if err != nil {
		glog.Fatalf("Unable to create authenticated client: %s", err)
	}
	// Create a new API.
	api, err := buildapi.NewAPI(client)
	if err != nil {
		glog.Fatalf("Failed to create client: %s", err)
	}
	// List all the buildids that come after the given buildid.
	builds, err := api.List("git_master-skia", 3529135)
	if err != nil {
		glog.Fatalf("Failed to retrieve builds: %s", err)
	}
	for _, b := range builds {
		fmt.Printf("%d %d\n", b.BuildId, b.TS)
	}
}
