package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
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
	assert.Equal(t, "163b9c435a7fbf1367ed9ee71839a4cc", hash)

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
	assert.Equal(t, "f54b0278c8d075817fa590056437bab3", hash)

	o.Animated = true
	o.Duration = 1.5
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "baa851459d5f03fc7422c1cce7bb8a74", hash)

	o.OffScreen = true
	o.OffScreenWidth = 256
	o.OffScreenHeight = 256
	o.OffScreenSampleCount = 1
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "cb94299a913c6ac9053e46b685971aca", hash)

	o.OffScreenTexturable = true
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "ee4bc8e0f854533af31e19cc2650f14f", hash)

	o.OffScreenMipMap = true
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "78a8c62fbd8170ae082c0efb6c0e3bde", hash)

	o.SourceMipMap = true
	hash, err = o.ComputeHash(code)
	assert.NoError(t, err)
	assert.Equal(t, "a325ad1b372655bb1cb1fc195310e878", hash)
}
