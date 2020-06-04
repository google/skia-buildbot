// Program to generate TypeScript definition files for Goland structs that are
// serialized to JSON for the web UI.
package main

import (
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
)

func main() {
	s := go2ts.New()
	err := s.Add(machine.Description{})
	if err != nil {
		sklog.Fatal(err)
	}

	util.WithWriteFile("./modules/json/index.ts", func(w io.Writer) error {
		s.Render(w)
		return nil
	})
}
