package fuzzy

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/image/text"
)

type testCase struct {
	name    string
	matcher FuzzyMatcher
	image1  image.Image
	image2  image.Image
	want    bool
}

func TestFuzzyMatcher_ZeroMaxDifferentPixels_ZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name:    "different size images, returns false",
			matcher: FuzzyMatcher{},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			want: false,
		},

		{
			name:    "identical images, returns true",
			matcher: FuzzyMatcher{},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name:    "one different pixel, returns false",
			matcher: FuzzyMatcher{},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000001 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image1, tc.image2), "image1 vs image2")
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image2, tc.image1), "image2 vs image1")
		})
	}
}

func TestFuzzyMatcher_ZeroMaxDifferentPixels_NonZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name: "different size images, returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "identical images, returns true",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "one pixel at PixelDeltaThreshold (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "one pixel at PixelDeltaThreshold (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image1, tc.image2), "image1 vs image2")
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image2, tc.image1), "image2 vs image1")
		})
	}
}

func TestFuzzyMatcher_NonZeroMaxDifferentPixels_ZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name: "different size images, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels: 2,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "identical images, returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels: 2,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels: 2,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels: 2,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0xFFFFFFFF
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels: 2,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0xFFFFFFFF
			0xFFFFFFFF 0x00000000`),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image1, tc.image2), "image1 vs image2")
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image2, tc.image1), "image2 vs image1")
		})
	}
}

func TestFuzzyMatcher_NonZeroMaxDifferentPixels_NonZeroPixelDeltaThreshold(t *testing.T) {
	unittest.SmallTest(t)

	tests := []testCase{
		{
			name: "different size images, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			3 2
			0x00000000 0x00000000 0x00000000
			0x00000000 0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "identical images, returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		/////////////////////////////////////////////////////////
		// number of different pixels = MaxDifferentPixels - 1 //
		/////////////////////////////////////////////////////////

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},


		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold (deltas in some channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold (deltas in all channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels - 1, one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},


		/////////////////////////////////////////////////////
		// number of different pixels = MaxDifferentPixels //
		/////////////////////////////////////////////////////

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000001
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000001
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold (deltas in some channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000001
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold (deltas in all channels), returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000001
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000001
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000001
			0x00000000 0x00000000`),
			want: false,
		},

		/////////////////////////////////////////////////////////
		// number of different pixels = MaxDifferentPixels + 1 //
		/////////////////////////////////////////////////////////

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold - 1 (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08070000 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold - 1 (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040403 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08080000 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040404 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold + 1 (deltas in some channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x08090000 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels = MaxDifferentPixels + 1, one pixel at PixelDeltaThreshold + 1 (deltas in all channels), returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 16,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x04040405 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image1, tc.image2), "image1 vs image2")
			assert.Equal(t, tc.want, tc.matcher.Match(tc.image2, tc.image1), "image2 vs image1")
		})
	}
}
