package main

import (
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/frontend"
)

func main() {
	f, err := frontend.New()
	if err != nil {
		sklog.Fatal(err)
	}
	f.Serve()
}
