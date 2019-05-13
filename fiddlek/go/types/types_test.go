package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestOptions(t *testing.T) {
	unittest.SmallTest(t)
	code := `void draw(SkCanvas* canvas) {
    SkPaint p;
    p.setColor(SK_ColorRED);
    p.setAntiAlias(true);
    p.setStyle(SkPaint::kStroke_Style);
    p.setStrokeWidth(10);

    canvas->drawLine(20, 20, 100, 100, p);
}`
	o := Options{
		Width:  256,
		Height: 256,
		Source: 0,
	}
	hash, err := o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "cbb8dee39e9f1576cd97c2d504db8eee", hash)

	code = `void draw(SkCanvas* canvas) {
    SkPaint p;
    p.setColor(SK_ColorRED);
    p.setAntiAlias(true);
    p.setStyle(SkPaint::kStroke_Style);
    p.setStrokeWidth(10);

    canvas->drawLine(20, 20, 100, 100, p);
}`
	o = Options{
		Width:  256,
		Height: 256,
		Source: 0,
		SRGB:   true,
		F16:    true,
	}
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "fddc0a319e575c79a97ff535b455dc5d", hash)

	o.Animated = true
	o.Duration = 1.5
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "92c7b15afd12fd711ce65ba412574e3d", hash)

	o.OffScreen = true
	o.OffScreenWidth = 256
	o.OffScreenHeight = 256
	o.OffScreenSampleCount = 1
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "a76b7ac7fa98c75e09e32e704adeedc3", hash)

	o.OffScreenTexturable = true
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "03cd8e8e9858add8008e262f63925d7c", hash)

	o.OffScreenMipMap = true
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "052b394aa3078ac12f9a8ee7dde8ee65", hash)

	o.SourceMipMap = true
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "ccc4f49c7d91f444ba4d9cbc431e2822", hash)
}
