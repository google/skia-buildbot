// Application that executes a command on all CT workers and optionally prints
// their outputs.
package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/ct/go/util"
	"go.skia.org/infra/go/common"
)

var (
	cmd         = flag.String("cmd", "hostname", "Command to execute on CT slaves")
	timeout     = flag.Duration("timeout", 10*time.Second, "Duration after which the cmd will timeout")
	printOutput = flag.Bool("print_output", true, "Whether output of command from CT slaves should be printed")
)

func main() {
	common.Init()
	out, err := util.SSH(*cmd, util.Slaves, *timeout)
	if err != nil {
		glog.Fatal(err)
	}
	if *printOutput {
		for k, v := range out {
			fmt.Printf("\n=====%s=====\n%s\n", k, v)
		}
	}
}
