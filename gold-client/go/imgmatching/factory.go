package imgmatching

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/gold-client/go/imgmatching/exact"
	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
	"go.skia.org/infra/gold-client/go/imgmatching/sample_area"
	"go.skia.org/infra/gold-client/go/imgmatching/sobel"
)

// MakeMatcher takes a map of optional keys and returns the specified image matching algorithm
// name, and the corresponding Matcher instance (or nil if none is specified).
//
// It returns a non-nil error if the specified image matching algorithm is invalid, or if any
// required parameters are not found, or if the parameter values are not valid.
func MakeMatcher(optionalKeys map[string]string) (AlgorithmName, Matcher, error) {
	algorithmNameStr, ok := optionalKeys[AlgorithmNameOptKey]
	algorithmName := AlgorithmName(algorithmNameStr)

	// Exact matching by default.
	if !ok {
		algorithmName = ExactMatching
	}

	switch algorithmName {
	case ExactMatching:
		return ExactMatching, &exact.Matcher{}, nil

	case FuzzyMatching:
		matcher, err := makeFuzzyMatcher(optionalKeys)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		return FuzzyMatching, matcher, nil

	case SampleAreaMatching:
		matcher, err := makeSampleAreaMatcher(optionalKeys)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		return SampleAreaMatching, matcher, nil

	case SobelFuzzyMatching:
		matcher, err := makeSobelFuzzyMatcher(optionalKeys)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		return SobelFuzzyMatching, matcher, nil

	default:
		return "", nil, skerr.Fmt("unrecognized image matching algorithm: %q", algorithmName)
	}
}

