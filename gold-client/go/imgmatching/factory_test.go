package imgmatching

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/gold-client/go/imgmatching/exact"
	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
	"go.skia.org/infra/gold-client/go/imgmatching/sample_area"
	"go.skia.org/infra/gold-client/go/imgmatching/sobel"
)

func TestMakeMatcher_UnknownAlgorithm_ReturnsError(t *testing.T) {
	_, _, err := MakeMatcher(map[string]string{
		AlgorithmNameOptKey: "FakeAlgorithm",
	})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), `unrecognized image matching algorithm: "FakeAlgorithm"`)
}

func TestMakeMatcher_NoAlgorithmSpecified_ReturnsExactMatching(t *testing.T) {
	algorithmName, matcher, err := MakeMatcher(map[string]string{})

	assert.NoError(t, err)
	assert.Equal(t, ExactMatching, algorithmName)
	assert.Equal(t, matcher, &exact.Matcher{})
}

func TestMakeMatcher_ExactMatchingExplicitlySpecified_ReturnsExactMatching(t *testing.T) {
	algorithmName, matcher, err := MakeMatcher(map[string]string{
		AlgorithmNameOptKey: string(ExactMatching),
	})

	assert.NoError(t, err)
	assert.Equal(t, ExactMatching, algorithmName)
	assert.Equal(t, matcher, &exact.Matcher{})
}

// missing is a sentinel value used to represent missing parameter values.
const missing = "missing value"

// fuzzyMatchingTestCase represents a test case for MakeMatcher() where a fuzzy.Matcher is
// instantiated.
type fuzzyMatchingTestCase struct {
	name                          string
	maxDifferentPixels            string
	pixelDeltaThreshold           string
	pixelPerChannelDeltaThreshold string
	ignoredBorderThickness        string
	want                          fuzzy.Matcher
	error                         string
}

