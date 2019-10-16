// Simple command line app the applies our image diff library to two PNGs.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"log"
	"os"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

var (
	out = flag.String("out", "", "Filename to write the diff image to.")
)

func main() {
	common.Init()
	if flag.NArg() != 2 {
		log.Fatal("Usage: imagediff [--out filename] imagepath1.png imagepath2.png\n")
	}
	a, err := openNRGBAFromFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	b, err := openNRGBAFromFile(flag.Arg(1))
	if err != nil {
		log.Fatal(err)
	}
	metrics, d := diff.PixelDiff(a, b)
	fmt.Printf("Dimensions are different: %v\n", metrics.DimDiffer)
	fmt.Printf("Number of pixels different: %v\n", metrics.NumDiffPixels)
	fmt.Printf("Pixel diff percent: %v\n", metrics.PixelDiffPercent)
	fmt.Printf("Max RGBA: %v\n", metrics.MaxRGBADiffs)
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

func openNRGBAFromFile(fileName string) (*image.NRGBA, error) {
	var img *image.NRGBA
	err := util.WithReadFile(fileName, func(r io.Reader) error {
		im, err := png.Decode(r)
		if err != nil {
			return err
		}
		img = diff.GetNRGBA(im)
		return nil
	})
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return img, nil
}