// makeFuzzyMatcher returns a fuzzy.Matcher instance set up with the parameter values in the
// given optional keys map.
func makeFuzzyMatcher(optionalKeys map[string]string) (*fuzzy.Matcher, error) {
	maxDifferentPixels, err := getAndValidateIntParameter(MaxDifferentPixels, 0, math.MaxInt32, true /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// The maximum value corresponds to the maximum possible per-channel delta sum. This assumes four
	// channels (R, G, B, A), each represented with 8 bits; hence 1020 = 255*4.
	pixelDeltaThreshold, err := getAndValidateIntParameter(PixelDeltaThreshold, 0, 1020, false /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// The maximum value corresponds to the maximum possible channel value. This assumes 8 bits with
	// a max of 255 for a single channel.
	pixelPerChannelDeltaThreshold, err := getAndValidateIntParameter(
		PixelPerChannelDeltaThreshold, 0, 255, false /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Ensure that at most one of the sum or per-channel options is set.
	if pixelDeltaThreshold > 0 && pixelPerChannelDeltaThreshold > 0 {
		return nil, skerr.Fmt(
			"only one of %s and %s can be set", PixelDeltaThreshold, PixelPerChannelDeltaThreshold)
	}

	ignoredBorderThickness, err := getAndValidateIntParameter(IgnoredBorderThickness, 0, math.MaxInt32, false /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &fuzzy.Matcher{
		MaxDifferentPixels:            maxDifferentPixels,
		PixelDeltaThreshold:           pixelDeltaThreshold,
		PixelPerChannelDeltaThreshold: pixelPerChannelDeltaThreshold,
		IgnoredBorderThickness:        ignoredBorderThickness,
	}, nil
}

// makeSampleAreaMatcher returns a sample_area.Matcher instance set up with the
// parameter values in the given optional keys map.
func makeSampleAreaMatcher(optionalKeys map[string]string) (*sample_area.Matcher, error) {
	// Determine the width/height of each sample that will be compared between the
	// two images. We need to set the max lower since we will be squaring the
	// value later.
	maxIntSqrt := math.Sqrt(math.MaxInt32)
	sampleAreaWidth, err := getAndValidateIntParameter(
		SampleAreaWidth, 1, int(maxIntSqrt), true /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Determine how many pixels in the sample area are allowed to differ and
	// still be treated as a successful comparison.
	maxDifferentPixels, err := getAndValidateIntParameter(
		MaxDifferentPixelsPerArea, 0, sampleAreaWidth*sampleAreaWidth,
		true /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// Determine what the tolerance is for slightly different pixels. This is the
	// maximum per-pixel, per-channel delta allowed. Defaults to 0 if not
	// specified.
	sampleAreaChannelDeltaThreshold, err := getAndValidateIntParameter(
		SampleAreaChannelDeltaThreshold, 0, 255, false /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &sample_area.Matcher{
		SampleAreaWidth:                 sampleAreaWidth,
		MaxDifferentPixelsPerArea:       maxDifferentPixels,
		SampleAreaChannelDeltaThreshold: sampleAreaChannelDeltaThreshold,
	}, nil
}

// makeSobelFuzzyMatcher returns a sobel.Matcher instance set up with the parameter
// values in the given optional keys map.
func makeSobelFuzzyMatcher(optionalKeys map[string]string) (*sobel.Matcher, error) {
	// Instantiate the fuzzy.Matcher that will be embedded in the sobel.Matcher.
	fuzzyMatcher, err := makeFuzzyMatcher(optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// This assumes the Sobel operator returns an 8-bit per-pixel value indicating how likely a pixel
	// is to be part of an edge.
	edgeThreshold, err := getAndValidateIntParameter(EdgeThreshold, 0, 255, true /* =required */, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &sobel.Matcher{
		Matcher:       *fuzzyMatcher,
		EdgeThreshold: edgeThreshold,
	}, nil
}

// getAndValidateIntParameter extracts and validates the given required integer parameter from the
// given map of optional keys.
//
// Minimum and maximum value validation can be disabled by setting parameters min and max to
// math.MinInt32 and math.MaxInt32, respectively.
//
// If required is false and the parameter is not present in the map of optional keys, a value of 0
// will be returned.
func getAndValidateIntParameter(name AlgorithmParamOptKey, min, max int, required bool, optionalKeys map[string]string) (int, error) {
	// Validate bounds.
	if min >= max {
		// This is almost surely a programming error.
		panic(fmt.Sprintf("min must be strictly less than max, min was %d, max was %d", min, max))
	}

	// Validate presence.
	stringVal, ok := optionalKeys[string(name)]
	if !ok {
		if required {
			return 0, skerr.Fmt("required image matching parameter not found: %q", name)
		}
		return 0, nil
	}

	// Value cannot be empty.
	if strings.TrimSpace(stringVal) == "" {
		return 0, skerr.Fmt("image matching parameter %q cannot be empty", name)
	}

	// Value must be a valid 32-bit integer.
	//
	// Note: The "int" type in Go has a platform-specific bit size of *at least* 32 bits, so we
	// explicitly parse the value as a 32-bit int to keep things deterministic across platforms.
	// Additionally, this ensures the math.MinInt32 and math.MaxInt32 sentinel values for the mix and
	// max parameters work as expected.
	int64Val, err := strconv.ParseInt(stringVal, 0, 32)
	if err != nil {
		return 0, skerr.Fmt("parsing integer value for image matching parameter %q: %q", name, err.Error())
	}
	intVal := int(int64Val)

	// Value must be between bounds.
	if intVal < min || intVal > max {
		// No lower bound, so value must be violating the upper bound.
		if min == math.MinInt32 {
			return 0, skerr.Fmt("image matching parameter %q must be at most %d, was: %d", name, max, int64Val)
		}

		// No upper bound, so value must be violating the lower bound.
		if max == math.MaxInt32 {
			return 0, skerr.Fmt("image matching parameter %q must be at least %d, was: %d", name, min, int64Val)
		}

		// Value has both an upper and lower bound.
		return 0, skerr.Fmt("image matching parameter %q must be between %d and %d, was: %d", name, min, max, int64Val)
	}

	return intVal, nil
}
