package main

import (
	"image"
	"image/png"
	"log"
	"os"

	"github.com/skia-dev/glog"
)

type FloatImage struct {
	Pix    []float64
	width  int
	height int
}

type FloatGrayImage struct {
	Pix    []float64
	width  int
	height int
}

func (img *FloatImage) Get(x, y int) (r, g, b, a float64) {
	index := 4 * (y*img.width + x)
	return img.Pix[index+0], img.Pix[index+1], img.Pix[index+2], img.Pix[index+3]
}

func (img *FloatGrayImage) Get(x, y int) (L float64) {
	index := (y*img.width + x)
	return img.Pix[index]
}

func MakeFloatImage(width, height int) (result *FloatImage) {
	return &FloatImage{
		width:  width,
		height: height,
		Pix:    make([]float64, width*height*4),
	}
}

func MakeFloatGrayImage(width, height int) (result *FloatGrayImage) {
	return &FloatGrayImage{
		width:  width,
		height: height,
		Pix:    make([]float64, width*height),
	}
}

func CopyImageToFloat(img image.Image) (result *FloatImage) {
	src := toNRGBA(img)
	result = MakeFloatImage(src.Bounds().Max.X, src.Bounds().Max.Y)
	parallel(result.height, func(partStart, partEnd int) {
		for y := partStart; y < partEnd; y++ {
			for x := 0; x < result.width; x++ {
				index := 4 * (y*result.width + x)
				result.Pix[index+0] = float64(src.Pix[index+0]) / 255.0
				result.Pix[index+1] = float64(src.Pix[index+1]) / 255.0
				result.Pix[index+2] = float64(src.Pix[index+2]) / 255.0
				result.Pix[index+3] = float64(src.Pix[index+3]) / 255.0
			}
		}
	})
	return
}

func (img *FloatImage) ToNRGBA() (result *image.NRGBA) {
	result = image.NewNRGBA(image.Rect(0, 0, img.width, img.height))

	parallel(img.height, func(partStart, partEnd int) {
		for y := partStart; y < partEnd; y++ {
			for x := 0; x < img.width; x++ {
				index := 4 * (y*img.width + x)
				result.Pix[index+0] = float_clamp(img.Pix[index+0])
				result.Pix[index+1] = float_clamp(img.Pix[index+1])
				result.Pix[index+2] = float_clamp(img.Pix[index+2])
				result.Pix[index+3] = float_clamp(img.Pix[index+3])
			}
		}
	})
	return
}

func (img *FloatGrayImage) ToNRGBA() (result *image.NRGBA) {
	result = image.NewNRGBA(image.Rect(0, 0, img.width, img.height))

	parallel(img.height, func(partStart, partEnd int) {
		for y := partStart; y < partEnd; y++ {
			for x := 0; x < img.width; x++ {
				dst_index := 4 * (y*img.width + x)
				src_index := y*img.width + x
				result.Pix[dst_index+0] = float_clamp(img.Pix[src_index])
				result.Pix[dst_index+1] = float_clamp(img.Pix[src_index])
				result.Pix[dst_index+2] = float_clamp(img.Pix[src_index])
				result.Pix[dst_index+3] = 255
			}
		}
	})
	return
}

func (img *FloatImage) Set(x, y int, r, g, b, a float64) {
	index := 4 * (y*img.width + x)
	img.Pix[index+0] = r
	img.Pix[index+1] = g
	img.Pix[index+2] = b
	img.Pix[index+3] = a
}

func (img *FloatGrayImage) Set(x, y int, L float64) {
	index := (y*img.width + x)
	img.Pix[index] = L
}

func (src *FloatImage) Dump(fname string) {
	img := src.ToNRGBA()
	file, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		glog.Errorf("Failed to encode: %s", err)
	}
}

func (src *FloatGrayImage) Dump(fname string) {
	img := src.ToNRGBA()
	file, err := os.Create(fname)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(file, img); err != nil {
		glog.Errorf("Failed to encode: %s", err)
	}
}
