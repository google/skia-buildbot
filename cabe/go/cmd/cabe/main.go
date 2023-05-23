// package main is the main executable for the cabe cli interface.
package main

import (
	"os"

	"github.com/urfave/cli/v2"
	cabecli "go.skia.org/infra/cabe/go/cmd/cabe/cli"
)

// flag names
const (
	pinpointJobIDFlagName = "pinpoint-job"
)

// flags
var pinpointJobIDFlag = &cli.StringFlag{
	Name:  pinpointJobIDFlagName,
	Value: "",
	Usage: "ID of the pinpoint job to check",
}

func main() {
	app := &cli.App{
		Name:        "cabe cli",
		Description: "cabe cli provides cli tools for debugging analyzer process",
		Commands: []*cli.Command{
			{
				Name:        "check",
				Description: "check runs some diagnostic checks on perf experiment jobs.",
				Usage:       "cabe_cli check -- --pinpoint-job <pinpoint-job>",
				Flags: []cli.Flag{
					pinpointJobIDFlag,
				},
				Action: func(ctx *cli.Context) error {
					return cabecli.Check(ctx.Context, ctx.String(pinpointJobIDFlagName))
				},
			},
		},
	}
	_ = app.Run(os.Args)
}