// commonMaxDifferentPixelsTestCases returns test cases for the MaxDifferentPixels
// optional key.
//
// These tests are shared between TestMakeMatcher_FuzzyMatching and
// TestMakeMatcher_SobelFuzzyMatching.
func commonMaxDifferentPixelsTestCases() []fuzzyMatchingTestCase {
	return []fuzzyMatchingTestCase{
		{
			name:                          "max different pixels: missing, returns error",
			maxDifferentPixels:            missing,
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         `required image matching parameter not found: "fuzzy_max_different_pixels"`,
		},
		{
			name:                          "max different pixels: empty, returns error",
			maxDifferentPixels:            "",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_max_different_pixels" cannot be empty`,
		},
		{
			name:                          "max different pixels: non-integer value, returns error",
			maxDifferentPixels:            "not an integer",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         "invalid syntax",
		},
		{
			name:                          "max different pixels: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:            fmt.Sprintf("%d", math.MinInt32-1),
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         "out of range",
		},
		{
			name:                          "max different pixels: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:            fmt.Sprintf("%d", math.MaxInt32+1),
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         "out of range",
		},
		{
			name:                          "max different pixels: value = -1, returns error",
			maxDifferentPixels:            "-1",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_max_different_pixels" must be at least 0, was: -1`,
		},
		{
			name:                          "max different pixels: value = 0, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "max different pixels: value = math.MaxInt32, success",
			maxDifferentPixels:            fmt.Sprintf("%d", math.MaxInt32),
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            math.MaxInt32,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
	}
}

// commonPixelDeltaThresholdTestCases returns test cases for the PixelDeltaThreshold
// and PixelPerChannelDeltaThreshold optional keys.
//
// These tests are shared between TestMakeMatcher_FuzzyMatching and
// TestMakeMatcher_SobelFuzzyMatching.
func commonPixelDeltaThresholdTestCases() []fuzzyMatchingTestCase {
	return []fuzzyMatchingTestCase{
		{
			name:                          "pixel delta thresholds: both missing, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           missing,
			pixelPerChannelDeltaThreshold: missing,
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "pixel delta thresholds: both unset, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "pixel delta thresholds: both set, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "1",
			pixelPerChannelDeltaThreshold: "1",
			ignoredBorderThickness:        missing,
			error:                         `only one of fuzzy_pixel_delta_threshold and fuzzy_pixel_per_channel_delta_threshold can be set`,
		},
		{
			name:                          "pixel delta threshold: empty, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_pixel_delta_threshold" cannot be empty`,
		},
		{
			name:                          "pixel per channel delta threshold: empty, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_pixel_per_channel_delta_threshold" cannot be empty`,
		},
		{
			name:                          "pixel delta threshold: non-integer value, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "not an integer",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         "invalid syntax",
		},
		{
			name:                          "pixel per channel delta threshold: non-integer value, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "not an integer",
			ignoredBorderThickness:        missing,
			error:                         "invalid syntax",
		},
		{
			name:                          "pixel delta threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           fmt.Sprintf("%d", math.MinInt32-1),
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         "out of range",
		},
		{
			name:                          "pixel per channel delta threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: fmt.Sprintf("%d", math.MinInt32-1),
			ignoredBorderThickness:        missing,
			error:                         "out of range",
		},
		{
			name:                          "pixel delta threshold: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           fmt.Sprintf("%d", math.MaxInt32+1),
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         "out of range",
		},
		{
			name:                          "pixel per channel delta threshold: per-channel non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: fmt.Sprintf("%d", math.MaxInt32+1),
			ignoredBorderThickness:        missing,
			error:                         "out of range",
		},
		{
			name:                          "pixel delta threshold: value = -1, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "-1",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1020, was: -1`,
		},
		{
			name:                          "pixel per channel delta threshold: value = -1, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "-1",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_pixel_per_channel_delta_threshold" must be between 0 and 255, was: -1`,
		},
		{
			name:                          "pixel delta threshold: value = 0, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "pixel per channel delta threshold: value = 0, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "1",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           1,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "pixel delta threshold: value = 1020, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "1020",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           1020,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "pixel per channel delta threshold: value = 255, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "255",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 255,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "pixel delta threshold: value = 1021, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "1021",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_pixel_delta_threshold" must be between 0 and 1020, was: 1021`,
		},
		{
			name:                          "pixel per channel delta threshold: value = 256, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "256",
			ignoredBorderThickness:        missing,
			error:                         `image matching parameter "fuzzy_pixel_per_channel_delta_threshold" must be between 0 and 255, was: 256`,
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
			name:                          "ignored border thickness: missing, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        missing,
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "ignored border thickness: empty, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "",
			error:                         `image matching parameter "fuzzy_ignored_border_thickness" cannot be empty`,
		},
		{
			name:                          "ignored border thickness: non-integer value, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "not an integer",
			error:                         "invalid syntax",
		},
		{
			name:                          "ignored border thickness: non-32-bit integer (math.MinInt32 - 1), returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        fmt.Sprintf("%d", math.MinInt32-1),
			error:                         "out of range",
		},
		{
			name:                          "ignored border thickness: non-32-bit integer (math.MaxInt32 + 1), returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        fmt.Sprintf("%d", math.MaxInt32+1),
			error:                         "out of range",
		},
		{
			name:                          "ignored border thickness: value = -1, returns error",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "-1",
			error:                         `image matching parameter "fuzzy_ignored_border_thickness" must be at least 0, was: -1`,
		},
		{
			name:                          "ignored border thickness: value = 0, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        0,
			},
		},
		{
			name:                          "ignored border thickness: value = 1, success",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "1",
			want: fuzzy.Matcher{
				MaxDifferentPixels:            0,
				PixelDeltaThreshold:           0,
				PixelPerChannelDeltaThreshold: 0,
				IgnoredBorderThickness:        1,
			},
		},
	}
}

