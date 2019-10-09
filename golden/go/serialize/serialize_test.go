package serialize

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gcs/gcs_testutils"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/types/expectations"
)

const (
	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/goldentile.json"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_BUCKET = "skia-infra-testdata"
	TEST_DATA_STORAGE_PATH   = "gold-testdata/goldentile.json.gz"
)

var testParamsList = []paramtools.Params{
	map[string]string{
		"config":                "8888",
		types.CORPUS_FIELD:      "gmx",
		types.PRIMARY_KEY_FIELD: "foo1",
	},
	map[string]string{
		"config":                "565",
		types.CORPUS_FIELD:      "gmx",
		types.PRIMARY_KEY_FIELD: "foo1",
	},
	map[string]string{
		"config":                "gpu",
		types.CORPUS_FIELD:      "gm",
		types.PRIMARY_KEY_FIELD: "foo2",
	},
}

func TestSerializeStrings(t *testing.T) {
	unittest.SmallTest(t)
	testArr := []string{}
	for i := 0; i < 100; i++ {
		testArr = append(testArr, fmt.Sprintf("str-%4d", i))
	}

	bytesArr := stringsToBytes(testArr)
	found, err := bytesToStrings(bytesArr)
	require.NoError(t, err)
	require.Equal(t, testArr, found)

	var bufWriter bytes.Buffer
	require.NoError(t, writeStringArr(&bufWriter, testArr))

	found, err = readStringArr(bytes.NewBuffer(bufWriter.Bytes()))
	require.NoError(t, err)
	require.Equal(t, testArr, found)
}

func TestSerializeCommits(t *testing.T) {
	unittest.SmallTest(t)
	testCommits := []*tiling.Commit{
		{
			CommitTime: 42,
			Hash:       "ffffffffffffffffffffffffffffffffffffffff",
			Author:     "test@test.cz",
		},
		{
			CommitTime: 43,
			Hash:       "eeeeeeeeeee",
			Author:     "test@test.cz",
		},
		{
			CommitTime: 44,
			Hash:       "aaaaaaaaaaa",
			Author:     "test@test.cz",
		},
	}

	var buf bytes.Buffer
	require.NoError(t, writeCommits(&buf, testCommits))

	found, err := readCommits(bytes.NewBuffer(buf.Bytes()))
	require.NoError(t, err)
	require.Equal(t, testCommits, found)
}

func TestSerializeParamSets(t *testing.T) {
	unittest.SmallTest(t)
	testParamSet := paramtools.ParamSet(map[string][]string{})
	for _, p := range testParamsList {
		testParamSet.AddParams(p)
	}

	var buf bytes.Buffer
	keyToInt, valToInt, err := writeParamSets(&buf, testParamSet)
	require.NoError(t, err)

	intToKey, intToVal, err := readParamSets(bytes.NewBuffer(buf.Bytes()))
	require.NoError(t, err)

	require.Equal(t, len(testParamSet), len(keyToInt))
	require.Equal(t, len(keyToInt), len(intToKey))
	require.Equal(t, len(valToInt), len(intToVal))

	for key, idx := range keyToInt {
		require.Equal(t, key, intToKey[idx])
	}

	for val, idx := range valToInt {
		require.Equal(t, val, intToVal[idx])
	}

	for _, testParams := range testParamsList {
		byteArr := paramsToBytes(keyToInt, valToInt, testParams)
		found, err := bytesToParams(intToKey, intToVal, byteArr)
		require.NoError(t, err)
		require.Equal(t, testParams, found)
	}
}

func TestIntsToBytes(t *testing.T) {
	unittest.SmallTest(t)
	data := []int{20266, 20266, 20266, 20266}
	testBytes := intsToBytes(data)
	found, err := bytesToInts(testBytes)
	require.NoError(t, err)
	require.Equal(t, data, found)
}

func TestSerializeTile(t *testing.T) {
	unittest.LargeTest(t)
	tile, cleanupFn := getTestTile(t)
	defer cleanupFn()

	var buf bytes.Buffer
	digestToInt, err := writeDigests(&buf, tile.Traces)
	require.NoError(t, err)

	intToDigest, err := readDigests(bytes.NewBuffer(buf.Bytes()))
	require.NoError(t, err)
	require.Equal(t, len(digestToInt), len(intToDigest))

	for digest, idx := range digestToInt {
		require.Equal(t, digest, intToDigest[idx])
	}

	buf = bytes.Buffer{}
	require.NoError(t, SerializeTile(&buf, tile))

	foundTile, err := DeserializeTile(bytes.NewBuffer(buf.Bytes()))
	require.NoError(t, err)
	require.Equal(t, len(tile.Traces), len(foundTile.Traces))
	require.Equal(t, tile.Commits, foundTile.Commits)

	// NOTE(stephana): Not comparing ParamSet because it is inconsistent with
	// the parameters of the traces.

	for id, trace := range tile.Traces {
		foundTrace, ok := foundTile.Traces[id]
		require.True(t, ok)
		require.Equal(t, trace, foundTrace)
	}
}

func TestDeSerializeSample(t *testing.T) {
	unittest.LargeTest(t)
	tile, cleanupFn := getTestTile(t)
	defer cleanupFn()

	testExp := expectations.Expectations{
		"test-01": map[types.Digest]expectations.Label{"d_01": expectations.Positive, "d_02": expectations.Negative},
		"test-02": map[types.Digest]expectations.Label{"d_03": expectations.Untriaged, "d_04": expectations.Positive},
	}

	inOneHour := time.Now().Add(time.Hour).UTC()
	ignoreRules := []*ignore.Rule{
		ignore.NewRule("test-rule", inOneHour, "dev=true", "Some comment !"),
		ignore.NewRule("test-rule-2", inOneHour, "dev=false", "Another comment !"),
	}

	sample := &Sample{
		Tile:         tile,
		Expectations: testExp.DeepCopy(),
		IgnoreRules:  ignoreRules,
	}

	var buf bytes.Buffer
	require.NoError(t, sample.Serialize(&buf))

	foundSample, err := DeserializeSample(&buf)
	require.NoError(t, err)

	// Tile (de)serialization is tested above.
	require.Equal(t, sample.IgnoreRules, foundSample.IgnoreRules)
	require.Equal(t, sample.Expectations, foundSample.Expectations)
}

func getTestTile(t *testing.T) (*tiling.Tile, func()) {
	testDataDir := TEST_DATA_DIR
	testutils.RemoveAll(t, testDataDir)
	require.NoError(t, gcs_testutils.DownloadTestDataFile(t, TEST_DATA_STORAGE_BUCKET, TEST_DATA_STORAGE_PATH, TEST_DATA_PATH))

	f, err := os.Open(TEST_DATA_PATH)
	require.NoError(t, err)

	tile, err := types.TileFromJson(f, &types.GoldenTrace{})
	require.NoError(t, err)

	return tile, func() {
		defer testutils.RemoveAll(t, testDataDir)
	}
}
