package main

import (
	"go.skia.org/infra/fiddle/go/buildsecwrap"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
)

func main() {
	common.Init()
	if err := buildsecwrap.Build("/tmp"); err != nil {
		sklog.Fatalf("Failed to build: %s", err)
	}
}
