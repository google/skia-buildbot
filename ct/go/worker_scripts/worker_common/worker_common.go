/*
	Common initialization for worker scripts.
*/

package worker_common

import (
	"flag"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	Local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func Init() {
	common.Init()
	if *Local {
		util.SetVarsForLocal()
	}
}
