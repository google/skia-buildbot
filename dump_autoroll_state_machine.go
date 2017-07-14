package main

import (
	"go.skia.org/infra/autoroll/go/state_machine"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

func main() {
	common.Init()
	if err := state_machine.DumpGraphviz("state_machine"); err != nil {
		sklog.Fatal(err)
	}
}
