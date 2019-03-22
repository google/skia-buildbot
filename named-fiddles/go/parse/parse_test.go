package parse

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
)

const active_sample = `// Copyright 2019 Google LLC.
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.
#include "fiddle/examples.h"
// HASH=bc9c7ea424d10bbcd1e5a88770d4794e
REG_FIDDLE(Alpha_Constants_a, 256, 128, false, 1) {
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

const inactive_sample = `#if 0  // Disabled until updated to use current API.
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

const bad_sample = `
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

func TestParse(t *testing.T) {
	testutils.SmallTest(t)

	fc, err := parseCpp(active_sample)
	assert.NoError(t, err)
	fc, err = parseCpp(inactive_sample)
	assert.Equal(t, err, ErrorInactiveExample)
	assert.Nil(t, fc)
	fc, err = parseCpp(bad_sample)
	assert.Error(t, err)
}
