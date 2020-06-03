/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"
	"fmt"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/swarming"
)

var (
	Local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

// Init initializes common master tasks and returns an authenticated swarming client.
func Init(appName string) (swarming.ApiClient, error) {
	common.InitWithMust(appName)
	initRest()

	// Use task based authentication and Luci context.
	ts, err := auth.NewLUCIContextTokenSource(auth.SCOPE_FULL_CONTROL)
	if err != nil {
		return nil, fmt.Errorf("Could not get token source: %s", err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	return swarming.NewApiClient(httpClient, swarming.SWARMING_SERVER_PRIVATE)
}

// func InitWithMetrics2(appName string, promPort *string) {
// 	common.InitWithMust(
// 		appName,
// 		common.PrometheusOpt(promPort),
// 		common.MetricsLoggingOpt(),
// 	)
// 	initRest()
// }

// remove lots of things from here.
func initRest() {
	if *Local {
		util.SetVarsForLocal()
	}
}
