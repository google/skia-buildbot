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

const animatedSample = `// Copyright 2020 Google LLC.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.
#include "tools/fiddle/examples.h"
REG_FIDDLE_ANIMATED(pong, 256, 300, false, 0, 10.5) {
static SkScalar PingPong(double t, SkScalar period, SkScalar phase,
                         SkScalar ends, SkScalar mid) {
  double value = ::fmod(t + phase, period);
  double half = period / 2.0;
  double diff = ::fabs(value - half);
  return SkDoubleToScalar(ends + (1.0 - diff / half) * (mid - ends));
}

void draw(SkCanvas* canvas) {
  canvas->clear(SK_ColorBLACK);
  float ballX = PingPong(frame * duration, 2.5f, 0.0f, 0.0f, 1.0f);
  float ballY = PingPong(frame * duration, 2.0f, 0.4f, 0.0f, 1.0f);

  SkPaint p;
  p.setColor(SK_ColorWHITE);
  p.setAntiAlias(true);

  float bX = ballX * 472 + 20;
  float bY = ballY * 200 + 28;

  if (canvas->recordingContext()) {
    canvas->drawRect(SkRect::MakeXYWH(236, bY - 15, 10, 30), p);
    bX -= 256;
  } else {
    canvas->drawRect(SkRect::MakeXYWH(10, bY - 15, 10, 30), p);
  }
  canvas->drawCircle(bX, bY, 5, p);
}
}  // END FIDDLE`

const srgbSample = `// Copyright 2020 Google LLC.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.
#include "tools/fiddle/examples.h"
REG_FIDDLE_SRGB(50_percent_gray, 530, 150, true, 0, 18.5, true) {
static sk_sp<SkShader> make_bw_dither() {
    auto surf = SkSurface::MakeRasterN32Premul(2, 2);
    surf->getCanvas()->drawColor(SK_ColorWHITE);
    surf->getCanvas()->drawRect({0, 0, 1, 1}, SkPaint());
    surf->getCanvas()->drawRect({1, 1, 2, 2}, SkPaint());
    return surf->makeImageSnapshot()->makeShader(SkTileMode::kRepeat,
                                                 SkTileMode::kRepeat,
                                                 SkSamplingOptions(SkFilterMode::kLinear));
}

void draw(SkCanvas* canvas) {
    canvas->drawColor(SK_ColorWHITE);
    SkFont font(nullptr, 12);

    // BW Dither
    canvas->translate(5, 5);
    SkPaint p;
    p.setShader(make_bw_dither());
    canvas->drawRect({0, 0, 100, 100}, p);
    SkPaint black;
    canvas->drawString("BW Dither", 0, 125, font, black);
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

func TestParse_AnimatedSample_InformationExtracted(t *testing.T) {
	unittest.SmallTest(t)

	fc, err := ParseCpp(animatedSample)
	assert.NoError(t, err)
	assert.Equal(t, "pong", fc.Name)
	assert.Equal(t, 256, fc.Options.Width)
	assert.Equal(t, 300, fc.Options.Height)
	assert.Equal(t, 0, fc.Options.Source)
	assert.False(t, fc.Options.TextOnly)
	assert.Equal(t, 10.5, fc.Options.Duration)
}

func TestParse_SRGBSample_InformationExtracted(t *testing.T) {
	unittest.SmallTest(t)

	fc, err := ParseCpp(srgbSample)
	assert.NoError(t, err)
	assert.Equal(t, "50_percent_gray", fc.Name)
	assert.Equal(t, 530, fc.Options.Width)
	assert.Equal(t, 150, fc.Options.Height)
	assert.Equal(t, 0, fc.Options.Source)
	assert.True(t, fc.Options.TextOnly)
	assert.Equal(t, 18.5, fc.Options.Duration)
	assert.True(t, fc.Options.F16)
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
	assert.Contains(t, err.Error(), "failed to find any REG_FIDDLE*")
}

func TestParse_MacroBadWidthParam_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	_, err := ParseCpp(badMacro)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing width")
}
