package imgmatching

// AlgorithmNameOptKey is the optional key used to indicate a non-exact matching algorithm.
const AlgorithmNameOptKey = "image_matching_algorithm"

// AlgorithmName is a non-exact image matching algorithm specified via the AlgorithmNameOptKey
// optional key, e.g. "fuzzy".
type AlgorithmName string

const (
	ExactMatching      = AlgorithmName("exact")
	FuzzyMatching      = AlgorithmName("fuzzy")
	SampleAreaMatching = AlgorithmName("sample_area")
	SobelFuzzyMatching = AlgorithmName("sobel")
)

// AlgorithmParamOptKey is an optional key indicating a parameter for the specified non-exact image
// matching algorithm, e.g. "fuzzy_max_different_pixels".
type AlgorithmParamOptKey string

const (
	// MaxDifferentPixels is the optional key used to specify the MaxDifferentPixels parameter of
	// algorithms FuzzyMatching and SobelFuzzyMatching.
	MaxDifferentPixels = AlgorithmParamOptKey("fuzzy_max_different_pixels")

	// PixelDeltaThreshold is the optional key used to specify the PixelDeltaThreshold parameter of
	// algorithms FuzzyMatching and SobelFuzzyMatching.
	PixelDeltaThreshold = AlgorithmParamOptKey("fuzzy_pixel_delta_threshold")

	// IgnoredBorderThickness is the optional key used to specify the IgnoredBorderThickness
	// parameter of algorithms FuzzyMatching and SobelFuzzyMatching.
	IgnoredBorderThickness = AlgorithmParamOptKey("fuzzy_ignored_border_thickness")

	// EdgeThreshold is the optional key used to specify the EdgeThreshold parameter of the
	// SobelFuzzyMatching algorithm.
	EdgeThreshold = AlgorithmParamOptKey("sobel_edge_threshold")

	// SampleAreaWidth is the optional key used to specify the SampleAreaWidth
	// parameter of the SampleAreaMatching algorithm.
	SampleAreaWidth = AlgorithmParamOptKey("sample_area_width")

	// MaxDifferentPixelsPerArea is the optional key used to specify the
	// MaxDifferentPixelsPerArea parameter of the SampleAreaMatching algorithm.
	MaxDifferentPixelsPerArea = AlgorithmParamOptKey("sample_area_max_different_pixels_per_area")

	// SampleAreaChannelDeltaThreshold is the optional key used to specify the
	// SampleAreaChannelDeltaThreshold parameter of the SampleAreaMatching algorithm.
	SampleAreaChannelDeltaThreshold = AlgorithmParamOptKey("sample_area_channel_delta_threshold")
)
