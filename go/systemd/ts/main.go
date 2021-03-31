// Program to generate a TypeScript definition of the systemd.UnitStatus Go type.
//
//go:generate go run . -o ../../../infra-sk/modules/systemd-unit-status-sk/json/index.ts
package main

import (
	"flag"
	"io"

	"github.com/coreos/go-systemd/v22/dbus"
	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/go/systemd"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	generator.AddWithName(dbus.UnitStatus{}, "DBusUnitStatus")
	generator.AddWithName(systemd.UnitStatus{}, "SystemdUnitStatus")

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
