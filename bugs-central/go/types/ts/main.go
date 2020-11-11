// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
package main

import (
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	generator := go2ts.New()
	err := generator.AddMultiple(
		types.IssueCountsData{},
		types.Issue{},
		types.ClientSourceQueryRequest{},
		types.IssuesOutsideSLOResponse{},
		types.GetClientsResponse{},
		types.GetClientCountsResponse{},
		types.GetChartsDataResponse{})
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
