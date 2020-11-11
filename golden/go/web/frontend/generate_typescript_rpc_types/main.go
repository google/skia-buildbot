package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
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
	if err := addTypes(generator); err != nil {
		sklog.Fatal(err)
	}

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}

func addTypes(generator *go2ts.Go2TS) error {

	// Ensure go2ts sees the ParamSet type for the first time with the go2ts:"ignorenil" annotation.
	type ignoreNil struct {
		ParamSet paramtools.ParamSet `go2ts:"ignorenil"`
	}
	if err := generator.AddWithName(ignoreNil{}, "IgnoreNil_DO_NOT_USE"); err != nil {
		return skerr.Wrap(err)
	}

	// Response for the /json/v1/changelist/{system}/{id} RPC endpoint.
	if err := generator.AddWithName(frontend.ChangeListSummary{}, "ChangeListSummaryResponse"); err != nil {
		return skerr.Wrap(err)
	}

	// Response for the /json/v1/paramset RPC endpoint.
	if err := generator.AddWithName(tiling.Tile{}.ParamSet, "ParamSetResponse"); err != nil {
		return skerr.Wrap(err)
	}

	// Response for the /json/v1/search RPC endpoint.
	if err := generator.AddWithName(search_frontend.SearchResponse{}, "SearchResponse"); err != nil {
		return skerr.Wrap(err)
	}

	// Request for the /json/v1/triage RPC endpoint.
	if err := generator.Add(frontend.TriageRequest{}); err != nil {
		return skerr.Wrap(err)
	}

	// Response for the /json/v1/trstatus RPC endpoint.
	if err := generator.AddWithName(status.GUIStatus{}, "StatusResponse"); err != nil {
		return skerr.Wrap(err)
	}

	if err := generator.AddUnionWithName(expectations.AllLabel, "Label"); err != nil {
		return skerr.Wrap(err)
	}

	if err := generator.AddUnionWithName(common.AllRefClosest, "RefClosest"); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}
