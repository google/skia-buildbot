package analysis

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"

	"skia.googlesource.com/buildbot.git/go/testutils"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/diff"
	"skia.googlesource.com/buildbot.git/golden/go/expstorage"
	"skia.googlesource.com/buildbot.git/golden/go/filediffstore"
	"skia.googlesource.com/buildbot.git/golden/go/types"
	ptypes "skia.googlesource.com/buildbot.git/perf/go/types"
)

var (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/goldentile.json.zip"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/goldentile.json.gz"
)

func init() {
	filediffstore.Init()
}

func TestGetListTestDetails(t *testing.T) {
	const CORPUS = "corpus1"

	digests := [][]string{
		[]string{"d_11", "d_12", ptypes.MISSING_DIGEST, "d_14"},
		[]string{"d_21", ptypes.MISSING_DIGEST, "d_23", "d_24"},
		[]string{"d_31", "d_32", "d_33", "d_34"},
		[]string{ptypes.MISSING_DIGEST, "d_42", "d_43", "d_44"},
		[]string{"d_51", "d_52", ptypes.MISSING_DIGEST, "d_54"},
	}

	params := []map[string]string{
		map[string]string{types.PRIMARY_KEY_FIELD: "t1", types.CORPUS_FIELD: CORPUS, "p1": "v11", "p2": "v21", "p3": "v31"},
		map[string]string{types.PRIMARY_KEY_FIELD: "t2", types.CORPUS_FIELD: CORPUS, "p1": "v12", "p2": "v21", "p3": "v32"},
		map[string]string{types.PRIMARY_KEY_FIELD: "t3", types.CORPUS_FIELD: CORPUS, "p1": "v11", "p2": "v22", "p3": "v33"},
		map[string]string{types.PRIMARY_KEY_FIELD: "t4", types.CORPUS_FIELD: CORPUS, "p1": "v12", "p2": "v22", "p3": "v34"},
		map[string]string{types.PRIMARY_KEY_FIELD: "t5", types.CORPUS_FIELD: CORPUS, "p1": "v13", "p2": "v22", "p3": "v34"},
	}

	start := time.Now().Unix()
	commits := []*ptypes.Commit{
		&ptypes.Commit{CommitTime: start + 10, Hash: "h1", Author: "John Doe 1"},
		&ptypes.Commit{CommitTime: start + 20, Hash: "h2", Author: "John Doe 2"},
		&ptypes.Commit{CommitTime: start + 30, Hash: "h3", Author: "John Doe 3"},
		&ptypes.Commit{CommitTime: start + 40, Hash: "h4", Author: "John Doe 4"},
	}

	LABELING_1 := map[string]types.TestClassification{
		"t1": map[string]types.Label{"d_12": types.NEGATIVE},
		"t2": map[string]types.Label{"d_23": types.POSITIVE, "d_24": types.POSITIVE},
		"t3": map[string]types.Label{"d_32": types.NEGATIVE, "d_33": types.POSITIVE},
		"t4": map[string]types.Label{"d_42": types.POSITIVE, "d_43": types.POSITIVE},
		"t5": map[string]types.Label{"d_51": types.POSITIVE, "d_54": types.POSITIVE},
	}
	STATUS_OK_1, UNTRIAGED_COUNT_1, NEGATIVE_COUNT_1 := false, 3, 0

	LABELING_2 := map[string]types.TestClassification{
		"t1": map[string]types.Label{"d_14": types.POSITIVE},
		"t3": map[string]types.Label{"d_34": types.POSITIVE},
		"t4": map[string]types.Label{"d_44": types.POSITIVE},
	}
	STATUS_OK_2, UNTRIAGED_COUNT_2, NEGATIVE_COUNT_2 := true, 0, 0

	assert.Equal(t, len(digests), len(params))
	assert.Equal(t, len(digests[0]), len(commits))

	diffStore := NewMockDiffStore()
	expStore := expstorage.NewMemExpectationsStore()
	tileStore := NewMockTileStore(t, digests, params, commits)
	ignoreStore := types.NewMemIgnoreStore()
	timeBetweenPolls := 10 * time.Hour
	a := NewAnalyzer(expStore, tileStore, diffStore, ignoreStore, mockUrlGenerator, filediffstore.MemCacheFactory, timeBetweenPolls)

	allTests, err := a.ListTestDetails(nil)
	assert.Nil(t, err)

	// Poll until ready
	for allTests == nil {
		time.Sleep(10 * time.Millisecond)
		allTests, err = a.ListTestDetails(nil)
		assert.Nil(t, err)
	}
	assert.NotNil(t, allTests)
	assert.Equal(t, len(params), len(allTests.Tests))

	// Make sure the lookup function works correctly.
	for _, oneTest := range a.current.TestDetails.Tests {
		found := a.current.TestDetails.lookup(oneTest.Name)
		assert.NotNil(t, found)
		assert.Equal(t, oneTest, found)
	}

	test1, err := a.GetTestDetails("t1", nil)
	assert.Nil(t, err)
	assert.NotNil(t, test1)
	assert.Equal(t, commits, test1.Commits)
	assert.Equal(t, 1, len(test1.Tests))
	assert.Equal(t, 0, len(test1.Query))
	assert.Equal(t, "t1", test1.Tests[0].Name)
	assert.Equal(t, 0, len(test1.Tests[0].Positive))
	assert.Equal(t, 0, len(test1.Tests[0].Negative))
	assert.Equal(t, 3, len(test1.Tests[0].Untriaged))

	// Query tiles
	list1, err := a.ListTestDetails(map[string][]string{"p1": []string{"v11"}, "head": []string{"0"}})
	assert.Nil(t, err)
	assert.Equal(t, 5, len(list1.Tests))
	assert.Equal(t, 4, len(list1.Commits))
	assert.Equal(t, 3, len(findTest(t, list1, "t1").Untriaged))
	assert.Equal(t, 4, len(findTest(t, list1, "t3").Untriaged))

	// Verify the other tests do not contain untriaged values.
	assert.Equal(t, 0, len(findTest(t, list1, "t2").Untriaged))
	assert.Equal(t, 0, len(findTest(t, list1, "t4").Untriaged))
	assert.Equal(t, 0, len(findTest(t, list1, "t5").Untriaged))

	// // Slice the tests
	list1, err = a.ListTestDetails(map[string][]string{"cs": []string{"h2"}, "head": []string{"0"}})
	assert.Nil(t, err)
	assert.Equal(t, 5, len(list1.Tests))

	assert.Equal(t, 2, len(findTest(t, list1, "t1").Untriaged))
	assert.Equal(t, 2, len(findTest(t, list1, "t2").Untriaged))

	assert.Equal(t, 3, len(findTest(t, list1, "t3").Untriaged))
	assert.Equal(t, 3, len(findTest(t, list1, "t4").Untriaged))
	assert.Equal(t, 2, len(findTest(t, list1, "t5").Untriaged))

	// Triage some digests.
	list1, err = a.SetDigestLabels(LABELING_1, "John Doe")
	assert.Nil(t, err)
	assert.Equal(t, len(LABELING_1), len(list1.Tests))

	status := a.GetStatus()
	assert.NotNil(t, status)
	assert.Equal(t, STATUS_OK_1, status.OK)
	assert.Equal(t, STATUS_OK_1, status.CorpStatus[CORPUS].OK)
	assert.Equal(t, UNTRIAGED_COUNT_1, status.CorpStatus[CORPUS].UntriagedCount)
	assert.Equal(t, NEGATIVE_COUNT_1, status.CorpStatus[CORPUS].NegativeCount)

	list1, err = a.SetDigestLabels(LABELING_2, "Jim Doe")
	assert.Nil(t, err)
	assert.Equal(t, len(LABELING_2), len(list1.Tests))
	list1, err = a.ListTestDetails(nil)

	status = a.GetStatus()
	assert.NotNil(t, status)
	assert.Equal(t, STATUS_OK_2, status.OK)
	assert.Equal(t, STATUS_OK_2, status.CorpStatus[CORPUS].OK)
	assert.Equal(t, UNTRIAGED_COUNT_2, status.CorpStatus[CORPUS].UntriagedCount)
	assert.Equal(t, NEGATIVE_COUNT_2, status.CorpStatus[CORPUS].NegativeCount)
}

