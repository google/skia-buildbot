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

// MatcherFactory represents a Matcher factory that builds a Matcher from a map of optional keys.
// It is implemented by MatcherFactoryImpl.
//
// The purpose of this interface is to make testing easier.
type MatcherFactory interface {
	// Make takes a map of optional keys and returns the specified image matching algorithm name, and
	// the corresponding Matcher instance (or nil if none is specified).
	//
	// It returns a non-nil error if the specified image matching algorithm is invalid, or if any
	// required parameters are not found, or if the parameter values are not valid.
	Make(optionalKeys map[string]string) (AlgorithmName, Matcher, error)
}

// MatcherFactoryImpl implements the MatcherFactory interface.
type MatcherFactoryImpl struct{}

// Make implements the MatcherFactory interface.
func (m MatcherFactoryImpl) Make(optionalKeys map[string]string) (AlgorithmName, Matcher, error) {
	algorithmNameStr, ok := optionalKeys[ImageMatchingAlgorithmOptionalKey]
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
	maxDifferentPixels, err := getAndValidateInt64Parameter(FuzzyMatchingMaxDifferentPixels, 0, math.MaxInt64, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// The maximum value corresponds to the maximum possible per-channel delta sum. This assumes four
	// channels (R, G, B, A), each represented with 8 bits; hence 256*4.
	pixelDeltaThreshold, err := getAndValidateInt64Parameter(FuzzyMatchingPixelDeltaThreshold, 0, 256*4, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &fuzzy.FuzzyMatcher{
		MaxDifferentPixels:  uint32(maxDifferentPixels),
		PixelDeltaThreshold: uint32(pixelDeltaThreshold),
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
	edgeThreshold, err := getAndValidateInt64Parameter(SobelFuzzyMatchingEdgeThreshold, 0, 255, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &sobel.SobelFuzzyMatcher{
		FuzzyMatcher:  *fuzzyMatcher,
		EdgeThreshold: uint8(edgeThreshold),
	}, nil
}

// getAndValidateInt64Parameter extracts and validates the given required integer parameter from
// the given map of optional keys.
//
// Minimum and maximum value validation can be disabled by setting parameters min and max to
// math.MinInt64 and math.MaxInt64, respectively.
func getAndValidateInt64Parameter(name AlgorithmParameterNameOptionalKey, min, max int64, optionalKeys map[string]string) (int64, error) {
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

	// Value must be a valid integer.
	int64Val, err := strconv.ParseInt(stringVal, 10, 64)
	if err != nil {
		return 0, skerr.Fmt("parsing integer value for image matching parameter %q: %q", name, err.Error())
	}

	// Value must be between bounds.
	if int64Val < min || int64Val > max {
		// Value has an upper bound.
		if min == math.MinInt64 {
			return 0, skerr.Fmt("image matching parameter %q must be at most %d, was: %d", name, max, int64Val)
		}

		// Value has a lower bound.
		if max == math.MaxInt64 {
			return 0, skerr.Fmt("image matching parameter %q must be at least %d, was: %d", name, min, int64Val)
		}

		// Value has both an upper and lower bound.
		return 0, skerr.Fmt("image matching parameter %q must be between %d and %d, was: %d", name, min, max, int64Val)
	}

	return int64Val, nil
}

// Make sure MatcherFactoryImpl fulfills the MatcherFactory interface.
var _ MatcherFactory = (*MatcherFactoryImpl)(nil)
