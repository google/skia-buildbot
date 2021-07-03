// Program to generate JSON Schema definitions for the InstanceConfig struct.
//
//go:generate go run .
package main

import (
	"encoding/json"
	"io"

	"github.com/alecthomas/jsonschema"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
)

func main() {
	b, err := json.MarshalIndent(jsonschema.Reflect(&config.InstanceConfig{}), "", "  ")
	if err != nil {
		sklog.Fatal(err)
	}
	err = util.WithWriteFile("../instanceConfigSchema.json", func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
