package imgmatching

// Optional key used to indicate a non-exact matching algorithm.
const ImageMatchingAlgorithmOptionalKey = "image_matching_algorithm"

// AlgorithmName is a non-exact image matching algorithm specified via the
// ImageMatchingAlgorithmOptionalKey optional key, e.g. "fuzzy".
type AlgorithmName string

const (
	ExactMatching      = AlgorithmName("")
	FuzzyMatching      = AlgorithmName("fuzzy")
	SobelFuzzyMatching = AlgorithmName("sobel")
)

// AlgorithmParameterNameOptionalKey is an optional key indicating a parameter for the specified
// non-exact image matching algorithm, e.g. "fuzzy_max_different_pixels".
type AlgorithmParameterNameOptionalKey string

const (
	// Parameters for FuzzyMatching.
	FuzzyMatchingMaxDifferentPixels  = AlgorithmParameterNameOptionalKey("fuzzy_max_different_pixels")
	FuzzyMatchingPixelDeltaThreshold = AlgorithmParameterNameOptionalKey("fuzzy_pixel_delta_threshold")

	// Parameters for SobelFuzzyMatching.
	SobelFuzzyMatchingEdgeThreshold = AlgorithmParameterNameOptionalKey("sobel_edge_threshold")
)
