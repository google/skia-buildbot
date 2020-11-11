// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
package main

import (
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
)

func main() {
	generator := go2ts.New()
	err := generator.Add(machine.Description{})
	if err != nil {
		sklog.Fatal(err)
	}

	err = generator.AddUnion(machine.AllModes)
	if err != nil {
		sklog.Fatal(err)
	}

	err = util.WithWriteFile("./modules/json/index.ts", func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
