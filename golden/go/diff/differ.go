package diff

import "image"

// Takes two images and returns an application dependent diff result structure
// as a generic interface
type Differ interface {
	// Function that computes diff result
	Diff(img1, img2 image.Image) (interface{}, error)
}

// Function signature for custom diff functions
type DiffFunc func(img1, img2 image.Image) (interface{}, error)
