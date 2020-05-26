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

func runTestCases(t *testing.T, tests []testCase, makeMatcher func() Matcher) {
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			matcher := makeMatcher()

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

	runTestCases(t, tests, func() Matcher {
		return Matcher{
			MaxDifferentPixels:  0,
			PixelDeltaThreshold: 0,
		}
	})
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
			expectedMaxPixelDelta:      0x08 + 0x07,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x03,
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
			expectedMaxPixelDelta:      0x08 + 0x08,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x04,
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
			expectedMaxPixelDelta:      0x08 + 0x09,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x05,
		},
	}

	runTestCases(t, tests, func() Matcher {
		return Matcher{
			MaxDifferentPixels:  0,
			PixelDeltaThreshold: 16,
		}
	})
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
			expectedMaxPixelDelta:      0xFF + 0xFF + 0xFF + 0xFF,
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
			expectedMaxPixelDelta:      0xFF + 0xFF + 0xFF + 0xFF,
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
			expectedMaxPixelDelta:      0xFF + 0xFF + 0xFF + 0xFF,
		},
	}

	runTestCases(t, tests, func() Matcher {
		return Matcher{
			MaxDifferentPixels:  2,
			PixelDeltaThreshold: 0,
		}
	})
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
			expectedMaxPixelDelta:      0x08 + 0x07,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x03,
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
			expectedMaxPixelDelta:      0x08 + 0x08,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x04,
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
			expectedMaxPixelDelta:      0x08 + 0x09,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x05,
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
			expectedMaxPixelDelta:      0x08 + 0x07,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x03,
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
			expectedMaxPixelDelta:      0x08 + 0x08,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x04,
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
			expectedMaxPixelDelta:      0x08 + 0x09,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x05,
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
			expectedMaxPixelDelta:      0x08 + 0x07,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x03,
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
			expectedMaxPixelDelta:      0x08 + 0x08,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x04,
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
			expectedMaxPixelDelta:      0x08 + 0x09,
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
			expectedMaxPixelDelta:      0x04 + 0x04 + 0x04 + 0x05,
		},
	}

	runTestCases(t, tests, func() Matcher {
		return Matcher{
			MaxDifferentPixels:  2,
			PixelDeltaThreshold: 16,
		}
	})
}

func TestMatcher_NonZeroIgnoredBorderThickness_Success(t *testing.T) {
	unittest.SmallTest(t)

	imageWithNoBorder := text.MustToNRGBA(`! SKTEXTSIMPLE
	10 10
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00
	0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00`)

	imageWith1pxBorder := text.MustToNRGBA(`! SKTEXTSIMPLE
	10 10
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0x00 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`)

	imageWith2pxBorder := text.MustToNRGBA(`! SKTEXTSIMPLE
	10 10
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0xFF 0xFF
	0xFF 0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0xFF 0xFF
	0xFF 0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0xFF 0xFF
	0xFF 0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0xFF 0xFF
	0xFF 0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0xFF 0xFF
	0xFF 0xFF 0x00 0x00 0x00 0x00 0x00 0x00 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`)

	imageWith3pxBorder := text.MustToNRGBA(`! SKTEXTSIMPLE
	10 10
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0x00 0x00 0x00 0x00 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0x00 0x00 0x00 0x00 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0x00 0x00 0x00 0x00 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0x00 0x00 0x00 0x00 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`)

	imageWith4pxBorder := text.MustToNRGBA(`! SKTEXTSIMPLE
	10 10
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0x00 0x00 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0x00 0x00 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF
	0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF 0xFF`)

	tests := []testCase{
		{
			name:                       "IgnoredBorderThickness = 2, image with 4px border, returns false",
			image1:                     imageWith4pxBorder,
			image2:                     imageWithNoBorder,
			expectedToMatch:            false,
			expectedNumDifferentPixels: 32,
			expectedMaxPixelDelta:      765, // A SKTEXTSIMPLE value of 0x00 is equivalent to 0x000000FF.
		},
		{
			name:                       "IgnoredBorderThickness = 2, image with 3px border, returns false",
			image1:                     imageWith3pxBorder,
			image2:                     imageWithNoBorder,
			expectedToMatch:            false,
			expectedNumDifferentPixels: 20,
			expectedMaxPixelDelta:      765, // A SKTEXTSIMPLE value of 0x00 is equivalent to 0x000000FF.
		},
		{
			name:            "IgnoredBorderThickness = 2, image with 2px border, returns true",
			image1:          imageWith2pxBorder,
			image2:          imageWithNoBorder,
			expectedToMatch: true,
		},
		{
			name:            "IgnoredBorderThickness = 2, image with 1px border, returns true",
			image1:          imageWith1pxBorder,
			image2:          imageWithNoBorder,
			expectedToMatch: true,
		},
	}

	runTestCases(t, tests, func() Matcher {
		return Matcher{
			MaxDifferentPixels:     0,
			PixelDeltaThreshold:    0,
			IgnoredBorderThickness: 2,
		}
	})
}
