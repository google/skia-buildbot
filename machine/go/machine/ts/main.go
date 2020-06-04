package main

import (
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/machine/go/machine"
)

func main() {
	s := go2ts.New()
	s.Add(machine.Description{})

	util.WithWriteFile("./modules/json/index.ts", func(w io.Writer) error {
		s.Render(w)
		return nil
	})
}
