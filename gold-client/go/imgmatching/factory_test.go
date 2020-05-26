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

func TestMakeMatcher_UnknownAlgorithm_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	_, _, err := MakeMatcher(map[string]string{
		AlgorithmNameOptKey: "FakeAlgorithm",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `unrecognized image matching algorithm: "FakeAlgorithm"`)
}

func TestMakeMatcher_NoAlgorithmSpecified_ReturnsExactMatching(t *testing.T) {
	unittest.SmallTest(t)

	algorithmName, matcher, err := MakeMatcher(map[string]string{})

	assert.NoError(t, err)
	assert.Equal(t, ExactMatching, algorithmName)
	assert.Nil(t, matcher)
}

func TestMakeMatcher_ExactMatchingExplicitlySpecified_ReturnsExactMatching(t *testing.T) {
	unittest.SmallTest(t)

	algorithmName, matcher, err := MakeMatcher(map[string]string{
		AlgorithmNameOptKey: string(ExactMatching),
	})

	assert.NoError(t, err)
	assert.Equal(t, ExactMatching, algorithmName)
	assert.Nil(t, matcher)
}

// missing is a sentinel value used to represent missing parameter values.
const missing = "missing value"

// fuzzyMatchingTestCase represents a test case for MakeMatcher() where a fuzzy.Matcher is
// instantiated.
type fuzzyMatchingTestCase struct {
	name                   string
	maxDifferentPixels     string
	pixelDeltaThreshold    string
	ignoredBorderThickness string
	want                   fuzzy.Matcher
	error                  string
}

// commonMaxDifferentPixelsTestCases returns test cases for the MaxDifferentPixels
// optional key.
//
// These tests are shared between TestMakeMatcher_FuzzyMatching and
// TestMakeMatcher_SobelFuzzyMatching.
func commonMaxDifferentPixelsTestCases() []fuzzyMatchingTestCase {
	return []fuzzyMatchingTestCase{
		{
			name:                   "max different pixels: missing, returns error",
			maxDifferentPixels:     missing,
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			error:                  `required image matching parameter not found: "fuzzy_max_different_pixels"`,
		},
		{
			name:                   "max different pixels: empty, returns error",
			maxDifferentPixels:     "",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			error:                  `image matching parameter "fuzzy_max_different_pixels" cannot be empty`,
		},
		{
			name:                   "max different pixels: non-integer value, returns error",
			maxDifferentPixels:     "not an integer",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			error:                  "invalid syntax",
		},
		{
			name:                   "max different pixels: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:     fmt.Sprintf("%d", math.MinInt32-1),
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			error:                  "out of range",
		},
		{
			name:                   "max different pixels: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:     fmt.Sprintf("%d", math.MaxInt32+1),
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			error:                  "out of range",
		},
		{
			name:                   "max different pixels: value = -1, returns error",
			maxDifferentPixels:     "-1",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			error:                  `image matching parameter "fuzzy_max_different_pixels" must be at least 0, was: -1`,
		},
		{
			name:                   "max different pixels: value = 0, success",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:     0,
				PixelDeltaThreshold:    0,
				IgnoredBorderThickness: 0,
			},
		},
		{
			name:                   "max different pixels: value = math.MaxInt32, success",
			maxDifferentPixels:     fmt.Sprintf("%d", math.MaxInt32),
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:     math.MaxInt32,
				PixelDeltaThreshold:    0,
				IgnoredBorderThickness: 0,
			},
		},
	}
}

