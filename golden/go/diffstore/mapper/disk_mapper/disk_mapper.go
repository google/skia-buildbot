package disk_mapper

import (
	"fmt"
	"image"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/mapper"
	"go.skia.org/infra/golden/go/types"
)

const (
	imgExtension = "png"
)

// DiskMapper implements the Mapper interface.
// It uses diff.DiffMetrics as the Gold diff metric.
// It stores the images on disk using a two
// level radix prefix (i.e. for digest "abcdefg.png", the
// image will be in ab/cd/abcdefg.png). The use of the radix
// allows us to work around limits in Linux of how many files
// can be in a folder.
type DiskMapper struct {
	util.LRUCodec
}

// New returns a new instance of DiskMapper that uses
// a JSON coded to serialize/deserialize instances of diff.DiffMetrics.
func New(diffInstance interface{}) *DiskMapper {
	return &DiskMapper{LRUCodec: util.JSONCodec(diffInstance)}
}

// DiffFn implements the DiffStoreMapper interface.
func (g *DiskMapper) DiffFn(left *image.NRGBA, right *image.NRGBA) interface{} {
	return diff.DefaultDiffFn(left, right)
}

// ImagePaths implements the DiffStoreMapper interface.
func (g *DiskMapper) ImagePaths(imageID types.Digest) (string, string) {
	gsPath := fmt.Sprintf("%s.%s", imageID, imgExtension)
	localPath := fileutil.TwoLevelRadixPath(gsPath)
	return localPath, gsPath
}

// Make sure DiskMapper fulfills the Mapper interface
var _ mapper.Mapper = (*DiskMapper)(nil)
