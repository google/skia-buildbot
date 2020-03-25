package imgmatching

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
	"go.skia.org/infra/gold-client/go/imgmatching/sobel"
)

// MakeMatcher takes a map of optional keys and returns the specified image matching algorithm
// name, and the corresponding Matcher instance (or nil if none is specified).
//
// It returns a non-nil error if the specified image matching algorithm is invalid, or if any
// required parameters are not found, or if the parameter values are not valid.
func MakeMatcher(optionalKeys map[string]string) (AlgorithmName, Matcher, error) {
	algorithmNameStr, ok := optionalKeys[AlgorithmOptionalKey]
	algorithmName := AlgorithmName(algorithmNameStr)

	// Exact matching by default.
	if !ok {
		algorithmName = ExactMatching
	}

	switch algorithmName {
	case ExactMatching:
		// No Matcher implementation necessary for exact matching as this is done ad-hoc.
		return ExactMatching, nil, nil

	case FuzzyMatching:
		matcher, err := makeFuzzyMatcher(optionalKeys)
		if err != nil {
			return "", nil, skerr.Wrap(err)
		}
		return FuzzyMatching, matcher, nil

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

// makeFuzzyMatcher returns a fuzzy.FuzzyMatcher instance set up with the parameter values in the
// given optional keys map.
func makeFuzzyMatcher(optionalKeys map[string]string) (*fuzzy.FuzzyMatcher, error) {
	maxDifferentPixels, err := getAndValidateIntParameter(FuzzyMatchingMaxDifferentPixels, 0, math.MaxInt32, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// The maximum value corresponds to the maximum possible per-channel delta sum. This assumes four
	// channels (R, G, B, A), each represented with 8 bits; hence 1020 = 255*4.
	pixelDeltaThreshold, err := getAndValidateIntParameter(FuzzyMatchingPixelDeltaThreshold, 0, 1020, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &fuzzy.FuzzyMatcher{
		MaxDifferentPixels:  maxDifferentPixels,
		PixelDeltaThreshold: pixelDeltaThreshold,
	}, nil
}

// makeSobelFuzzyMatcher returns a sobel.SobelFuzzyMatcher instance set up with the parameter
// values in the given optional keys map.
func makeSobelFuzzyMatcher(optionalKeys map[string]string) (*sobel.SobelFuzzyMatcher, error) {
	// Instantiate the fuzzy.FuzzyMatcher that will be embedded in the sobel.SobelFuzzyMatcher.
	fuzzyMatcher, err := makeFuzzyMatcher(optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// This assumes the Sobel operator returns an 8-bit per-pixel value indicating how likely a pixel
	// is to be part of an edge.
	edgeThreshold, err := getAndValidateIntParameter(SobelFuzzyMatchingEdgeThreshold, 0, 255, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &sobel.SobelFuzzyMatcher{
		FuzzyMatcher:  *fuzzyMatcher,
		EdgeThreshold: edgeThreshold,
	}, nil
}

// getAndValidateIntParameter extracts and validates the given required integer parameter from the
// given map of optional keys.
//
// Minimum and maximum value validation can be disabled by setting parameters min and max to
// math.MinInt32 and math.MaxInt32, respectively.
func getAndValidateIntParameter(name AlgorithmParameterOptionalKey, min, max int, optionalKeys map[string]string) (int, error) {
	// Validate bounds.
	if min >= max {
		// This is almost surely a programming error.
		panic(fmt.Sprintf("min must be strictly less than max, min was %d, max was %d", min, max))
	}

	// Validate presence.
	stringVal, ok := optionalKeys[string(name)]
	if !ok {
		return 0, skerr.Fmt("required image matching parameter not found: %q", name)
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
