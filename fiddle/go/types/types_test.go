package types

import (
	"testing"

	"go.skia.org/infra/go/testutils"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *testing.T) {
	testutils.SmallTest(t)
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
}
