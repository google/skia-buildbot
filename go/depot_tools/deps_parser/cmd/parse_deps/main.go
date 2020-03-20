package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/depot_tools/deps_parser"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	file = flag.String("deps", "DEPS", "DEPS file to parse.")
)

func main() {
	common.Init()

	if err := util.WithReadFile(*file, func(r io.Reader) error {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}
		deps, err := deps_parser.ParseDeps(string(b))
		if err != nil {
			return err
		}
		for _, dep := range deps {
			fmt.Println(fmt.Sprintf("%s: %s @ %s", dep.Path, dep.Id, dep.Version))
		}
		return nil
	}); err != nil {
		sklog.Fatal(err)
	}
}
