package thumb

import (
	"image"
	"testing"
)

func TestAbsPath(t *testing.T) {
	if got, want := ThumbAbsPath("fred/foo.png"), "fred/foo-thumbnail.png"; got != want {
		t.Errorf("Incorrect thumbnail name: Got %v Want %v", got, want)
	}
}

func TestThumbnail(t *testing.T) {

	// Create a small red image.
	r := image.Rect(0, 0, 2, 2)
	src := image.NewNRGBA(r)
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i+0] = 0xff
		src.Pix[i+3] = 0xff
	}
	// Thumbnail it.
	dst := Thumbnail(src)

	if got, want := dst.Bounds().Dx(), DIM; got != want {
		t.Errorf("Failed to resize x correctly: Got %v Want %v", got, want)
	}
	if got, want := dst.Bounds().Dy(), DIM; got != want {
		t.Errorf("Failed to resize y correctly: Got %v Want %v", got, want)
	}
	R, G, _, _ := dst.At(0, 0).RGBA()
	if got, want := R, uint32(0xffff); got != want {
		t.Errorf("Failed to copy color: Got %v Want %v", got, want)
	}
	if got, want := G, uint32(0x0000); got != want {
		t.Errorf("Failed to copy color: Got %v Want %v", got, want)
	}

	// Test a point outside the original image.
	R, G, _, _ = dst.At(5, 5).RGBA()
	if got, want := R, uint32(0x0000); got != want {
		t.Errorf("Failed to copy color: Got %v Want %v", got, want)
	}
	if got, want := G, uint32(0x0000); got != want {
		t.Errorf("Failed to copy color: Got %v Want %v", got, want)
	}
}
