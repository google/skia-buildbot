// Program to generate TypeScript definition files for Goland structs that are
// serialized to JSON for the web UI.
package main

import (
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/fiddlek/go/types"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func addMultiple(generator *go2ts.Go2TS, instances []interface{}) error {
	for _, inst := range instances {
		err := generator.Add(inst)
		if err != nil {
			return err
		}
	}
	return nil
}

type unionAndName struct {
	v        interface{}
	typeName string
}

func addMultipleUnions(generator *go2ts.Go2TS, unions []unionAndName) error {
	for _, u := range unions {
		if err := generator.AddUnionWithName(u.v, u.typeName); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	generator := go2ts.New()
	err := addMultiple(generator, []interface{}{
		types.Options{},
		types.Result{},
	})
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
