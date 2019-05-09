package common

import (
	"path/filepath"
	"strconv"

	"go.skia.org/infra/golden/go/types"
)

// ct_pixel_diff doesn't use image digests like Gold does to identify
// images uniquely.  Instead, it uses a 4 part string like:
// backer-20180907133025/withpatch/18/http___www_instagram_com
// This type alias makes the DiffStoreMapper below compatible with
// Gold, while still documenting that there is a difference.
type ImageID = types.Digest

// Returns the diffstore.PixelDiffIDPathMapper image ID for a screenshot, which
// has the format runID/{nopatch/withpatch}/rank/URLfilename.
func GetImageID(runID, patchType, filename string, rank int) ImageID {
	rankStr := strconv.Itoa(rank)
	return ImageID(filepath.Join(runID, patchType, rankStr, filename))
}