func TestMakeMatcher_FuzzyMatching(t *testing.T) {
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
			if tc.pixelPerChannelDeltaThreshold != missing {
				optionalKeys[string(PixelPerChannelDeltaThreshold)] = tc.pixelPerChannelDeltaThreshold
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
	type sobelFuzzyMatchingTestCase struct {
		name                          string
		edgeThreshold                 string
		maxDifferentPixels            string
		pixelDeltaThreshold           string
		pixelPerChannelDeltaThreshold string
		ignoredBorderThickness        string
		want                          sobel.Matcher
		error                         string
	}

	tests := []sobelFuzzyMatchingTestCase{
		{
			name:                          "all parameters missing, returns error",
			edgeThreshold:                 missing,
			maxDifferentPixels:            missing,
			pixelDeltaThreshold:           missing,
			pixelPerChannelDeltaThreshold: missing,
			ignoredBorderThickness:        missing,

			error: "required image matching parameter not found",
		},
		{
			name:                          "edge threshold: missing, returns error",
			edgeThreshold:                 missing,
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			error:                         `required image matching parameter not found: "sobel_edge_threshold"`,
		},
		{
			name:                          "edge threshold: empty, returns error",
			edgeThreshold:                 "",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			error:                         `image matching parameter "sobel_edge_threshold" cannot be empty`,
		},
		{
			name:                          "edge threshold: non-integer value, returns error",
			edgeThreshold:                 "not an integer",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			error:                         "invalid syntax",
		},
		{
			name:                          "edge threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			edgeThreshold:                 fmt.Sprintf("%d", math.MinInt32-1),
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			error:                         "out of range",
		},
		{
			name:                          "edge threshold: non-32-bit integer (math.MaxInt32 + 1), returns error",
			edgeThreshold:                 fmt.Sprintf("%d", math.MaxInt32+1),
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			error:                         "out of range",
		},
		{
			name:                          "edge threshold: value < 0, returns error",
			edgeThreshold:                 "-1",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			error:                         `image matching parameter "sobel_edge_threshold" must be between 0 and 255, was: -1`,
		},
		{
			name:                          "edge threshold: value = 0, success",
			edgeThreshold:                 "0",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			want: sobel.Matcher{
				Matcher: fuzzy.Matcher{
					MaxDifferentPixels:            0,
					PixelDeltaThreshold:           0,
					PixelPerChannelDeltaThreshold: 0,
					IgnoredBorderThickness:        0,
				},
				EdgeThreshold: 0,
			},
		},
		{
			name:                          "edge threshold: 0 < value < 255, success",
			edgeThreshold:                 "254",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			want: sobel.Matcher{
				Matcher: fuzzy.Matcher{
					MaxDifferentPixels:            0,
					PixelDeltaThreshold:           0,
					PixelPerChannelDeltaThreshold: 0,
					IgnoredBorderThickness:        0,
				},
				EdgeThreshold: 254,
			},
		},
		{
			name:                          "edge threshold: value = 255, success",
			edgeThreshold:                 "255",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			want: sobel.Matcher{
				Matcher: fuzzy.Matcher{
					MaxDifferentPixels:            0,
					PixelDeltaThreshold:           0,
					PixelPerChannelDeltaThreshold: 0,
					IgnoredBorderThickness:        0,
				},
				EdgeThreshold: 255,
			},
		},
		{
			name:                          "edge threshold: value > 255, returns error",
			edgeThreshold:                 "256",
			maxDifferentPixels:            "0",
			pixelDeltaThreshold:           "0",
			pixelPerChannelDeltaThreshold: "0",
			ignoredBorderThickness:        "0",
			error:                         `image matching parameter "sobel_edge_threshold" must be between 0 and 255, was: 256`,
		},
	}

	// Append test cases for FuzzyMatching.
	appendCommonTestCase := func(tc fuzzyMatchingTestCase) {
		tests = append(tests, sobelFuzzyMatchingTestCase{
			name:                          tc.name,
			edgeThreshold:                 "0",
			maxDifferentPixels:            tc.maxDifferentPixels,
			pixelDeltaThreshold:           tc.pixelDeltaThreshold,
			pixelPerChannelDeltaThreshold: tc.pixelPerChannelDeltaThreshold,
			ignoredBorderThickness:        tc.ignoredBorderThickness,
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
			if tc.pixelPerChannelDeltaThreshold != missing {
				optionalKeys[string(PixelPerChannelDeltaThreshold)] = tc.pixelPerChannelDeltaThreshold
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

func TestMakeMatcher_SampleAreaMatching_Error(t *testing.T) {
	type sampleAreaErrorTestCase struct {
		name                            string
		sampleAreaWidth                 string
		maxDifferentPixelsPerArea       string
		sampleAreaChannelDeltaThreshold string
		error                           string
	}

	maxIntSqrt := int(math.Sqrt(math.MaxInt32))

	tests := []sampleAreaErrorTestCase{
		// Missing cases.
		{
			name:                            "sample area width: missing, returns error",
			sampleAreaWidth:                 missing,
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           `required image matching parameter not found: "sample_area_width"`,
		},
		{
			name:                            "max different pixels per area: missing, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       missing,
			sampleAreaChannelDeltaThreshold: "0",
			error:                           `required image matching parameter not found: "sample_area_max_different_pixels_per_area"`,
		},
		{
			name:                            "all parameters missing, returns error",
			sampleAreaWidth:                 missing,
			maxDifferentPixelsPerArea:       missing,
			sampleAreaChannelDeltaThreshold: missing,
			error:                           "required image matching parameter not found",
		},
		// Empty cases.
		{
			name:                            "sample area width: empty, returns error",
			sampleAreaWidth:                 "",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           `image matching parameter "sample_area_width" cannot be empty`,
		},
		{
			name:                            "max different pixels per area: empty, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           `image matching parameter "sample_area_max_different_pixels_per_area" cannot be empty`,
		},
		{
			name:                            "sample area channel delta threshold: empty, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "",
			error:                           `image matching parameter "sample_area_channel_delta_threshold" cannot be empty`,
		},
		// Non-integer cases.
		{
			name:                            "sample area width: non-integer value, returns error",
			sampleAreaWidth:                 "not an integer",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           "invalid syntax",
		},
		{
			name:                            "max different pixels per area: non-integer value, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "not an integer",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           "invalid syntax",
		},
		{
			name:                            "sample area channel delta threshold: non-integer value, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "not an integer",
			error:                           "invalid syntax",
		},
		// Non-32-bit integer cases.
		{
			name:                            "sample area width: non-32-bit integer (math.MinInt32 - 1), returns error",
			sampleAreaWidth:                 fmt.Sprintf("%d", math.MinInt32-1),
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           "out of range",
		},
		{
			name:                            "sample area width: non-32-bit integer (math.MaxInt32 + 1), returns error",
			sampleAreaWidth:                 fmt.Sprintf("%d", math.MaxInt32+1),
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           "out of range",
		},
		{
			name:                            "max different pixels per area: non-32-bit integer (math.MinInt32 - 1), returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       fmt.Sprintf("%d", math.MinInt32-1),
			sampleAreaChannelDeltaThreshold: "0",
			error:                           "out of range",
		},
		{
			name:                            "max different pixels per area: non-32-bit integer (math.MaxInt32 + 1), returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       fmt.Sprintf("%d", math.MaxInt32+1),
			sampleAreaChannelDeltaThreshold: "0",
			error:                           "out of range",
		},
		{
			name:                            "sample area channel delta threshold: non-32-bit integer (math.MinInt32 - 1), returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: fmt.Sprintf("%d", math.MinInt32-1),
			error:                           "out of range",
		},
		{
			name:                            "sample area channel delta threshold: non-32-bit integer (math.MaxInt32 + 1), returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: fmt.Sprintf("%d", math.MaxInt32+1),
			error:                           "out of range",
		},
		// Under lower limit cases.
		{
			name:                            "sample area width: value < 1, returns error",
			sampleAreaWidth:                 "0",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           fmt.Sprintf(`image matching parameter "sample_area_width" must be between 1 and %d, was: 0`, maxIntSqrt),
		},
		{
			name:                            "max different pixels per area: value < 0, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "-1",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           `image matching parameter "sample_area_max_different_pixels_per_area" must be between 0 and 4, was: -1`,
		},
		{
			name:                            "sample area channel delta threshold: value < 0, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "-1",
			error:                           `image matching parameter "sample_area_channel_delta_threshold" must be between 0 and 255, was: -1`,
		},
		// Over upper limit cases.
		{
			name:                            "sample area width: value > max int 32 square root, returns error",
			sampleAreaWidth:                 fmt.Sprintf("%d", maxIntSqrt+1),
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           fmt.Sprintf(`image matching parameter "sample_area_width" must be between 1 and %d, was: %d`, maxIntSqrt, maxIntSqrt+1),
		},
		{
			name:                            "max different pixels per area: value > number of sampled pixels, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "5",
			sampleAreaChannelDeltaThreshold: "0",
			error:                           `image matching parameter "sample_area_max_different_pixels_per_area" must be between 0 and 4, was: 5`,
		},
		{
			name:                            "sample area channel delta threshold: value > 255, returns error",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "256",
			error:                           `image matching parameter "sample_area_channel_delta_threshold" must be between 0 and 255, was: 256`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				AlgorithmNameOptKey: string(SampleAreaMatching),
			}
			if tc.sampleAreaWidth != missing {
				optionalKeys[string(SampleAreaWidth)] = tc.sampleAreaWidth
			}
			if tc.maxDifferentPixelsPerArea != missing {
				optionalKeys[string(MaxDifferentPixelsPerArea)] = tc.maxDifferentPixelsPerArea
			}
			if tc.sampleAreaChannelDeltaThreshold != missing {
				optionalKeys[string(SampleAreaChannelDeltaThreshold)] = tc.sampleAreaChannelDeltaThreshold
			}

			_, _, err := MakeMatcher(optionalKeys)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.error)
		})
	}
}

func TestMakeMatcher_SampleAreaMatching_Success(t *testing.T) {
	type sampleAreaSuccessTestCase struct {
		name                            string
		sampleAreaWidth                 string
		maxDifferentPixelsPerArea       string
		sampleAreaChannelDeltaThreshold string
		want                            sample_area.Matcher
	}

	maxIntSqrt := int(math.Sqrt(math.MaxInt32))

	tests := []sampleAreaSuccessTestCase{
		// Missing cases.
		{
			name:                            "sample area channel delta threshold: missing, uses default",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: missing,
			want: sample_area.Matcher{
				SampleAreaWidth:                 2,
				MaxDifferentPixelsPerArea:       0,
				SampleAreaChannelDeltaThreshold: 0,
			},
		},
		// At lower limit cases.
		{
			name:                            "sample area width: value = lower limit, success",
			sampleAreaWidth:                 "1",
			maxDifferentPixelsPerArea:       "1",
			sampleAreaChannelDeltaThreshold: "0",
			want: sample_area.Matcher{
				SampleAreaWidth:                 1,
				MaxDifferentPixelsPerArea:       1,
				SampleAreaChannelDeltaThreshold: 0,
			},
		},
		{
			name:                            "max different pixels per area: value = lower limit, success",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			want: sample_area.Matcher{
				SampleAreaWidth:                 2,
				MaxDifferentPixelsPerArea:       0,
				SampleAreaChannelDeltaThreshold: 0,
			},
		},
		{
			name:                            "sample area channel delta threshold: value = lower limit, success",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "1",
			sampleAreaChannelDeltaThreshold: "0",
			want: sample_area.Matcher{
				SampleAreaWidth:                 2,
				MaxDifferentPixelsPerArea:       1,
				SampleAreaChannelDeltaThreshold: 0,
			},
		},
		// At upper limit cases.
		{
			name:                            "sample area width: value = upper limit, success",
			sampleAreaWidth:                 fmt.Sprintf("%d", maxIntSqrt),
			maxDifferentPixelsPerArea:       "0",
			sampleAreaChannelDeltaThreshold: "0",
			want: sample_area.Matcher{
				SampleAreaWidth:                 maxIntSqrt,
				MaxDifferentPixelsPerArea:       0,
				SampleAreaChannelDeltaThreshold: 0,
			},
		},
		{
			name:                            "max different pixels per area: value = upper limit, success",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "4",
			sampleAreaChannelDeltaThreshold: "0",
			want: sample_area.Matcher{
				SampleAreaWidth:                 2,
				MaxDifferentPixelsPerArea:       4,
				SampleAreaChannelDeltaThreshold: 0,
			},
		},
		{
			name:                            "sample area channel delta threshold: value = upper limit, success",
			sampleAreaWidth:                 "2",
			maxDifferentPixelsPerArea:       "1",
			sampleAreaChannelDeltaThreshold: "255",
			want: sample_area.Matcher{
				SampleAreaWidth:                 2,
				MaxDifferentPixelsPerArea:       1,
				SampleAreaChannelDeltaThreshold: 255,
			},
		},
		// Between limits cases already handled by the lower limit cases for the
		// other arguments.
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			optionalKeys := map[string]string{
				AlgorithmNameOptKey: string(SampleAreaMatching),
			}
			optionalKeys[string(SampleAreaWidth)] = tc.sampleAreaWidth
			optionalKeys[string(MaxDifferentPixelsPerArea)] = tc.maxDifferentPixelsPerArea
			if tc.sampleAreaChannelDeltaThreshold != missing {
				optionalKeys[string(SampleAreaChannelDeltaThreshold)] = tc.sampleAreaChannelDeltaThreshold
			}

			algorithmName, matcher, err := MakeMatcher(optionalKeys)

			assert.NoError(t, err)
			assert.Equal(t, SampleAreaMatching, algorithmName)
			assert.Equal(t, &tc.want, matcher)
		})
	}
}
