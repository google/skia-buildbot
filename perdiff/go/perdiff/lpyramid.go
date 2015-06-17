package main

const MAX_PYR_LEVELS = 8

type LPyramid struct {
	levels []*FloatGrayImage
}

func CreateLPyramid(img *FloatGrayImage) (pyramid *LPyramid) {

	pyramid = new(LPyramid)

	pyramid.levels = make([]*FloatGrayImage, MAX_PYR_LEVELS)

	pyramid.levels[0] = MakeFloatGrayImage(img.width, img.height)
	copy(pyramid.levels[0].Pix, img.Pix)

	for i := 1; i < MAX_PYR_LEVELS; i++ {
		pyramid.Convolve(i)
	}

	return
}

func (pyramid *LPyramid) Convolve(level int) {

	kernel := [...]float64{0.05, 0.25, 0.4, 0.25, 0.05}

	width := pyramid.levels[0].width
	height := pyramid.levels[0].height

	pyramid.levels[level] = MakeFloatGrayImage(width, height)

	parallel(height, func(partStart, partEnd int) {
		for y := partStart; y < partEnd; y++ {
			for x := 0; x < width; x++ {
				target_value := 0.0

				for i := -2; i <= 2; i++ {
					for j := -2; j <= 2; j++ {
						nx := x + i
						ny := y + j

						if nx < 0 {
							nx = -nx
						}
						if ny < 0 {
							ny = -ny
						}
						if nx >= width {
							nx = 2*width - nx - 1
						}
						if ny >= height {
							ny = 2*height - ny - 1
						}

						src := pyramid.Get(nx, ny, level-1)
						target_value += kernel[i+2] * kernel[j+2] * src
					}
				}

				pyramid.Set(x, y, level, target_value)
			}
		}
	})
}

func (pyramid *LPyramid) Get(x, y, level int) float64 {
	index := x + y*pyramid.levels[level].width
	l := level
	if l > MAX_PYR_LEVELS {
		l = MAX_PYR_LEVELS
	}
	return pyramid.levels[l].Pix[index]
}

func (pyramid *LPyramid) Set(x, y, level int, value float64) {
	index := x + y*pyramid.levels[level].width
	l := level
	if l > MAX_PYR_LEVELS {
		l = MAX_PYR_LEVELS
	}
	pyramid.levels[l].Pix[index] = value
}
