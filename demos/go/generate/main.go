//go:generate go run . -o ../../modules/rpc_types.ts

package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"

	"go.skia.org/infra/demos/go/frontend"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	addTypes(generator)

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}

func addTypes(generator *go2ts.Go2TS) {
	generator.Add(frontend.Metadata{})
}
