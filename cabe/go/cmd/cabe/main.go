// package main is the main executable for the cabe cli interface.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/nooplogging"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"
	"go.skia.org/infra/go/tracing/loggingtracer"

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
	loggingFlag := &cli.BoolFlag{
		Name:  "logging",
		Value: false,
		Usage: "Turn on logging while running commands.",
	}
	logTracingFlag := &cli.BoolFlag{
		Name:  "log-tracing",
		Value: false,
		Usage: "Turn on tracing while running commands. Will print trace logs to stdout.",
	}

	var mainSpan *trace.Span

	app := &cli.App{
		Name:        "cabe",
		Usage:       "Command-line tool for working with cabe",
		Description: "cli tools for analyzing and debugging pinpoint A/B experiment tryobs using cabe",
		Flags: []cli.Flag{
			loggingFlag,
			logTracingFlag,
		},
		Before: func(c *cli.Context) error {
			if c.Bool(loggingFlag.Name) {
				sklogimpl.SetLogger(stdlogging.New(os.Stderr))
			} else {
				sklogimpl.SetLogger(nooplogging.New())
			}
			if c.Bool(logTracingFlag.Name) {
				loggingtracer.Initialize()
				c.Context, mainSpan = trace.StartSpan(c.Context, "cabe cli main")
			}
			return nil
		},
		After: func(c *cli.Context) error {
			mainSpan.End()
			return nil
		},
		Commands: []*cli.Command{
			cabecli.AnalyzeCommand(),
			cabecli.CheckCommand(),
			cabecli.ReadCASCommand(),
			cabecli.SandwichCommand(),
			{
				Name:     "markdown",
				HideHelp: true,
				Usage:    "Generates markdown help for cabe.",
				Action: func(c *cli.Context) error {
					body, err := c.App.ToMarkdown()
					if err != nil {
						return skerr.Wrap(err)
					}
					fmt.Println(body)
					return nil
				},
			},
		},
	}
	if err := app.Run(os.Args); err != nil {
		_, _ = fmt.Fprintf(app.ErrWriter, "Error: %s\n", err)
		os.Exit(1)
	}
}
