package imgmatching

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
	"go.skia.org/infra/gold-client/go/imgmatching/sobel"
)

func TestMatcherFactoryImpl_Make_UnknownAlgorithm_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	f := MatcherFactoryImpl{}
	_, _, err := f.Make(map[string]string{
		ImageMatchingAlgorithmOptionalKey: "FakeAlgorithm",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `unrecognized image matching algorithm: "FakeAlgorithm"`)
}

func TestMatcherFactoryImpl_Make_NoAlgorithmSpecified_ReturnsExactMatching(t *testing.T) {
	unittest.SmallTest(t)

	f := MatcherFactoryImpl{}
	algorithmName, matcher, err := f.Make(map[string]string{})

	assert.NoError(t, err)
	assert.Equal(t, ExactMatching, algorithmName)
	assert.Nil(t, matcher)
}

func TestMatcherFactoryImpl_Make_ExactMatchingExplicitlySpecified_ReturnsExactMatching(t *testing.T) {
	unittest.SmallTest(t)

	f := MatcherFactoryImpl{}
	algorithmName, matcher, err := f.Make(map[string]string{
		ImageMatchingAlgorithmOptionalKey: string(ExactMatching),
	})

	assert.NoError(t, err)
	assert.Equal(t, ExactMatching, algorithmName)
	assert.Nil(t, matcher)
}

// missing represents a missing parameter value.
const missing = "missing value"

// fuzzyMatchingTestCase represents a test case for the fuzzy.FuzzyMatcher.
type fuzzyMatchingTestCase struct {
	name                string
	maxDifferentPixels  string
	pixelDeltaThreshold string
	want                fuzzy.FuzzyMatcher
	error               string
}

// fuzzyMatchingTestCases returns the test cases used to test MatcherFactoryImpl#Make() when the
// FuzzyMatching algorithm is specified.
//
// These test cases are also used to test MatcherFactoryImpl#Make() when the SobelFuzzyMatching
// algorithm is specified. This is because sobel.SobelFuzzyMatcher embeds fuzzy.FuzzyMatcher, thus
// these test cases apply to both matchers.
func fuzzyMatchingTestCases() []fuzzyMatchingTestCase {
	return []fuzzyMatchingTestCase{
		{
			name:                "max different pixels: missing, returns error",
			maxDifferentPixels:  missing,
			pixelDeltaThreshold: "0",
			error:               `required image matching parameter not found: "fuzzy_max_different_pixels"`,
		},
		{
			name:                "max different pixels: empty, returns error",
			maxDifferentPixels:  "",
			pixelDeltaThreshold: "0",
			error:               `image matching parameter "fuzzy_max_different_pixels" cannot be empty`,
		},
		{
			name:                "max different pixels: non-integer value, returns error",
			maxDifferentPixels:  "not an integer",
			pixelDeltaThreshold: "0",
			error:               `parsing integer value for image matching parameter "fuzzy_max_different_pixels"`,
		},
		{
			name:                "max different pixels: value < 0, returns error",
			maxDifferentPixels:  "-1",
			pixelDeltaThreshold: "0",
			error:               `image matching parameter "fuzzy_max_different_pixels" must be at least 0, was: -1`,
		},
		{
			name:                "max different pixels: value = 0, success",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			want: fuzzy.FuzzyMatcher{
				MaxDifferentPixels:  0,
				PixelDeltaThreshold: 0,
			},
		},
		{
			name:                "max different pixels: value = math.MaxUint32, success",
			maxDifferentPixels:  fmt.Sprintf("%d", math.MaxUint32),
			pixelDeltaThreshold: "0",
			want: fuzzy.FuzzyMatcher{
				MaxDifferentPixels:  math.MaxUint32,
				PixelDeltaThreshold: 0,
			},
		},
		{
			name:                "pixel delta threshold: missing, returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: missing,
			error:               `required image matching parameter not found: "fuzzy_pixel_delta_threshold"`,
		},
		{
			name:                "pixel delta threshold: empty, returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "",
			error:               `image matching parameter "fuzzy_pixel_delta_threshold" cannot be empty`,
		},
		{
			name:                "pixel delta threshold: non-integer value, returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "not an integer",
			error:               `parsing integer value for image matching parameter "fuzzy_pixel_delta_threshold"`,
		},
		{
			name:                "pixel delta threshold: value < 0, returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "-1",
			error:               `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1024, was: -1`,
		},
		{
			name:                "pixel delta threshold: value = 0, success",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			want: fuzzy.FuzzyMatcher{
				MaxDifferentPixels:  0,
				PixelDeltaThreshold: 0,
			},
		},
		{
			name:                "pixel delta threshold: 0 < value < 1024, success",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "1023",
			want: fuzzy.FuzzyMatcher{
				MaxDifferentPixels:  0,
				PixelDeltaThreshold: 1023,
			},
		},
		{
			name:                "pixel delta threshold: value = 1024, success",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "1024",
			want: fuzzy.FuzzyMatcher{
				MaxDifferentPixels:  0,
				PixelDeltaThreshold: 1024,
			},
		},
		{
			name:                "pixel delta threshold: value > 1024, returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "1025",
			error:               `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1024, was: 1025`,
		},
	}
}

