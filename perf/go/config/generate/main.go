// Program to generate JSON Schema definitions for the InstanceConfig struct.
//
//go:generate go run .
package main

import (
	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/perf/go/config"
)

func main() {
	jsonschema.GenerateSchema("../instanceConfigSchema.json", &config.InstanceConfig{})
}
