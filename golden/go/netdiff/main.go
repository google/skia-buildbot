// Simple command line app the applies our image diff library to two PNGs.
package main

import (
	"flag"
	"fmt"
	"os"

	"google.golang.org/grpc"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore"
)

var (
	imagePort = flag.String("image_port", ":9001", "Address that serves image files via HTTP.")
	grpcPort  = flag.String("grpc_port", ":9000", "gRPC service address (e.g., ':9000')")
)

func main() {
	defer common.LogPanic()
	common.Init()
	if flag.NArg() < 2 {
		sklog.Fatalf("Usage: %s digest1 digest2 [digest3 ... digestN]\n", os.Args[0])
	}

	args := flag.Args()
	mainDigest := args[0]
	rightDigests := args[1:]

	// Create the client connection and connect to the server.
	conn, err := grpc.Dial(*grpcPort, grpc.WithInsecure())
	if err != nil {
		sklog.Fatalf("Unable to connect to grpc service: %s", err)
	}

	diffStore, err := diffstore.NewNetDiffStore(conn)
	if err != nil {
		sklog.Fatalf("Unable to initialize NetDiffStore: %s", err)
	}

	diffResult, err := diffStore.Get(diff.PRIORITY_NOW, mainDigest, rightDigests)
	if err != nil {
		sklog.Fatalf("Unable to compare digests: %s", err)
	}

	for _, rDigest := range rightDigests {
		fmt.Printf("%s <-> %s\n", mainDigest, rDigest)
		metrics, ok := diffResult[rDigest]
		if ok {
			fmt.Printf("    Dimensions are different: %v\n", metrics.DimDiffer)
			fmt.Printf("    Number of pixels different: %v\n", metrics.NumDiffPixels)
			fmt.Printf("    Pixel diff percent: %v\n", metrics.PixelDiffPercent)
			fmt.Printf("    Max RGBA: %v\n", metrics.MaxRGBADiffs)
		} else {
			fmt.Printf("    ERR: No result available.")
		}
	}
}
