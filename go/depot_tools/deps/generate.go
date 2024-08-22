//go:build ignore
// +build ignore

package main

import (
	"go.skia.org/infra/go/depot_tools/generator"
)

func main() {
	generator.MustGenerate("../../../DEPS")
}
