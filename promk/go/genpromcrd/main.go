// Package main implements the genpromcrd command line application.
package main

import (
	"os"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/promk/go/genpromcrd/genpromcrd"
)

func main() {
	app := genpromcrd.NewApp()

	if err := app.Main(os.Args); err != nil {
		sklog.Fatal(err)
	}
}
