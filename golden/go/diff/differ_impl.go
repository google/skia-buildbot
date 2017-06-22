package diff

import "image"

// DifferImpl implements Differ
type DifferImpl struct {
	// Function to compute diff result
	diffFn   DiffFunc
}

// Takes in custom diff function and returns a new instance of differ
func NewDiffer(diffFn func(image.Image, image.Image) (*DiffMetrics, *image.NRGBA)) Differ {
	return &DifferImpl{
		diffFn:   diffFn,
	}
}

// Diff applies custom diff function to two input images
func (d *DifferImpl) Diff(img1, img2 image.Image) (*DiffMetrics, *image.NRGBA) {
	return d.diffFn(img1, img2)
}
