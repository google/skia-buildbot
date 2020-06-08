// Program to generate TypeScript definition files for Goland structs that are
// serialized to JSON for the web UI.
package main

import (
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/clustering2"
)

func main() {
	generator := go2ts.New()
	err := generator.Add(clustering2.ValuePercent{})
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
