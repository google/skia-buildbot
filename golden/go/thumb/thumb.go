package thumb

import (
	"image"
	"image/draw"
	"path"

	"github.com/nfnt/resize"
)

const DIM = 64

// AbsPath turns an absolute path of an image into an absolute
// path for that images thumbnail. It does not check if the thumbnail
// exists.
func AbsPath(filepath string) string {
	ext := path.Ext(filepath)
	prefix := filepath[:len(filepath)-len(ext)]
	return prefix + "-thumbnail" + ext
}

// Thumbnail will return a 64x64 thumbnail of the given image. The resulting
// image will always be 64x64, and the aspect ratio of the original image will
// be retained. Transparent black will fill in the remaining area.
func Thumbnail(img image.Image) image.Image {
	ret := resize.Thumbnail(DIM, DIM, img, resize.Bilinear)
	if ret.Bounds().Dx() < DIM || ret.Bounds().Dy() < DIM {
		r := image.Rect(0, 0, DIM, DIM)
		blank := image.NewNRGBA(r)
		draw.Draw(blank, r, ret, image.Point{}, draw.Src)
		ret = blank
	}
	return ret
}
