package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

const goodSample = `// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.
#include "fiddle/examples.h"
// HASH=bc9c7ea424d10bbcd1e5a88770d4794e
REG_FIDDLE(Alpha_Constants_a, 400, 600, true, 2) {
void draw(SkCanvas* canvas) {
    std::vector<int32_t> srcPixels;
    srcPixels.resize(source.height() * source.rowBytes());
    SkPixmap pixmap(SkImageInfo::MakeN32Premul(source.width(), source.height()),
                    &srcPixels.front(), source.rowBytes());
    source.readPixels(pixmap, 0, 0);
    for (int y = 0; y < 16; ++y) {
        for (int x = 0; x < 16; ++x) {
            int32_t* blockStart = &srcPixels.front() + y * source.width() * 16 + x * 16;
            size_t transparentCount = 0;
            for (int fillY = 0; fillY < source.height() / 16; ++fillY) {
                for (int fillX = 0; fillX < source.width() / 16; ++fillX) {
                    const SkColor color = SkUnPreMultiply::PMColorToColor(blockStart[fillX]);
                    transparentCount += SkColorGetA(color) == SK_AlphaTRANSPARENT;
                }
                blockStart += source.width();
            }
            if (transparentCount > 200) {
                blockStart = &srcPixels.front() + y * source.width() * 16 + x * 16;
                for (int fillY = 0; fillY < source.height() / 16; ++fillY) {
                    for (int fillX = 0; fillX < source.width() / 16; ++fillX) {
                        blockStart[fillX] = SK_ColorRED;
                    }
                    blockStart += source.width();
                }
            }
        }
    }
    SkBitmap bitmap;
    bitmap.installPixels(pixmap);
    canvas->drawBitmap(bitmap, 0, 0);
}
}  // END FIDDLE`

const shortSample = `// Copyright 2019 Google LLC.
REG_FIDDLE(EmptyFiddle, 256, 128, false, 1) {
void draw(SkCanvas* canvas) {
}
}  // END FIDDLE`

const missingEndSample = `// Copyright 2019 Google LLC.
REG_FIDDLE(Alpha_Constants_a, 256, 128, false, 1) {
void draw(SkCanvas* canvas) {
}
}`

const inactiveSample = `#if 0  // Disabled until updated to use current API.
// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.
#include "fiddle/examples.h"
// HASH=f0e584aec20eaee7a5bfed62aa885eee
REG_FIDDLE(TextBlobBuilder_allocRun, 256, 60, false, 0) {
void draw(SkCanvas* canvas) {
    SkTextBlobBuilder builder;
    SkFont font;
    SkPaint paint;
    const SkTextBlobBuilder::RunBuffer& run = builder.allocRun(font, 5, 20, 20);
    paint.textToGlyphs("hello", 5, run.glyphs);
    canvas->drawRect({20, 20, 30, 30}, paint);
    canvas->drawTextBlob(builder.make(), 20, 20, paint);
}
}  // END FIDDLE
#endif  // Disabled until updated to use current API.`

const missingRegSample = `
// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.
void draw(SkCanvas* canvas) {
    SkTextBlobBuilder builder;
    SkFont font;
    SkPaint paint;
    const SkTextBlobBuilder::RunBuffer& run = builder.allocRun(font, 5, 20, 20);
    paint.textToGlyphs("hello", 5, run.glyphs);
    canvas->drawRect({20, 20, 30, 30}, paint);
    canvas->drawTextBlob(builder.make(), 20, 20, paint);
}
}  // END FIDDLE`

const badMacro = `REG_FIDDLE(TextBlobBuilder_allocRun, foo, 60, false, 0) {`

const textonlySample = `// Copyright 2019 Google LLC.
REG_FIDDLE(50_percent_gray, 256, 128, true, 0) {
void draw(SkCanvas* canvas) {
}
}  // END FIDDLE`

func TestParse_ValidSamples_InformationExtracted(t *testing.T) {
	unittest.SmallTest(t)

	fc, err := ParseCpp(goodSample)
	assert.NoError(t, err)
	assert.Equal(t, "Alpha_Constants_a", fc.Name)
	assert.Equal(t, 400, fc.Options.Width)
	assert.Equal(t, 600, fc.Options.Height)
	assert.Equal(t, 2, fc.Options.Source)
	assert.True(t, fc.Options.TextOnly)

	fc, err = ParseCpp(shortSample)
	assert.NoError(t, err)
	assert.Equal(t, "EmptyFiddle", fc.Name)
	assert.Equal(t, "void draw(SkCanvas* canvas) {\n}", fc.Code)
	assert.Equal(t, 256, fc.Options.Width)
	assert.Equal(t, 128, fc.Options.Height)
	assert.Equal(t, 1, fc.Options.Source)
	assert.False(t, fc.Options.TextOnly)

	fc, err = ParseCpp(textonlySample)
	assert.NoError(t, err)
	assert.Equal(t, "50_percent_gray", fc.Name)
	assert.Equal(t, "void draw(SkCanvas* canvas) {\n}", fc.Code)
	assert.Equal(t, 256, fc.Options.Width)
	assert.Equal(t, 128, fc.Options.Height)
	assert.Equal(t, 0, fc.Options.Source)
	assert.True(t, fc.Options.TextOnly)
}

func TestParse_MissingEndFiddle_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := ParseCpp(missingEndSample)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "END FIDDLE")
}

func TestParse_FiddleIfDeffedOut_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := ParseCpp(inactiveSample)
	assert.Equal(t, err, ErrorInactiveExample)
}

func TestParse_MissingRegFiddle_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := ParseCpp(missingRegSample)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to find REG_FIDDLE")
}

func TestParse_MacroBadWidthParam_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := ParseCpp(badMacro)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing width")
}
