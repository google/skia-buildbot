/*
	Common initialization for master scripts.
*/

package master_common

import (
	"context"
	"flag"
	"fmt"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cas"
	"go.skia.org/infra/go/cas/rbe"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/luciauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"google.golang.org/api/compute/v1"
)

var (
	// Local indicates whether we are running locally, as opposed to in
	// production.
	Local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

// Init initializes common master tasks and returns an authenticated swarming client.
func Init(appName string) (swarming.ApiClient, cas.CAS, error) {
	common.InitWithMust(appName)
	initRest()

	// Use task based authentication and Luci context.
	ts, err := luciauth.NewLUCIContextTokenSource(auth.SCOPE_FULL_CONTROL, compute.CloudPlatformScope)
	if err != nil {
		return nil, nil, fmt.Errorf("Could not get token source: %s", err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	swarmClient, err := swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	casClient, err := rbe.NewClient(context.TODO(), rbe.InstanceChromeSwarming, ts)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	return swarmClient, casClient, nil
}

func initRest() {
	if *Local {
		util.SetVarsForLocal()
	}
}