// commonMaxDifferentPixelsTestCases returns test cases for the PixelDeltaThreshold
// optional key.
//
// These tests are shared between TestMakeMatcher_FuzzyMatching and
// TestMakeMatcher_SobelFuzzyMatching.
func commonPixelDeltaThresholdTestCases() []fuzzyMatchingTestCase {
	return []fuzzyMatchingTestCase{
		{
			name:                   "pixel delta threshold: missing, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    missing,
			ignoredBorderThickness: missing,
			error:                  `required image matching parameter not found: "fuzzy_pixel_delta_threshold"`,
		},
		{
			name:                   "pixel delta threshold: empty, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "",
			ignoredBorderThickness: missing,
			error:                  `image matching parameter "fuzzy_pixel_delta_threshold" cannot be empty`,
		},
		{
			name:                   "pixel delta threshold: non-integer value, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "not an integer",
			ignoredBorderThickness: missing,
			error:                  "invalid syntax",
		},
		{
			name:                   "pixel delta threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    fmt.Sprintf("%d", math.MinInt32-1),
			ignoredBorderThickness: missing,
			error:                  "out of range",
		},
		{
			name:                   "pixel delta threshold: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    fmt.Sprintf("%d", math.MaxInt32+1),
			ignoredBorderThickness: missing,
			error:                  "out of range",
		},
		{
			name:                   "pixel delta threshold: value = -1, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "-1",
			ignoredBorderThickness: missing,
			error:                  `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1020, was: -1`,
		},
		{
			name:                   "pixel delta threshold: value = 0, success",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:     0,
				PixelDeltaThreshold:    0,
				IgnoredBorderThickness: 0,
			},
		},
		{
			name:                   "pixel delta threshold: value = 1020, success",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "1020",
			ignoredBorderThickness: missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:     0,
				PixelDeltaThreshold:    1020,
				IgnoredBorderThickness: 0,
			},
		},
		{
			name:                   "pixel delta threshold: value = 1021, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "1021",
			ignoredBorderThickness: missing,
			error:                  `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1020, was: 1021`,
		},
	}
}

// commonIgnoredBorderThicknessTestCases returns test cases for the IgnoredBorderThickness
// optional key.
//
// These tests are shared between TestMakeMatcher_FuzzyMatching and
// TestMakeMatcher_SobelFuzzyMatching.
func commonIgnoredBorderThicknessTestCases() []fuzzyMatchingTestCase {
	return []fuzzyMatchingTestCase{
		{
			name:                   "ignored border thickness: missing, success",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:     0,
				PixelDeltaThreshold:    0,
				IgnoredBorderThickness: 0,
			},
		},
		{
			name:                   "ignored border thickness: empty, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "",
			error:                  `image matching parameter "fuzzy_ignored_border_thickness" cannot be empty`,
		},
		{
			name:                   "ignored border thickness: non-integer value, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "not an integer",
			error:                  "invalid syntax",
		},
		{
			name:                   "ignored border thickness: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: fmt.Sprintf("%d", math.MinInt32-1),
			error:                  "out of range",
		},
		{
			name:                   "ignored border thickness: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: fmt.Sprintf("%d", math.MaxInt32+1),
			error:                  "out of range",
		},
		{
			name:                   "ignored border thickness: value = -1, returns error",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "-1",
			error:                  `image matching parameter "fuzzy_ignored_border_thickness" must be at least 0, was: -1`,
		},
		{
			name:                   "ignored border thickness: value = 0, success",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			want: fuzzy.Matcher{
				MaxDifferentPixels:     0,
				PixelDeltaThreshold:    0,
				IgnoredBorderThickness: 0,
			},
		},
		{
			name:                   "ignored border thickness: value = 1, success",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "1",
			want: fuzzy.Matcher{
				MaxDifferentPixels:     0,
				PixelDeltaThreshold:    0,
				IgnoredBorderThickness: 1,
			},
		},
	}
}

func TestMakeMatcher_FuzzyMatching(t *testing.T) {
	unittest.SmallTest(t)

	tests := []fuzzyMatchingTestCase{
		{
			name:                   "all parameters missing, returns error",
			maxDifferentPixels:     missing,
			pixelDeltaThreshold:    missing,
			ignoredBorderThickness: missing,
			error:                  "required image matching parameter not found",
		},
	}
	tests = append(tests, commonMaxDifferentPixelsTestCases()...)
	tests = append(tests, commonPixelDeltaThresholdTestCases()...)
	tests = append(tests, commonIgnoredBorderThicknessTestCases()...)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				AlgorithmNameOptKey: string(FuzzyMatching),
			}
			if tc.maxDifferentPixels != missing {
				optionalKeys[string(MaxDifferentPixels)] = tc.maxDifferentPixels
			}
			if tc.pixelDeltaThreshold != missing {
				optionalKeys[string(PixelDeltaThreshold)] = tc.pixelDeltaThreshold
			}
			if tc.ignoredBorderThickness != missing {
				optionalKeys[string(IgnoredBorderThickness)] = tc.ignoredBorderThickness
			}

			algorithmName, matcher, err := MakeMatcher(optionalKeys)

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

