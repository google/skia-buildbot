//go:generate bazelisk run //:go -- run . -o ../../modules/rpc_types.ts

package main

import (
	"flag"
	"io"

	"github.com/skia-dev/go2ts"
	"go.skia.org/infra/codesize/go/codesizeserver/rpc"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

func main() {
	var outputPath = flag.String("o", "", "Path to the output TypeScript file.")
	flag.Parse()

	generator := go2ts.New()
	generator.Add(rpc.BinaryRPCRequest{})
	generator.Add(rpc.BinaryRPCResponse{})
	generator.Add(rpc.BinarySizeDiffRPCRequest{})
	generator.Add(rpc.BinarySizeDiffRPCResponse{})
	generator.Add(rpc.MostRecentBinariesRPCResponse{})

	err := util.WithWriteFile(*outputPath, func(w io.Writer) error {
		return generator.Render(w)
	})
	if err != nil {
		sklog.Fatal(err)
	}
}
