package main

import (
	"math"
)

func applyColorMapping(src *FloatImage, fn func(r, g, b, a float64) (outr, outg, outb, outa float64)) *FloatImage {
	width := src.width
	height := src.height
	dst := MakeFloatImage(width, height)

	parallel(height, func(partStart, partEnd int) {
		for y := partStart; y < partEnd; y++ {
			for x := 0; x < width; x++ {
				i := 4 * (y*src.width + x)
				j := 4 * (y*dst.width + x)

				r := src.Pix[i+0]
				g := src.Pix[i+1]
				b := src.Pix[i+2]
				a := src.Pix[i+3]

				newr, newg, newb, newa := fn(r, g, b, a)

				dst.Pix[j+0] = newr
				dst.Pix[j+1] = newg
				dst.Pix[j+2] = newb
				dst.Pix[j+3] = newa
			}
		}
	})

	return dst
}

func applyColorMappingGray(src *FloatImage, fn func(r, g, b, a float64) float64) *FloatGrayImage {
	width := src.width
	height := src.height
	dst := MakeFloatGrayImage(width, height)

	parallel(height, func(partStart, partEnd int) {
		for y := partStart; y < partEnd; y++ {
			for x := 0; x < width; x++ {
				i := 4 * (y*src.width + x)
				j := y*src.width + x

				r := src.Pix[i+0]
				g := src.Pix[i+1]
				b := src.Pix[i+2]
				a := src.Pix[i+3]

				y := fn(r, g, b, a)

				dst.Pix[j+0] = y
			}
		}
	})

	return dst
}

func AdjustGamma(img *FloatImage, gamma float64) *FloatImage {
	e := 1.0 / math.Max(gamma, 0.0001)

	fn := func(r, g, b, a float64) (float64, float64, float64, float64) {
		return math.Pow(r, e), math.Pow(g, e), math.Pow(b, e), a
	}

	return applyColorMapping(img, fn)
}

func RGBAToLAB(img *FloatImage) *FloatImage {
	fn := func(r, g, b, a float64) (float64, float64, float64, float64) {

		x, y, z := ConvertAdobeRGBToXYZ(r, g, b)
		l, a, b := ConvertXYZToLAB(x, y, z)

		return l, a, b, 1.0
	}

	return applyColorMapping(img, fn)
}

func RGBAToXYZ(img *FloatImage) *FloatImage {
	fn := func(r, g, b, a float64) (float64, float64, float64, float64) {

		x, y, z := ConvertAdobeRGBToXYZ(r, g, b)

		return x, y, z, 1
	}

	return applyColorMapping(img, fn)
}

func RGBAToY(img *FloatImage) *FloatGrayImage {
	fn := func(r, g, b, a float64) float64 {
		_, y, _ := ConvertAdobeRGBToXYZ(r, g, b)

		return y
	}

	return applyColorMappingGray(img, fn)
}

// convert Adobe RGB (1998) with reference white D65 to XYZ
func ConvertAdobeRGBToXYZ(r, g, b float64) (x, y, z float64) {
	// matrix is from http://www.brucelindbloom.com/
	x = r*0.576700 + g*0.185556 + b*0.188212
	y = r*0.297361 + g*0.627355 + b*0.0752847
	z = r*0.0270328 + g*0.0706879 + b*0.991248
	return
}

var xWhite, yWhite, zWhite float64

func init() {
	xWhite, yWhite, zWhite = ConvertAdobeRGBToXYZ(1, 1, 1)
}

func ConvertXYZToLAB(x, y, z float64) (L, A, B float64) {
	const epsilon = 216.0 / 24389.0
	const kappa = 24389.0 / 27.0

	var f, r [3]float64
	r[0] = x / xWhite
	r[1] = y / yWhite
	r[2] = z / zWhite
	for i := 0; i < 3; i++ {
		if r[i] > epsilon {
			f[i] = math.Pow(r[i], 1.0/3.0)
		} else {
			f[i] = (kappa*r[i] + 16.0) / 116.0
		}
	}
	L = 116.0*f[1] - 16.0
	A = 500.0 * (f[0] - f[1])
	B = 200.0 * (f[1] - f[2])
	return
}
