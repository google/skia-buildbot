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
		AlgorithmOptionalKey: "FakeAlgorithm",
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
		AlgorithmOptionalKey: string(ExactMatching),
	})

	assert.NoError(t, err)
	assert.Equal(t, ExactMatching, algorithmName)
	assert.Nil(t, matcher)
}

// missing is a sentinel value used to represent missing parameter values.
const missing = "missing value"

// fuzzyMatchingTestCase represents a test case for MatcherFactoryImpl#Make() where a
// fuzzy.FuzzyMatcher is instantiated.
type fuzzyMatchingTestCase struct {
	name                string
	maxDifferentPixels  string
	pixelDeltaThreshold string
	want                fuzzy.FuzzyMatcher
	error               string
}

// commonMaxDifferentPixelsTestCases returns test cases for the FuzzyMatchingMaxDifferentPixels
// optional key.
//
// These tests are shared between TestMatcherFactoryImpl_Make_FuzzyMatching and
// TestMatcherFactoryImpl_Make_SobelFuzzyMatching.
func commonMaxDifferentPixelsTestCases() []fuzzyMatchingTestCase {
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
			error:               "invalid syntax",
		},
		{
			name:                "max different pixels: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:  fmt.Sprintf("%d", math.MinInt32-1),
			pixelDeltaThreshold: "0",
			error:               "out of range",
		},
		{
			name:                "max different pixels: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:  fmt.Sprintf("%d", math.MaxInt32+1),
			pixelDeltaThreshold: "0",
			error:               "out of range",
		},
		{
			name:                "max different pixels: value = -1, returns error",
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
			name:                "max different pixels: value = math.MaxInt32, success",
			maxDifferentPixels:  fmt.Sprintf("%d", math.MaxInt32),
			pixelDeltaThreshold: "0",
			want: fuzzy.FuzzyMatcher{
				MaxDifferentPixels:  math.MaxInt32,
				PixelDeltaThreshold: 0,
			},
		},
	}
}

// commonMaxDifferentPixelsTestCases returns test cases for the FuzzyMatchingPixelDeltaThreshold
// optional key.
//
// These tests are shared between TestMatcherFactoryImpl_Make_FuzzyMatching and
// TestMatcherFactoryImpl_Make_SobelFuzzyMatching.
func commonPixelDeltaThresholdTestCases() []fuzzyMatchingTestCase {
	return []fuzzyMatchingTestCase{
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
			error:               "invalid syntax",
		},
		{
			name:                "pixel delta threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: fmt.Sprintf("%d", math.MinInt32-1),
			error:               "out of range",
		},
		{
			name:                "pixel delta threshold: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: fmt.Sprintf("%d", math.MaxInt32+1),
			error:               "out of range",
		},
		{
			name:                "pixel delta threshold: value = -1, returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "-1",
			error:               `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1020, was: -1`,
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
			name:                "pixel delta threshold: value = 1020, success",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "1020",
			want: fuzzy.FuzzyMatcher{
				MaxDifferentPixels:  0,
				PixelDeltaThreshold: 1020,
			},
		},
		{
			name:                "pixel delta threshold: value = 1021, returns error",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "1021",
			error:               `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1020, was: 1021`,
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
	tests = append(tests, commonMaxDifferentPixelsTestCases()...)
	tests = append(tests, commonPixelDeltaThresholdTestCases()...)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				AlgorithmOptionalKey: string(FuzzyMatching),
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
			name:                "edge threshold: non-integer value, returns error",
			edgeThreshold:       "not an integer",
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			error:               "invalid syntax",
		},
		{
			name:                "edge threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			edgeThreshold:       fmt.Sprintf("%d", math.MinInt32-1),
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			error:               "out of range",
		},
		{
			name:                "edge threshold: non-32-bit integer (math.MaxInt32 + 1), returns error",
			edgeThreshold:       fmt.Sprintf("%d", math.MaxInt32+1),
			maxDifferentPixels:  "0",
			pixelDeltaThreshold: "0",
			error:               "out of range",
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
	appendCommonTestCase := func(tc fuzzyMatchingTestCase) {
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
	for _, tc := range commonMaxDifferentPixelsTestCases() {
		appendCommonTestCase(tc)
	}
	for _, tc := range commonPixelDeltaThresholdTestCases() {
		appendCommonTestCase(tc)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				AlgorithmOptionalKey: string(SobelFuzzyMatching),
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