func TestMakeMatcher_SobelFuzzyMatching(t *testing.T) {
	unittest.SmallTest(t)

	type sobelFuzzyMatchingTestCase struct {
		name                   string
		edgeThreshold          string
		maxDifferentPixels     string
		pixelDeltaThreshold    string
		ignoredBorderThickness string
		want                   sobel.Matcher
		error                  string
	}

	tests := []sobelFuzzyMatchingTestCase{
		{
			name:                   "all parameters missing, returns error",
			edgeThreshold:          missing,
			maxDifferentPixels:     missing,
			pixelDeltaThreshold:    missing,
			ignoredBorderThickness: missing,

			error: "required image matching parameter not found",
		},
		{
			name:                   "edge threshold: missing, returns error",
			edgeThreshold:          missing,
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			error:                  `required image matching parameter not found: "sobel_edge_threshold"`,
		},
		{
			name:                   "edge threshold: empty, returns error",
			edgeThreshold:          "",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			error:                  `image matching parameter "sobel_edge_threshold" cannot be empty`,
		},
		{
			name:                   "edge threshold: non-integer value, returns error",
			edgeThreshold:          "not an integer",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			error:                  "invalid syntax",
		},
		{
			name:                   "edge threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			edgeThreshold:          fmt.Sprintf("%d", math.MinInt32-1),
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			error:                  "out of range",
		},
		{
			name:                   "edge threshold: non-32-bit integer (math.MaxInt32 + 1), returns error",
			edgeThreshold:          fmt.Sprintf("%d", math.MaxInt32+1),
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			error:                  "out of range",
		},
		{
			name:                   "edge threshold: value < 0, returns error",
			edgeThreshold:          "-1",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			error:                  `image matching parameter "sobel_edge_threshold" must be between 0 and 255, was: -1`,
		},
		{
			name:                   "edge threshold: value = 0, success",
			edgeThreshold:          "0",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			want: sobel.Matcher{
				Matcher: fuzzy.Matcher{
					MaxDifferentPixels:     0,
					PixelDeltaThreshold:    0,
					IgnoredBorderThickness: 0,
				},
				EdgeThreshold: 0,
			},
		},
		{
			name:                   "edge threshold: 0 < value < 255, success",
			edgeThreshold:          "254",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			want: sobel.Matcher{
				Matcher: fuzzy.Matcher{
					MaxDifferentPixels:     0,
					PixelDeltaThreshold:    0,
					IgnoredBorderThickness: 0,
				},
				EdgeThreshold: 254,
			},
		},
		{
			name:                   "edge threshold: value = 255, success",
			edgeThreshold:          "255",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			want: sobel.Matcher{
				Matcher: fuzzy.Matcher{
					MaxDifferentPixels:     0,
					PixelDeltaThreshold:    0,
					IgnoredBorderThickness: 0,
				},
				EdgeThreshold: 255,
			},
		},
		{
			name:                   "edge threshold: value > 255, returns error",
			edgeThreshold:          "256",
			maxDifferentPixels:     "0",
			pixelDeltaThreshold:    "0",
			ignoredBorderThickness: "0",
			error:                  `image matching parameter "sobel_edge_threshold" must be between 0 and 255, was: 256`,
		},
	}

	// Append test cases for FuzzyMatching.
	appendCommonTestCase := func(tc fuzzyMatchingTestCase) {
		tests = append(tests, sobelFuzzyMatchingTestCase{
			name:                   tc.name,
			edgeThreshold:          "0",
			maxDifferentPixels:     tc.maxDifferentPixels,
			pixelDeltaThreshold:    tc.pixelDeltaThreshold,
			ignoredBorderThickness: tc.ignoredBorderThickness,
			want: sobel.Matcher{
				Matcher:       tc.want,
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
	for _, tc := range commonIgnoredBorderThicknessTestCases() {
		appendCommonTestCase(tc)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				AlgorithmNameOptKey: string(SobelFuzzyMatching),
			}
			if tc.edgeThreshold != missing {
				optionalKeys[string(EdgeThreshold)] = tc.edgeThreshold
			}
			if tc.maxDifferentPixels != missing {
				optionalKeys[string(MaxDifferentPixels)] = tc.maxDifferentPixels
			}
			if tc.pixelDeltaThreshold != missing {
				optionalKeys[string(PixelDeltaThreshold)] = tc.pixelDeltaThreshold
			}
			if tc.ignoredBorderThickness != missing {
				optionalKeys[string(IgnoredBorderThickness)] = tc.ignoredBorderThickness
			}

			algorithmName, matcher, err := MakeMatcher(optionalKeys)

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
