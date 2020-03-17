package imgmatching

import (
	"math"
	"strconv"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/gold-client/go/imgmatching/fuzzy"
	"go.skia.org/infra/gold-client/go/imgmatching/sobel"
)

// MatcherFactory represents an Matcher factory that builds a Matcher from a map of
// optional keys. It is implemented by MatcherFactoryImpl.
//
// The purpose of this interface is to make testing easier.
type MatcherFactory interface {

	// Make takes a map of optional keys and returns the specified image matching algorithm name, and
	// the corresponding Matcher instance; or nil if none is specified.
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

	// Exact matching by default. No Matcher implementation necessary for exact matching.
	if !ok || algorithmName == ExactMatching {
		return ExactMatching, nil, nil
	}

	switch algorithmName {
	case ExactMatching:
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
		return "", nil, skerr.Fmt("unrecognized image matching algorithm: %s", algorithmName)
	}
}

// makeFuzzyMatcher returns a fuzzy.FuzzyMatcher instance set up with the parameter values in the
// given optional keys map.
func makeFuzzyMatcher(optionalKeys map[string]string) (*fuzzy.FuzzyMatcher, error) {
	maxDifferentPixels, err := getIntParameter(FuzzyMatchingMaxDifferentPixels, 0, math.MaxInt64, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// The maximum value corresponds to the maximum possible per-channel delta sum. This assumes four
	// channels (R, G, B, A), each represented with 8 bits; hence 256*4.
	pixelDeltaThreshold, err := getIntParameter(FuzzyMatchingPixelDeltaThreshold, 0, 255*4, optionalKeys)
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
	// This assumes the Sobel operator returns an 8-bit per-pixel value indicating how likely a pixel
	// is to be part of an edge.
	edgeThreshold, err := getIntParameter(SobelFuzzyMatchingEdgeThreshold, 0, 255, optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	fuzzyMatcher, err := makeFuzzyMatcher(optionalKeys)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &sobel.SobelFuzzyMatcher{
		FuzzyMatcher:  *fuzzyMatcher,
		EdgeThreshold: edgeThreshold,
	}, nil
}

// getIntParameter extracts and validates the given integer parameter from the optional keys map.
func getIntParameter(name AlgorithmParameterNameOptionalKey, min, max int, optionalKeys map[string]string) (int, error) {
	stringVal, ok := optionalKeys[string(name)]
	if !ok {
		return 0, skerr.Fmt("required parameter not found: %s", name)
	}

	int64Val, err := strconv.ParseInt(stringVal, 10, 32)
	intVal := int(int64Val)
	if err != nil {
		return 0, skerr.Fmt("parsing integer value for image matching parameter %s: %s", name, err.Error())
	}

	if intVal < min || intVal > max {
		return 0, skerr.Fmt("image matching parameter %s must be between %d and %d, was: %d", name, min, max, intVal)
	}

	return intVal, nil
}

// Make sure MatcherFactoryImpl fulfills the MatcherFactory interface.
var _ MatcherFactory = (*MatcherFactoryImpl)(nil)
