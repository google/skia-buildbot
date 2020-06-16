package main

import (
	"flag"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/frontend"
)

func main() {
	var flags config.Flags
	var fs flag.FlagSet
	fs.Init("frontend", flag.ContinueOnError)
	flags.Register(&fs)
	f, err := frontend.New(&flags, &fs)
	if err != nil {
		sklog.Fatal(err)
	}
	f.Serve()
}
