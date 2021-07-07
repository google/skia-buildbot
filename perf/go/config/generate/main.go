// Program to generate JSON Schema definitions for the InstanceConfig struct.
//
//go:generate go run .
package main

import (
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/perf/go/config"
)

func main() {
	jsonutils.GenerateSchema("../instanceConfigSchema.json", &config.InstanceConfig{})
}
