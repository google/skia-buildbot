package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"path"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/common"
)

var (
	id      = flag.String("id", "", "ID of the build to retrieve.")
	workdir = flag.String("workdir", "workdir", "Working directory to use.")
)

func main() {
	defer common.LogPanic()
	common.Init()

	if *id == "" {
		glog.Fatal("ID is required.")
	}

	// Initialize the BuildBucket client.
	c, err := auth.NewClient(true, path.Join(*workdir, "oauth_token_cache"), buildbucket.DEFAULT_SCOPES...)
	if err != nil {
		glog.Fatal(err)
	}
	bb := buildbucket.NewClient(c)

	// Retrieve the build.
	build, err := bb.GetBuild(*id)
	if err != nil {
		glog.Fatal(err)
	}

	// Pretty print the build.
	b, err := json.Marshal(build)
	if err != nil {
		glog.Fatal(err)
	}
	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "\t"); err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Build: %s\n%s", build.Url, out.String())
}
