// Simple command line app that compares the images based on their digests.
// This is a simple standalone client to the skia_image_server.
// Primarily used for debugging.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diffstore"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/grpc"
)

var (
	grpcAddr = flag.String("grpc_address", "localhost:9000", "gRPC service address (e.g., ':9000')")
)

func main() {
	common.Init()
	if flag.NArg() < 2 {
		sklog.Fatalf("Usage: %s digest1 digest2 [digest3 ... digestN]\n", os.Args[0])
	}

	args := flag.Args()
	mainDigest := types.Digest(args[0])
	rightDigests := make(types.DigestSlice, 0, len(args)-1)
	for _, d := range args[1:] {
		rightDigests = append(rightDigests, types.Digest(d))
	}

	// Create the client connection and connect to the server.
	conn, err := grpc.Dial(*grpcAddr, grpc.WithInsecure())
	if err != nil {
		sklog.Fatalf("Unable to connect to grpc service: %s", err)
	}

	diffStore, err := diffstore.NewNetDiffStore(context.Background(), conn, "")
	if err != nil {
		sklog.Fatalf("Unable to initialize NetDiffStore: %s", err)
	}

	diffResult, err := diffStore.Get(context.Background(), mainDigest, rightDigests)
	if err != nil {
		sklog.Fatalf("Unable to compare digests: %s", err)
	}

	for _, rDigest := range rightDigests {
		fmt.Printf("%s <-> %s\n", mainDigest, rDigest)
		metrics := diffResult[rDigest]
		fmt.Printf("    Dimensions are different: %v\n", metrics.DimDiffer)
		fmt.Printf("    Number of pixels different: %v\n", metrics.NumDiffPixels)
		fmt.Printf("    Pixel diff percent: %v\n", metrics.PixelDiffPercent)
		fmt.Printf("    Max RGBA: %v\n", metrics.MaxRGBADiffs)
	}
}
