package main

import (
	"flag"
	"io/ioutil"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	url   = flag.String("url", "", "URL to request.")
)

func main() {
	common.Init()
	defer common.Defer()
	skiaversion.MustLogVersion()
	ts, err := auth.NewDefaultTokenSource(*local, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatal(err)
	}
	client := httputils.DefaultClientConfig().With2xxOnly().WithTokenSource(ts).Client()
	resp, err := client.Get(*url)
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		sklog.Fatal(err)
	}
	sklog.Infof("Response:\n%s", string(b))
}
