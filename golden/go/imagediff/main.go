// Simple command line app the applies our image diff library to two PNGs.
package main

import (
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/golden/go/diff"
)

var (
	out = flag.String("out", "", "Filename to write the diff image to.")
)

func main() {
	defer common.LogPanic()
	common.Init()
	if flag.NArg() != 2 {
		log.Fatal("Usage: imagediff [--out filename] imagepath1.png imagepath2.png\n")
	}
	a, err := diff.OpenImage(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	b, err := diff.OpenImage(flag.Arg(1))
	if err != nil {
		log.Fatal(err)
	}
	metrics, d := diff.Diff(a, b)
	fmt.Printf("Dimensions are different: %v\n", metrics.DimDiffer)
	fmt.Printf("Number of pixels different: %v\n", metrics.NumDiffPixels)
	fmt.Printf("Pixel diff percent: %v\n", metrics.PixelDiffPercent)
	if *out == "" {
		return
	} else {
		fmt.Println("Writing image diff.")
	}
	f, err := os.Create(*out)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(f, d); err != nil {
		log.Fatal(err)
	}
}
