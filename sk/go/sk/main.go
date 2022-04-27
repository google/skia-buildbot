package main

import (
	"github.com/urfave/cli/v2"

	"go.skia.org/infra/sk/go/asset"
	release_branch "go.skia.org/infra/sk/go/release-branch"
	"go.skia.org/infra/sk/go/try"
)

func main() {
	app := &cli.App{
		Name:        "sk",
		Description: `sk provides developer workflow tools for Skia.`,
		Commands: []*cli.Command{
			asset.Command(),
			release_branch.Command(),
			try.Command(),
		},
		Usage: "sk <subcommand>",
	}
	app.RunAndExitOnError()
}
