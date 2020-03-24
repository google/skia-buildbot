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
				PixelDeltaThreshold: 512,
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
				PixelDeltaThreshold: 512,
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
			name: "one different pixel with dR + dG + dB + dA < PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 512,
			},
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

		{
			name: "one different pixel with dR + dG + dB + dA = PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x0000FFFF 0x00000000
			0x00000000 0x00000000`),
			want: false,
		},

		{
			name: "one different pixel with dR + dG + dB + dA > PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				PixelDeltaThreshold: 512,
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
			name: "0 < number of different pixels < MaxDifferentPixels, returns false",
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
			name: "number of different pixels > MaxDifferentPixels, returns false",
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
				PixelDeltaThreshold: 512,
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
				PixelDeltaThreshold: 512,
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
		// 0 < number of different pixels < MaxDifferentPixels //
		/////////////////////////////////////////////////////////

		{
			name: "0 < number of different pixels < MaxDifferentPixels, all pixels with dR + dG + dB + dA < PixelDeltaThreshold, returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000001 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "0 < number of different pixels < MaxDifferentPixels, one pixel with dR + dG + dB + dA = PixelDeltaThreshold, returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x0000FFFF 0x00000000
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "0 < number of different pixels < MaxDifferentPixels, one pixel with dR + dG + dB + dA > PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
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

		/////////////////////////////////////////////////////
		// number of different pixels = MaxDifferentPixels //
		/////////////////////////////////////////////////////

		{
			name: "number of different pixels = MaxDifferentPixels, all pixels with dR + dG + dB + dA < PixelDeltaThreshold, returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000001 0x00000001
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel with dR + dG + dB + dA = PixelDeltaThreshold, returns true",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x0000FFFF 0x00000001
			0x00000000 0x00000000`),
			want: true,
		},

		{
			name: "number of different pixels = MaxDifferentPixels, one pixel with dR + dG + dB + dA > PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0x00000001
			0x00000000 0x00000000`),
			want: false,
		},

		/////////////////////////////////////////////////////
		// number of different pixels > MaxDifferentPixels //
		/////////////////////////////////////////////////////

		{
			name: "number of different pixels > MaxDifferentPixels, all pixels with dR + dG + dB + dA < PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000001 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels > MaxDifferentPixels, one pixel with dR + dG + dB + dA = PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x0000FFFF 0x00000001
			0x00000001 0x00000000`),
			want: false,
		},

		{
			name: "number of different pixels > MaxDifferentPixels, one pixel with dR + dG + dB + dA > PixelDeltaThreshold, returns false",
			matcher: FuzzyMatcher{
				MaxDifferentPixels:  2,
				PixelDeltaThreshold: 512,
			},
			image1: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0x00000000 0x00000000
			0x00000000 0x00000000`),
			image2: text.MustToNRGBA(`! SKTEXTSIMPLE
			2 2
			0xFFFFFFFF 0x00000001
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
