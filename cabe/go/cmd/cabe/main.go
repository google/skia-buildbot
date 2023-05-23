// package main is the main executable for the cabe cli interface.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	cabecli "go.skia.org/infra/cabe/go/cmd/cabe/cli"
)

func init() {
	// Workaround for "ERROR: logging before flag.Parse" messages that show
	// up due to some transitive dependency on glog (we don't use it directly).
	// See: https://github.com/kubernetes/kubernetes/issues/17162
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	_ = fs.Parse([]string{})
	flag.CommandLine = fs
}

func main() {
	app := &cli.App{
		Name:        "cabe cli",
		Description: "cabe cli provides cli tools for debugging analyzer process",
		Commands: []*cli.Command{
			cabecli.CheckCommand(),
		},
	}
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(app.ErrWriter, "Error: %s\n", err)
		os.Exit(1)
	}
}
