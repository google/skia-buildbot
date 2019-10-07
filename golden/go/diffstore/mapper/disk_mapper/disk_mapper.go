package disk_mapper

import (
	"image"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/mapper"
)

// DiskMapper implements the Mapper interface.
type DiskMapper struct {
	util.LRUCodec
}

// New returns a new instance of DiskMapper that uses
// a JSON coded to serialize/deserialize instances of diff.DiffMetrics.
func New(diffInstance interface{}) *DiskMapper {
	return &DiskMapper{LRUCodec: util.JSONCodec(diffInstance)}
}

// DiffFn implements the DiffStoreMapper interface.
func (g *DiskMapper) DiffFn(left *image.NRGBA, right *image.NRGBA) *diff.DiffMetrics {
	return diff.DefaultDiffFn(left, right)
}

// Make sure DiskMapper fulfills the Mapper interface
var _ mapper.Mapper = (*DiskMapper)(nil)
