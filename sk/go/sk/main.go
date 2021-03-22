package main

import (
	"github.com/urfave/cli/v2"

	"go.skia.org/infra/sk/go/try"
)

func main() {
	app := &cli.App{
		Name:  "sk",
		Usage: `sk provides developer workflow tools for Skia.`,
		Commands: []*cli.Command{
			try.Command(),
		},
	}
	app.RunAndExitOnError()
}
