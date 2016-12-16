// Application that executes a command on all CT bare-metal workers and optionally prints
// their outputs.
package main

import (
	"flag"
	"fmt"
	"time"

	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	cmd         = flag.String("cmd", "hostname", "Command to execute on CT bare-metal slaves")
	timeout     = flag.Duration("timeout", 10*time.Second, "Duration after which the cmd will timeout")
	printOutput = flag.Bool("print_output", true, "Whether output of command from CT bare-metal slaves should be printed")
)

func main() {
	defer common.LogPanic()
	common.Init()
	out, err := util.SshToBareMetalMachines(*cmd, util.BareMetalSlaves, *timeout)
	if err != nil {
		sklog.Fatal(err)
	}
	if *printOutput {
		for k, v := range out {
			fmt.Printf("\n=====%s=====\n%s\n", k, v)
		}
	}
}
