package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/rpc_types"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	outputPath = flag.String("o", "", "Path to the output TypeScript file.")
)

func addTypes(generator *go2ts.Go2TS) error {
	// Response for the /r/{roller}/json/status RPC endpoint.
	if err := generator.AddWithName(rpc_types.AutoRollStatus{}, "AutoRollStatus"); err != nil {
		return skerr.Wrap(err)
	}
	// Response for the /r/{roller}/json/ministatus RPC endpoint.
	if err := generator.AddWithName(rpc_types.AutoRollMiniStatus{}, "AutoRollMiniStatus"); err != nil {
		return skerr.Wrap(err)
	}
	// Response for the /json/all RPC endpoint.
	if err := generator.AddWithName(rpc_types.AutoRollMiniStatuses{}, "AutoRollMiniStatuses"); err != nil {
		return skerr.Wrap(err)
	}
	// Request for the /r/{roller}/json/mode RPC endpoint.
	if err := generator.AddWithName(rpc_types.AutoRollModeChangeRequest{}, "AutoRollModeChangeRequest"); err != nil {
		return skerr.Wrap(err)
	}
	// Request for the /r/{roller}/json/strategy RPC endpoint.
	if err := generator.AddWithName(rpc_types.AutoRollStrategyChangeRequest{}, "AutoRollStrategyChangeRequest"); err != nil {
		return skerr.Wrap(err)
	}
	// Request/response for the /r/{roller}/json/manual RPC endpoint.
	if err := generator.AddWithName(manual.ManualRollRequest{}, "AutoRollManualRollRequest"); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func main() {
	common.Init()
	generator := go2ts.New()
	if err := addTypes(generator); err != nil {
		sklog.Fatal(err)
	}
	if err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	}); err != nil {
		sklog.Fatal(err)
	}
}
