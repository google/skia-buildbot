// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
//
//go:generate bazelisk run //:go -- run . -o ../../../modules/json/index.ts
package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"

	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/am/go/note"
	"go.skia.org/infra/am/go/silence"
	"go.skia.org/infra/am/go/types"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	generator.AddIgnoreNil(paramtools.Params{})
	generator.AddIgnoreNil(paramtools.ParamSet{})
	generator.AddMultiple(
		incident.Incident{},
		silence.Silence{},
		note.Note{},
		types.RecentIncidentsResponse{},
		types.StatsRequest{},
		types.StatsResponse{},
		types.IncidentsResponse{},
		types.IncidentsInRangeRequest{},
		types.AuditLog{},
	)

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
