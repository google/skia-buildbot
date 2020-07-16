// Package common has a few types/constants used by search
// and its subpackages. Its primary goal is to break dependency cycles.
package common

// RefClosest is effectively an enum of two values - positive/negative.
type RefClosest string

const (
	// PositiveRef identifies the diff to the closest positive digest.
	PositiveRef = RefClosest("pos")

	// NegativeRef identifies the diff to the closest negative digest.
	NegativeRef = RefClosest("neg")

	// NoRef indicates no other digests match.
	NoRef = RefClosest("")
)

// AllRefClosest is a list of all possible RefClosest values.
var AllRefClosest = []RefClosest{PositiveRef, NegativeRef, NoRef}