func TestAgainstLiveData(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	// Download the testdata and remove the testdata directory at the end.
	err := testutils.DownloadTestDataFile(TEST_DATA_STORAGE_PATH, TEST_DATA_PATH)
	assert.Nil(t, err, "Unable to download testdata.")
	defer func() {
		os.RemoveAll(TEST_DATA_DIR)
	}()

	diffStore := NewMockDiffStore()
	expStore := expstorage.NewMemExpectationsStore()
	tileStore := NewMockTileStoreFromJson(t, TEST_DATA_PATH)
	ignoreStore := types.NewMemIgnoreStore()
	timeBetweenPolls := 10 * time.Hour
	a := NewAnalyzer(expStore, tileStore, diffStore, ignoreStore, mockUrlGenerator, filediffstore.MemCacheFactory, timeBetweenPolls)

	// Poll until the Analyzer has process the tile.
	var allTests *GUITestDetails
	allTests, err = a.ListTestDetails(nil)
	assert.Nil(t, err)
	for allTests == nil {
		time.Sleep(10 * time.Millisecond)
		allTests, err = a.ListTestDetails(nil)
		assert.Nil(t, err)
	}
	assert.NotNil(t, allTests)

	// Testdrive the json codec.
	codec := LabeledTileCodec(0)
	encTile, err := codec.Encode(a.current.Tile)
	assert.Nil(t, err)
	foundTile, err := codec.Decode(encTile)
	assert.Nil(t, err)
	assert.Equal(t, a.current.Tile, foundTile)

	// // // Query For 565
	allTests, err = a.ListTestDetails(map[string][]string{
		"config": []string{"565"},
	})
	assert.Nil(t, err)
	assert.True(t, len(allTests.Tests) > 0)
	for _, oneTestDetail := range allTests.Tests {
		for _, unt := range oneTestDetail.Untriaged {
			count, ok := unt.ParamCounts["config"]["565"]
			assert.True(t, ok)
			assert.True(t, count > 0)
			assert.Equal(t, 1, len(unt.ParamCounts["config"]))
		}
	}

	// Query within an individual tests.
	oneTest, err := a.GetTestDetails("blurcircles", map[string][]string{
		"config": []string{"565"},
	})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(oneTest.Tests))
	assert.Equal(t, "blurcircles", oneTest.Tests[0].Name)
	for _, unt := range oneTest.Tests[0].Untriaged {
		for key := range unt.ParamCounts["config"] {
			assert.Equal(t, "565", key)
		}
	}

	// Make sure the commits are consistent.
	assert.Equal(t, 1, len(oneTest.CommitsByDigest))
	for testName, commitsBD := range oneTest.CommitsByDigest {
		for _, commitIds := range commitsBD {
			assert.True(t, len(commitIds) > 0)
			assert.True(t, commitIds[0] < len(oneTest.Commits))
			assert.True(t, commitIds[0] >= 0)
			for i := 1; i < len(commitIds); i++ {
				assert.True(t, commitIds[i-1] < commitIds[i], fmt.Sprintf("%d is not smaller than %d", commitIds[i-1], commitIds[i]))
				assert.True(t, commitIds[i] < len(oneTest.Commits))
				assert.True(t, commitIds[i] >= 0)
			}
		}

		// Go through the current tile and check whether the digests match up.
		for _, trace := range a.current.Tile.Traces[testName] {
			for idx, digest := range trace.Digests {
				assert.NotNil(t, commitsBD[digest], fmt.Sprintf("Did not find digest: %s in \n %v", digest, commitsBD))
				assert.NotEqual(t, len(commitsBD[digest]), sort.SearchInts(commitsBD[digest], trace.CommitIds[idx]))
			}
		}
	}

	// Get the status.
	status := a.GetStatus()

	// Query over all corpora.
	allTests, err = a.ListTestDetails(nil)
	assert.Nil(t, err)
	assert.NotNil(t, allTests.AllParams)

	// Query each corpus individually and make sure the results make sense.
	testCount := 0
	testParams := map[string][]string{}
	for corpus, _ := range status.CorpStatus {
		q := map[string][]string{"source_type": []string{corpus}}
		corpusTests, err := a.ListTestDetails(q)
		q["head"] = []string{"1"}
		assert.Nil(t, err)
		assert.Equal(t, q, corpusTests.Query)
		assert.NotEqual(t, allTests.AllParams, corpusTests.AllParams)

		testCount += len(corpusTests.Tests)
		addParams(testParams, corpusTests.AllParams)
	}
	assert.Equal(t, len(allTests.Tests), testCount)
	assert.Equal(t, len(allTests.AllParams), len(testParams))
	for param, values := range allTests.AllParams {
		assert.NotNil(t, testParams[param])
		sort.Strings(values)
		sort.Strings(testParams[param])
		assert.Equal(t, values, testParams[param])
	}
	assert.Equal(t, allTests.AllParams, testParams)

	// Query for head and assert that status and the query agree.
	for corpus, corpStatus := range status.CorpStatus {
		headTests, err := a.ListTestDetails(map[string][]string{QUERY_HEAD: []string{"1"}, "source_type": []string{corpus}})
		assert.Nil(t, err)

		foundUntriaged := map[string]bool{}
		for _, oneTest := range headTests.Tests {
			for d := range oneTest.Untriaged {
				foundUntriaged[d] = true
			}
		}
		assert.Equal(t, corpStatus.UntriagedCount, len(foundUntriaged))
	}
}

