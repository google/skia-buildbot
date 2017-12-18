package search

import (
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

type ExpSlice []*expstorage.Expectations

func (e ExpSlice) Classification(testName, digest string) types.Label {
	for _, exp := range e {

	}
	return types.UNTRIAGED
}
