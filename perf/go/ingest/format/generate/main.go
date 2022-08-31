// Program to generate JSON Schema definitions for the InstanceConfig struct.
//
//go:generate bazelisk run //:go -- run .
package main

import (
	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/perf/go/ingest/format"
)

func main() {
	jsonschema.GenerateSchema("../formatSchema.json", &format.Format{})
}
