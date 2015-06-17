package main

import (
	"fmt"
	"image"
	"log"
	"math"
	"sync/atomic"
)

func Yee_Compare(ImgA, ImgB image.Image) (bool, int) {
	if ImgA.Bounds() != ImgB.Bounds() {
		log.Fatal("I'm sorry, but the two images provided do not have the same dimensions.")
		return false, 0
	}

	bounds := ImgA.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y

	identical := true

outer:
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r1, g1, b1, a1 := ImgA.At(x, y).RGBA()
			r2, g2, b2, a2 := ImgB.At(x, y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				identical = false
				break outer
			}
		}
	}
	if identical {
		log.Println("The images are binary identical.")
		return true, 0
	}

	if options.verbose {
		log.Println("Converting the images to floating point")
	}

	AFloat := CopyImageToFloat(ImgA)
	BFloat := CopyImageToFloat(ImgB)

	if options.debug {
		AFloat.Dump("a_float.png")
		BFloat.Dump("b_float.png")
	}

	if options.verbose {
		log.Println("Gamma correcting...")
	}

	AGamma := AdjustGamma(AFloat, options.gamma)
	BGamma := AdjustGamma(BFloat, options.gamma)

	if options.debug {
		AGamma.Dump("a_gamma.png")
		BGamma.Dump("b_gamma.png")
	}

	if options.verbose {
		log.Println("Converting to LAB")
	}

	ALAB := RGBAToLAB(AFloat)
	BLAB := RGBAToLAB(BFloat)

	if options.debug {
		ALAB.Dump("a_LAB.png")
		BLAB.Dump("b_LAB.png")
	}

	if options.verbose {
		log.Println("Converting to grayscale")
	}

	AGray := RGBAToY(AGamma)
	BGray := RGBAToY(BGamma)

	if options.debug {
		AGray.Dump("a_gray.png")
		AGray.Dump("b_gray.png")
	}

	if options.verbose {
		log.Println("Constructing Laplacian pyramids")
	}

	APyramid := CreateLPyramid(AGray)
	BPyramid := CreateLPyramid(BGray)

	if options.debug {
		for i := 0; i < MAX_PYR_LEVELS; i++ {
			fname := fmt.Sprintf("a_lpyramid_%d.png", i)
			APyramid.levels[i].Dump(fname)
			fname = fmt.Sprintf("b_lpyramid_%d.png", i)
			BPyramid.levels[i].Dump(fname)
		}
	}

	if options.verbose {
		log.Println("Done with Laplacian Pyramid construction")
	}

	num_one_degree_pixels := 2 * math.Tan(options.fov*0.5*math.Pi/180) * 180 / math.Pi
	pixels_per_degree := float64(width) / num_one_degree_pixels

	if options.verbose {
		log.Println("Performing test...")
	}

	num_pixels := 1.0
	adaptation_level := 0

	for i := 0; i < MAX_PYR_LEVELS; i++ {
		adaptation_level = i
		if num_pixels > num_one_degree_pixels {
			break
		}
		num_pixels *= 2
	}

	cpd := make([]float64, MAX_PYR_LEVELS)
	cpd[0] = 0.5 * pixels_per_degree
	for i := 1; i < MAX_PYR_LEVELS; i++ {
		cpd[i] = 0.5 * cpd[i-1]
	}

	csf_max := ContrastSensitivity(3.248, 100.0)

	F_freq := make([]float64, MAX_PYR_LEVELS-2)
	for i := 0; i < MAX_PYR_LEVELS-2; i++ {
		F_freq[i] = csf_max / ContrastSensitivity(cpd[i], 100.0)
	}

	var pixels_failed int32
	pixels_failed = 0

	parallel(height, func(partStart, partEnd int) {
		contrast := make([]float64, MAX_PYR_LEVELS-2)
		F_mask := make([]float64, MAX_PYR_LEVELS-2)
		for y := partStart; y < partEnd; y++ {
			for x := 0; x < width; x++ {
				sum_contrast := 0.0
				for i := 0; i < MAX_PYR_LEVELS-2; i++ {
					n1 := math.Abs(APyramid.Get(x, y, i) - APyramid.Get(x, y, i+1))
					n2 := math.Abs(BPyramid.Get(x, y, i) - BPyramid.Get(x, y, i+1))
					numerator := math.Max(n1, n2)

					d1 := math.Abs(APyramid.Get(x, y, i+2))
					d2 := math.Abs(BPyramid.Get(x, y, i+2))

					denominator := math.Max(d1, d2)

					if denominator < 1e-5 {
						denominator = 1e-5
					}
					contrast[i] = numerator / denominator
					sum_contrast += contrast[i]
				}
				if sum_contrast < 1e-5 {
					sum_contrast = 1e-5
				}

				adapt := APyramid.Get(x, y, adaptation_level) + BPyramid.Get(x, y, adaptation_level)
				adapt *= 0.5

				if adapt < 1e-5 {
					adapt = 1e-5
				}

				for i := 0; i < MAX_PYR_LEVELS-2; i++ {
					F_mask[i] = VisualMasking(contrast[i] * ContrastSensitivity(cpd[i], adapt))
				}

				factor := 0.0
				for i := 0; i < MAX_PYR_LEVELS-2; i++ {
					factor += contrast[i] * F_freq[i] * F_mask[i] / sum_contrast
				}
				if factor < 1 {
					factor = 1
				}

				if factor > 10 {
					factor = 10
				}

				delta := APyramid.Get(x, y, 0) - BPyramid.Get(x, y, 0)

				pass := true

				if delta > factor*VisibilityThreshold(adapt) {
					pass = false
				} else if !options.luminanceOnly {
					// CIE delta E test with some modifications
					color_scale := options.colorFactor
					// ramp down the color test in scotopic regions
					if adapt < 10.0 {
						// Don't do the color test at all
						color_scale = 0.0
					}

					_, da, _, _ := ALAB.Get(x, y)
					_, db, _, _ := BLAB.Get(x, y)

					da = da * da
					db = db * db

					delta_e := (da + db) * color_scale
					if delta_e > factor {
						pass = false
					}
				}
				if !pass {
					atomic.AddInt32(&pixels_failed, 1)

					if options.output != nil {
						options.output.Set(x, y, 1)
					}
				} else if options.output != nil {
					options.output.Set(x, y, 0)
				}
			}
		}
	})

	if options.verbose {
		log.Printf("Done!  Found %d pixels that were perceptually different.", pixels_failed)
	}

	if int(pixels_failed) < options.threshold {
		return true, int(pixels_failed)
	}

	return false, int(pixels_failed)
}
