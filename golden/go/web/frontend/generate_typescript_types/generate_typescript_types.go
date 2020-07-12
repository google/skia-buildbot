package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/status"
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
	// Response for the /json/search RPC endpoint.
	generator.Add(frontend.SearchResponse{})

	// Response for the /json/trstatus RPC endpoint.
	generator.AddWithName(status.GUIStatus{}, "StatusResponse")
}
