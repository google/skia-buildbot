package imgmatching

// Optional key used to indicate a non-exact matching algorithm.
const AlgorithmOptionalKey = "image_matching_algorithm"

// AlgorithmName is a non-exact image matching algorithm specified via the AlgorithmOptionalKey
// optional key, e.g. "fuzzy".
type AlgorithmName string

const (
	ExactMatching      = AlgorithmName("") // TODO(lovisolo): Set to a non-empty string.
	FuzzyMatching      = AlgorithmName("fuzzy")
	SobelFuzzyMatching = AlgorithmName("sobel")
)

// AlgorithmParameterOptionalKey is an optional key indicating a parameter for the specified
// non-exact image matching algorithm, e.g. "fuzzy_max_different_pixels".
type AlgorithmParameterOptionalKey string