func addParams(current map[string][]string, additional map[string][]string) {
	for param := range additional {
		current[param] = util.UnionStrings(current[param], additional[param])
	}
}

func findTest(t *testing.T, tDetails *GUITestDetails, testName string) *GUITestDetail {
	for _, td := range tDetails.Tests {
		if td.Name == testName {
			return td
		}
	}
	assert.FailNow(t, "Unable to find test: "+testName)
	return nil
}

// Mock the url generator function.
func mockUrlGenerator(path string) string {
	return path
}

// Mock the diffstore.
type MockDiffStore struct{}

func (m MockDiffStore) Get(dMain string, dRest []string) (map[string]*diff.DiffMetrics, error) {
	result := map[string]*diff.DiffMetrics{}
	for _, d := range dRest {
		result[d] = &diff.DiffMetrics{
			NumDiffPixels:     10,
			PixelDiffPercent:  1.0,
			PixelDiffFilePath: fmt.Sprintf("diffpath/%s-%s", dMain, d),
			MaxRGBADiffs:      []int{5, 3, 4, 0},
			DimDiffer:         false,
		}
	}
	return result, nil
}

func (m MockDiffStore) AbsPath(digest []string) map[string]string {
	result := map[string]string{}
	for _, d := range digest {
		result[d] = "abspath/" + d
	}
	return result
}

