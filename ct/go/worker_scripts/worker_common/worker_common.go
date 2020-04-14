/*
	Common initialization for worker scripts.
*/

package worker_common

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"runtime"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	Local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
)

func Init(ctx context.Context, useDepotTools bool) {
	common.Init()
	if *Local {
		util.SetVarsForLocal()
	} else {
		if runtime.GOOS == "windows" {
			// Set SystemRoot because of https://bugs.python.org/issue1384175#msg248951
			skutil.LogErr(os.Setenv("SystemRoot", "C:\\Windows"))
			util.DepotToolsDir = `C:\\Users\chrome-bot\depot_tools`
		}

		if useDepotTools {
			// Update depot_tools.
			skutil.LogErr(util.ExecuteCmd(ctx, filepath.Join(util.DepotToolsDir, "update_depot_tools"), []string{}, []string{}, util.UPDATE_DEPOT_TOOLS_TIMEOUT, nil, nil))
			// Add depot_tools to the PATH.
			skutil.LogErr(os.Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+util.DepotToolsDir))
		}

		if runtime.GOOS != "windows" {
			// Bring up Xvfb on workers (for GCE instances).
			if _, _, err := exec.RunIndefinitely(&exec.Command{
				Name:        "sudo",
				Args:        []string{"Xvfb", ":0", "-screen", "0", "1280x1024x24"},
				Env:         []string{},
				InheritPath: true,
				Timeout:     util.XVFB_TIMEOUT,
				LogStdout:   true,
				Stdout:      nil,
				LogStderr:   true,
				Stderr:      nil,
			}); err != nil {
				// CT's baremetal machines will already have an active display 0.
				sklog.Infof("Could not run Xvfb on Display 0: %s", err)
			}
		}
	}
}
