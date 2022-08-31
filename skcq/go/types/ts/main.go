// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
//
//go:generate bazelisk run //:go -- run . -o ../../../modules/json/index.ts
package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/skcq/go/types"
)

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	generator.AddMultiple(
		types.CurrentlyProcessingChange{},
		types.GetCurrentChangesRequest{},
		types.GetCurrentChangesResponse{},
		types.ChangeAttempts{},
		types.GetChangeAttemptsRequest{},
		types.GetChangeAttemptsResponse{},
	)
	generator.AddUnionWithName(types.AllVerifierStates, "VerifierState")

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
