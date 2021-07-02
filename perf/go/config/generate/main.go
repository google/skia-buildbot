// Program to generate JSON Schema definitions for the InstanceConfig structs.
//
//go:generate go run .
package main

import (
	"encoding/json"
	"io"
	"log"

	"github.com/alecthomas/jsonschema"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/config"
)

func main() {
	b, err := json.MarshalIndent(jsonschema.Reflect(&config.InstanceConfig{}), "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	err = util.WithWriteFile("../validate/instanceConfigSchema.json", func(w io.Writer) error {
		_, err := w.Write(b)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}
