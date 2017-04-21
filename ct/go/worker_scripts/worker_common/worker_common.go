/*
	Common initialization for worker scripts.
*/

package worker_common

import (
	"context"
	"flag"
	"os"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skexec"
	"go.skia.org/infra/go/sklog"
	skutil "go.skia.org/infra/go/util"
)

var (
	Local = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

	exec = skexec.NewExec()
)

func Init() {
	common.Init()
	if *Local {
		util.SetVarsForLocal()
	} else {
		// Add depot_tools to the PATH.
		skutil.LogErr(os.Setenv("PATH", os.Getenv("PATH")+":"+util.DepotToolsDir))
		// Add adb to the PATH.
		skutil.LogErr(os.Setenv("PATH", os.Getenv("PATH")+":/home/chrome-bot/KOT49H-hammerhead-userdebug-insecure"))
		// Bring up Xvfb on workers (for GCE instances).
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), util.XVFB_TIMEOUT)
			defer cancel()
			if err := exec.Run(&skexec.Command{
				Name:        "sudo",
				Args:        []string{"Xvfb", ":0", "-screen", "0", "1280x1024x24"},
				Env:         []string{},
				InheritPath: true,
				Context:     ctx,
				LogStdout:   true,
				Stdout:      nil,
				LogStderr:   true,
				Stderr:      nil,
			}); err != nil {
				// CT's baremetal machines will already have an active display 0.
				sklog.Infof("Could not run Xvfb on Display 0: %s", err)
			}
		}()
	}
}
