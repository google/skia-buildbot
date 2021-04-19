//go:generate go run . -o ../../../../modules/rpc_types.ts

package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expectations"
	"go.skia.org/infra/golden/go/search/common"
	search_frontend "go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/status"
	"go.skia.org/infra/golden/go/tiling"
	"go.skia.org/infra/golden/go/web/frontend"
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
	// Ensure go2ts sees the ParamSet type for the first time with the go2ts:"ignorenil" annotation.
	type ignoreNil struct {
		ParamSet paramtools.ParamSet `go2ts:"ignorenil"`
	}
	generator.AddWithName(ignoreNil{}, "IgnoreNil_DO_NOT_USE")

	// Response for the /json/v1/changelist/{system}/{id} RPC endpoint.
	generator.AddWithName(frontend.ChangelistSummary{}, "ChangelistSummaryResponse")

	// Response for the /json/v1/paramset RPC endpoint.
	generator.AddWithName(tiling.Tile{}.ParamSet, "ParamSetResponse")

	// Response for the /json/v1/search RPC endpoint.
	generator.AddWithName(search_frontend.SearchResponse{}, "SearchResponse")

	// Request for the /json/v1/triage RPC endpoint.
	generator.Add(frontend.TriageRequest{})

	// Response for the /json/v1/trstatus RPC endpoint.
	generator.AddWithName(status.GUIStatus{}, "StatusResponse")

	// Response for the /json/v1/byblame RPC endpoint.
	generator.Add(frontend.ByBlameResponse{})

	generator.AddUnionWithName(expectations.AllLabel, "Label")
	generator.AddUnionWithName(common.AllRefClosest, "RefClosest")
}
