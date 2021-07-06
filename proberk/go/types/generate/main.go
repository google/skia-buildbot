// Program to generate JSON Schema definitions for the Probe struct.
//
//go:generate go run .
package main

import (
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/proberk/go/types"
)

func main() {
	jsonutils.GenerateSchema("../probesSchema.json", &types.Probes{})
}
