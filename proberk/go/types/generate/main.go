// Program to generate JSON Schema definitions for the Probe struct.
//
//go:generate bazelisk run //:go -- run .
package main

import (
	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/proberk/go/types"
)

func main() {
	jsonschema.GenerateSchema("../probesSchema.json", &types.Probes{})
}
