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
	client, err := auth.NewJWTServiceAccountClient("", "", &http.Transport{Dial: httputils.DialTimeout}, androidbuildinternal.AndroidbuildInternalScope)
	if err != nil {
		glog.Fatalf("Unable to create authenticated client: %s", err)
	}
	api, err := buildapi.NewAPI(client)
	if err != nil {
		glog.Fatalf("Failed to create client: %s", err)
	}
	builds, err := api.List("git_master-skia", 3564425)
	if err != nil {
		glog.Fatalf("Failed to retrieve builds: %s", err)
	}
	glog.Infof("%#v", builds)
	for _, b := range builds {
		fmt.Printf("%d %d\n", b.BuildId, b.TS)
	}
}
