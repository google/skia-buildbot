// Package common has a few types/constants used by search
// and its subpackages. Its primary goal is to break dependency cycles.
package common

import (
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

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

// ExpSlice lets us search for expectations in one or more places - this
// is handy for checking the master branch's expectations and expectations
// for a given ChangeList, for example.
type ExpSlice []expectations.Expectations

// Classification returns the first non-untriaged label for the given
// test and digest, starting at the beginning of the ExpSlice and moving
// towards the end. If nothing is found, it says the digest is Untriaged.
func (e ExpSlice) Classification(test types.TestName, digest types.Digest) expectations.Label {
	for _, exp := range e {
		if label := exp.Classification(test, digest); label != expectations.Untriaged {
			return label
		}
	}
	return expectations.Untriaged
}
