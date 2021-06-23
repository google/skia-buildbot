package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/sk/go/asset"
	release_branch "go.skia.org/infra/sk/go/release-branch"
	"go.skia.org/infra/sk/go/try"
)

func main() {
	// Make sklog happy so it doesn't log errors.
	flag.Parse()
	exec.WriteInfoLog = exec.WriteLog{
		LogFunc: func(format string, args ...interface{}) {
			_, _ = fmt.Fprintf(os.Stdout, format, args...)
		},
	}
	exec.WriteWarningLog = exec.WriteLog{
		LogFunc: func(format string, args ...interface{}) {
			_, _ = fmt.Fprintf(os.Stderr, format, args...)
		},
	}

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
