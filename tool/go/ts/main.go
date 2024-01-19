// Program to generate TypeScript definition files for Golang structs that are
// serialized to JSON for the web UI.
//
//go:generate bazelisk run --config=mayberemote //:go -- run . -o ../../modules/json/index.ts
package main

import (
	"flag"
	"io"

	"go.skia.org/infra/go/go2ts"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/tool/go/tool"
	"go.skia.org/infra/tool/go/types"
)

type unionAndName struct {
	v        interface{}
	typeName string
}

func addMultipleUnions(generator *go2ts.Go2TS, unions []unionAndName) {
	for _, u := range unions {
		generator.AddUnionWithName(u.v, u.typeName)
	}
}

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	addMultipleUnions(generator, []unionAndName{
		{tool.AllAdoptionStages, "AdoptionStage"},
		{tool.AllAudienceValues, "Audiences"},
		{tool.AllPhases, "Phases"},
		{tool.AllDomains, "Domains"},
	})

	generator.AddMultiple(generator,
		tool.Tool{},
		types.CreateOrUpdateResponse{},
	)

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
