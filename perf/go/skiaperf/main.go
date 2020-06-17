package main

import (
	"github.com/spf13/pflag"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/frontend"
)

func main() {
	var flags config.FrontendFlags
	var fs pflag.FlagSet
	flags.Register(&fs)
	f, err := frontend.New(&flags, &fs)
	if err != nil {
		sklog.Fatal(err)
	}
	f.Serve()
}
