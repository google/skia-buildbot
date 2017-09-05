package goldcts

import (
	"sync"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/search"
	"go.skia.org/infra/golden/go/tsuite"
	"go.skia.org/infra/golden/go/types"
)

var DefaultTestNames = []string{
	"typefacerendering_pfbAndroid",
	"typefacerendering_pfbMac",
	"typefacerendering_pfbChromeOS",
	"typefacerendering_pfaChromeOS",
	"coloremoji",
	"badpaint",
	"typefacerendering_pfaAndroid",
	"fontmgr_matchChromeOS",
	"typefacestylesChromeOS",
	"typefacestyles_kerningChromeOS",
	"typefacerendering_pfbUbuntu",
	"lcdtextUbuntu",
	"fontmgr_matchUbuntu16",
	"verttext2Ubuntu",
	"typefacerendering_pfaUbuntu",
	"typefacestyles_kerningUbuntu",
	"rects_as_paths",
	"typefacerenderingChromeOS",
	"lcdtextChromeOS",
	"fontscalerUbuntu",
	"typefacestylesUbuntu",
	"typefacerenderingUbuntu",
	"discard",
	"fontmgr_iterUbuntu16",
	"gammatextUbuntu",
	"fontmgr_bounds_0.75_0Win7GDI",
	"fontmgr_iterWin8",
	"fontmgr_matchWin8",
	"fontmgr_matchWin7GDI",
	"fontmgr_iter_factoryWin8",
	"animatedGif",
	"savelayer_unclipped",
	"longlinedash",
	"clipped-bitmap-shaders-clamp",
	"fontmgr_boundsWin8GDI",
	"deferred_texture_image_none",
	"drawregion",
	"fontmgr_iterWin8GDI",
	"fontscalerChromeOS",
	"fontmgr_bounds_0.75_0Win8GDI",
	"typefacestyles_kerningMac",
	"small_color_stop",
	"copy_on_write_retain",
	"distantclip",
	"fontmgr_bounds_1_-0.25Win8GDI",
	"fontmgr_iter_factoryWin7GDI",
	"fontmgr_iterWin7",
	"all_variants_8888",
	"giantbitmap_clamp_point_scale",
	"verttext2ChromeOS",
}

type CTSResult struct {
}

type GoldCTS struct {
	current map[string]*CTSResult

	mutex sync.Mutex
}

func New() (*GoldCTS, error) {
	return nil, nil
}

func Get() map[string]*CTSResult {
	return nil
}

// Placeholder function that continously calculates CTS compliance for a given commit.
func CalcCTSResults() {
	// Iterate over traces of the non-ignored tile.

	// Group traces by device

	// For each device evalute the result by backend.

	// Return a list of devices and whether they pass the tests with a breakdown
	// of each tests pass/fail result maby with a percentage attached to ti.
}

// func BuildSuite(testNames []string, tile *tiling.Tile, storages *storage.Storage) (*tsuite.CompatTestSuite, error) {
func BuildSuite(testNames []string, searchAPI *search.SearchAPI, diffStore diff.DiffStore) (*tsuite.CompatTestSuite, error) {
	// Iterate over the tile. Pick all the tests we are interested in.
	ret := tsuite.New()

	q := search.Query{
		Limit: -1,
		Query: map[string][]string{
			types.PRIMARY_KEY_FIELD: {testNames[0]},
			types.CORPUS_FIELD:      {"gm"},
		},
		IncludeIgnores: false,
		Pos:            true,
		Head:           false,
	}

	// testsMap := util.NewStringSet(testNames)
	for _, testName := range testNames {
		q.Query[types.PRIMARY_KEY_FIELD][0] = testName
		result, err := searchAPI.Filter(&q)
		if err != nil {
			return nil, err
		}

		if (len(result) == 1) || (len(result[testName]) > 0) {
			classifier := tsuite.NewMemorizer()
			for _, digestInfo := range result[testName] {
				img, err := diffStore.GetImage(digestInfo.Digest)
				if err != nil {
					sklog.Errorf("Unable to get image for %s/%s", testName, digestInfo.Digest)
					continue
				}
				// classifier.Add(digestInfo.Digest, img, digestInfo.Label)
			}

		}

		ret.Add(testName, tsuite.AlwaysTrueClassifier{})
	}

	return ret, nil
}
