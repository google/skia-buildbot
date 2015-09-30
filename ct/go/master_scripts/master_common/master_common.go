/*
	Common initialization for master scripts.
*/

package master_common

import (
	"flag"
	"fmt"
	"path/filepath"

	"go.skia.org/infra/ct/go/frontend"
	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	Local         = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	localFrontend = flag.String("local_frontend", "http://localhost:8000/", "When local is true, base URL where CTFE is running.")
)

func Init() {
	common.Init()
	initRest()
}

func InitWithMetrics(appName string, graphiteServer *string) {
	common.InitWithMetrics(appName, graphiteServer)
	initRest()
}

func initRest() {
	if *Local {
		frontend.InitForTesting(*localFrontend)
		util.SetVarsForLocal()
	} else {
		frontend.MustInit()
	}
}

func WorkerSetupCmds() []string {
	if *Local {
		return []string{
			fmt.Sprintf("export GOPATH=%s;", filepath.Join(util.RepoDir, "go")),
			"export PATH=$GOPATH/bin:$PATH;",
		}
	} else {
		return []string{
			fmt.Sprintf("cd %s;", util.CtTreeDir),
			"git pull;",
			"make all;",
		}
	}
}
