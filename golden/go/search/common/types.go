// Package common has a few types/constants used by search
// and its subpackages. It's primary goal is to break dependency cycles.
package common

import "go.skia.org/infra/golden/go/types"

// RefClosest is effectively an
type RefClosest string

const (
	// PositiveRef identifies the diff to the closest positive digest.
	PositiveRef = RefClosest("pos")

	// NegativeRef identifies the diff to the closest negative digest.
	NegativeRef = RefClosest("neg")
)

type ExpSlice []types.Expectations

func (e ExpSlice) Classification(test types.TestName, digest types.Digest) types.Label {
	for _, exp := range e {
		if label := exp.Classification(test, digest); label != types.UNTRIAGED {
			return label
		}
	}
	return types.UNTRIAGED
}
