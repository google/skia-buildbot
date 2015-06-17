package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/png"
	"log"
	"os"

	"go.skia.org/infra/go/util"
)

var options struct {
	verbose       bool
	debug         bool
	output_fname  string
	fov           float64
	threshold     int
	gamma         float64
	luminance     float64
	luminanceOnly bool
	colorFactor   float64
	downsample    int
	output        *FloatGrayImage
}

func loadImages(files []string) []image.Image {
	images := make([]image.Image, len(files))

	for i, file := range files {
		if options.verbose {
			log.Println("Trying to load", file)
		}
		reader, err := os.Open(file)
		if err != nil {
			log.Fatal(err)
		}
		defer util.Close(reader)
		images[i], _, err = image.Decode(reader)
		if err != nil {
			log.Fatal(err)
		}
	}

	return images
}

func main() {
	flag.BoolVar(&options.verbose, "verbose", false, "Print a bunch of debugging information as we run")
	flag.BoolVar(&options.debug, "debug", false, "Dump intermediate images for debugging")
	flag.IntVar(&options.threshold, "threshold", 100, "Number of pixels below which differences are ignored")
	flag.Float64Var(&options.gamma, "gamma", 2.2, "Value to convert rgb into linear space")
	flag.Float64Var(&options.luminance, "luminance", 100.0, "White luminance (default 100 cdm^-2)")
	flag.BoolVar(&options.luminanceOnly, "luminanceOnly", false, "Only consider luminance; ignore color in the comparision")
	flag.Float64Var(&options.colorFactor, "colorFactor", 1.0, "How much of color to use (0.0 = ignore color, 1.0 = use it all)")
	flag.IntVar(&options.downsample, "downsample", 0, "How many powers of 2 to down sample the images")
	flag.StringVar(&options.output_fname, "output", "", "Write differences to the given filename")
	flag.Float64Var(&options.fov, "fov", 45.0, "Field of View subtended by the image (0.1 to 89.9)")

	flag.Parse()

	files := flag.Args()

	if len(files) != 2 {
		fmt.Println("I need two files to compare (I got", len(files), "files)")
		os.Exit(1)
	}

	if options.verbose {
		log.Println("I'm going to compare", files[0], "and", files[1])
	}

	images := loadImages(files)

	options.output = nil

	if options.output_fname != "" {
		options.output = MakeFloatGrayImage(images[0].Bounds().Max.X, images[0].Bounds().Max.Y)
	}

	if options.verbose {
		log.Println("Everything looks good, let's do the compare.")
	}

	result, num_pixels_different := Yee_Compare(images[0], images[1])

	if result {
		log.Println("Image compare succeeded!")
	} else {
		log.Println("Image compare failed.")
	}

	if num_pixels_different > 0 && options.output != nil {
		log.Printf("Writing differing pixels to %s", options.output_fname)
		options.output.Dump(options.output_fname)
	}

}