func TestMatcherFactoryImpl_Make_FuzzyMatching(t *testing.T) {
	unittest.SmallTest(t)

	tests := []fuzzyMatchingTestCase{
		{
			name:                "all parameters missing, returns error",
			maxDifferentPixels:  missing,
			pixelDeltaThreshold: missing,
			error:               "required image matching parameter not found",
		},
	}
	tests = append(tests, fuzzyMatchingTestCases()...)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				ImageMatchingAlgorithmOptionalKey: string(FuzzyMatching),
			}
			if tc.maxDifferentPixels != missing {
				optionalKeys[string(FuzzyMatchingMaxDifferentPixels)] = tc.maxDifferentPixels
			}
			if tc.pixelDeltaThreshold != missing {
				optionalKeys[string(FuzzyMatchingPixelDeltaThreshold)] = tc.pixelDeltaThreshold
			}

			algorithmName, matcher, err := MatcherFactoryImpl{}.Make(optionalKeys)

			if tc.error != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, FuzzyMatching, algorithmName)
				assert.Equal(t, &tc.want, matcher)
			}
		})
	}
}

func TestMatcherFactoryImpl_Make_SobelFuzzyMatching(t *testing.T) {
	unittest.SmallTest(t)

	type sobelFuzzyMatchingTestCase struct {
		name                string
		edgeThreshold       string
		maxDifferentPixels  string
		pixelDeltaThreshold string
		want                sobel.SobelFuzzyMatcher
		error               string
	}

	tests := []sobelFuzzyMatchingTestCase{
		{
			name:                "all parameters missing, returns error",
			edgeThreshold:       missing,
			maxDifferentPixels:  missing,
			pixelDeltaThreshold: missing,

			error: "required image matching parameter not found",
		},
		{
			name:                "edge threshold: missing, returns error",
			edgeThreshold:       missing,
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			error:               `required image matching parameter not found: "sobel_edge_threshold"`,
		},
		{
			name:                "edge threshold: empty, returns error",
			edgeThreshold:       "",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			error:               `image matching parameter "sobel_edge_threshold" cannot be empty`,
		},
		{
			name:                "edge threshold: value < 0, returns error",
			edgeThreshold:       "-1",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			error:               `image matching parameter "sobel_edge_threshold" must be between 0 and 255, was: -1`,
		},
		{
			name:                "edge threshold: value = 0, success",
			edgeThreshold:       "0",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			want: sobel.SobelFuzzyMatcher{
				FuzzyMatcher: fuzzy.FuzzyMatcher{
					MaxDifferentPixels:  0,
					PixelDeltaThreshold: 0,
				},
				EdgeThreshold: 0,
			},
		},
		{
			name:                "edge threshold: 0 < value < 255, success",
			edgeThreshold:       "254",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			want: sobel.SobelFuzzyMatcher{
				FuzzyMatcher: fuzzy.FuzzyMatcher{
					MaxDifferentPixels:  0,
					PixelDeltaThreshold: 0,
				},
				EdgeThreshold: 254,
			},
		},
		{
			name:                "edge threshold: value = 255, success",
			edgeThreshold:       "255",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			want: sobel.SobelFuzzyMatcher{
				FuzzyMatcher: fuzzy.FuzzyMatcher{
					MaxDifferentPixels:  0,
					PixelDeltaThreshold: 0,
				},
				EdgeThreshold: 255,
			},
		},
		{
			name:                "edge threshold: value > 255, returns error",
			edgeThreshold:       "256",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			error:               `image matching parameter "sobel_edge_threshold" must be between 0 and 255, was: 256`,
		},
	}

	// Append test cases for FuzzyMatching.
	for _, tc := range fuzzyMatchingTestCases() {
		tests = append(tests, sobelFuzzyMatchingTestCase{
			name:                tc.name,
			edgeThreshold:       "0",
			maxDifferentPixels:  tc.maxDifferentPixels,
			pixelDeltaThreshold: tc.pixelDeltaThreshold,
			want: sobel.SobelFuzzyMatcher{
				FuzzyMatcher:  tc.want,
				EdgeThreshold: 0,
			},
			error: tc.error,
		})
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				ImageMatchingAlgorithmOptionalKey: string(SobelFuzzyMatching),
			}
			if tc.edgeThreshold != missing {
				optionalKeys[string(SobelFuzzyMatchingEdgeThreshold)] = tc.edgeThreshold
			}
			if tc.maxDifferentPixels != missing {
				optionalKeys[string(FuzzyMatchingMaxDifferentPixels)] = tc.maxDifferentPixels
			}
			if tc.pixelDeltaThreshold != missing {
				optionalKeys[string(FuzzyMatchingPixelDeltaThreshold)] = tc.pixelDeltaThreshold
			}

			algorithmName, matcher, err := MatcherFactoryImpl{}.Make(optionalKeys)

			if tc.error != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.error)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, SobelFuzzyMatching, algorithmName)
				assert.Equal(t, &tc.want, matcher)
			}
		})
	}
}
