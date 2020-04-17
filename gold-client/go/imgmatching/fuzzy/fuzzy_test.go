package fuzzy

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/image/text"
)

type testCase struct {
	name                       string
	image1                     image.Image
	image2                     image.Image
	expectedToMatch            bool
	expectedNumDifferentPixels int
	expectedMaxPixelDelta      int
}

func runTestCases(t *testing.T, maxDifferentPixels, pixelDeltaThreshold int, tests []testCase) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := Matcher{
				MaxDifferentPixels:  maxDifferentPixels,
				PixelDeltaThreshold: pixelDeltaThreshold,
			}

			// image1 vs. image2.
			assert.Equal(t, tc.expectedToMatch, matcher.Match(tc.image1, tc.image2), "image1 vs image2: match")
			assert.Equal(t, tc.expectedNumDifferentPixels, matcher.NumDifferentPixels(), "image1 vs image2: number of different pixels")
			assert.Equal(t, tc.expectedMaxPixelDelta, matcher.MaxPixelDelta(), "image1 vs image2: max pixel delta")

			// image2 vs. image1.
			assert.Equal(t, tc.expectedToMatch, matcher.Match(tc.image2, tc.image1), "image2 vs image1")
			assert.Equal(t, tc.expectedNumDifferentPixels, matcher.NumDifferentPixels(), "image2 vs image1: number of different pixels")
			assert.Equal(t, tc.expectedMaxPixelDelta, matcher.MaxPixelDelta(), "image2 vs image1: max pixel delta")
		})
	}
}

func TestMatcher_ZeroMaxDifferentPixels_ZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name: "different size images, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			expectedToMatch: false,
		},

		{
			name: "identical images, returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch: true,
		},

		{
			name: "one different pixel, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000001 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      1,
		},
	}

	runTestCases(t, 0 /* =maxDifferentPixels */, 0 /* =pixelDeltaThreshold */, tests)
}

func TestMatcher_ZeroMaxDifferentPixels_NonZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name: "different size images, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			expectedToMatch: false,
		},

		{
			name: "identical images, returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch: true,
		},

		{
			name: "one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "one pixel at PixelDeltaThreshold (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "one pixel at PixelDeltaThreshold (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      17,
		},

		{
			name: "one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      17,
		},
	}

	runTestCases(t, 0 /* =maxDifferentPixels */, 16 /* =pixelDeltaThreshold */, tests)
}

func TestMatcher_NonZeroMaxDifferentPixels_ZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name: "different size images, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			expectedToMatch: false,
		},

		{
			name: "identical images, returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      1020,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0xFFFFFFFF
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 2,
			expectedMaxPixelDelta:      1020,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0xFFFFFFFF
			0xFFFFFFFF 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 3,
			expectedMaxPixelDelta:      1020,
		},
	}

	runTestCases(t, 2 /* =maxDifferentPixels */, 0 /* =pixelDeltaThreshold */, tests)
}

func TestMatcher_NonZeroMaxDifferentPixels_NonZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name: "different size images, returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			expectedToMatch: false,
		},

		{
			name: "identical images, returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch: true,
		},

		/////////////////////////////////////////////////////////
		// number of different pixels = MaxDifferentPixels - 1 //
		/////////////////////////////////////////////////////////

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold (deltas in some channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold (deltas in all channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      17,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000000
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 1,
			expectedMaxPixelDelta:      17,
		},

		/////////////////////////////////////////////////////
		// number of different pixels = MaxDifferentPixels //
		/////////////////////////////////////////////////////

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000001
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 2,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000001
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 2,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold (deltas in some channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000001
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 2,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold (deltas in all channels), returns true",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000001
			0x00000000 0x00000000`),
			expectedToMatch:            true,
			expectedNumDifferentPixels: 2,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000001
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 2,
			expectedMaxPixelDelta:      17,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000001
			0x00000000 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 2,
			expectedMaxPixelDelta:      17,
		},

		/////////////////////////////////////////////////////////
		// number of different pixels = MaxDifferentPixels + 1 //
		/////////////////////////////////////////////////////////

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000001
			0x00000001 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 3,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000001
			0x00000001 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 3,
			expectedMaxPixelDelta:      15,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000001
			0x00000001 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 3,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000001
			0x00000001 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 3,
			expectedMaxPixelDelta:      16,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000001
			0x00000001 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 3,
			expectedMaxPixelDelta:      17,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000001
			0x00000001 0x00000000`),
			expectedToMatch:            false,
			expectedNumDifferentPixels: 3,
			expectedMaxPixelDelta:      17,
		},
	}

	runTestCases(t, 2 /* =maxDifferentPixels */, 16 /* =pixelDeltaThreshold */, tests)
}