func (m MockDiffStore) ThumbAbsPath(digest []string) map[string]string {
	result := map[string]string{}
	for _, d := range digest {
		result[d] = "thumb/abspath/" + d
	}
	return result
}

func (m MockDiffStore) UnavailableDigests() map[string]bool {
	return nil
}

func (m MockDiffStore) CalculateDiffs([]string) {}

func NewMockDiffStore() diff.DiffStore {
	return MockDiffStore{}
}

// Mock the tilestore for GoldenTraces
func NewMockTileStore(t *testing.T, digests [][]string, params []map[string]string, commits []*ptypes.Commit) ptypes.TileStore {
	// Build the tile from the digests, params and commits.
	traces := map[string]ptypes.Trace{}

	for idx, traceDigests := range digests {
		traceParts := []string{}
		for _, v := range params[idx] {
			traceParts = append(traceParts, v)
		}
		sort.Strings(traceParts)

		traces[strings.Join(traceParts, ":")] = &ptypes.GoldenTrace{
			Params_: params[idx],
			Values:  traceDigests,
		}
	}

	tile := ptypes.NewTile()
	tile.Traces = traces
	tile.Commits = commits

	return &MockTileStore{
		t:    t,
		tile: tile,
	}
}

// NewMockTileStoreFromJson reads a tile that has been serialized to JSON
// and wraps an instance of MockTileStore around it.
func NewMockTileStoreFromJson(t *testing.T, fname string) ptypes.TileStore {
	f, err := os.Open(fname)
	assert.Nil(t, err)

	tile, err := ptypes.TileFromJson(f, &ptypes.GoldenTrace{})
	assert.Nil(t, err)

	return &MockTileStore{
		t:    t,
		tile: tile,
	}
}

type MockTileStore struct {
	t    *testing.T
	tile *ptypes.Tile
}

func (m *MockTileStore) Get(scale, index int) (*ptypes.Tile, error) {
	return m.tile, nil
}

func (m *MockTileStore) Put(scale, index int, tile *ptypes.Tile) error {
	assert.FailNow(m.t, "Should not be called.")
	return nil
}

func (m *MockTileStore) GetModifiable(scale, index int) (*ptypes.Tile, error) {
	return m.Get(scale, index)
}
