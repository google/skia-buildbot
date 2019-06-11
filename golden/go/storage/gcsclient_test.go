package storage

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/types"
)

const (
	// TEST_HASHES_GS_PATH is the bucket/path combination where the test file will be written.
	TEST_HASHES_GS_PATH = "skia-infra-testdata/hash_files/testing-known-hashes.txt"

	// TEST_BASELINE_GS_PATH is the root path of all baseline file in GCS.
	TEST_BASELINE_GS_PATH = "skia-infra-testdata/hash_files/testing-baselines"

	// Directory with testdata.
	TEST_DATA_DIR = "./testdata"

	// Local file location of the test data.
	TEST_DATA_PATH = TEST_DATA_DIR + "/10-test-sample-4bytes.tile"

	// Folder in the testdata bucket. See go/testutils for details.
	TEST_DATA_STORAGE_PATH = "gold-testdata/10-test-sample-4bytes.tile"
)

var (
	issueID = int64(5678)

	startCommit = &tiling.Commit{
		CommitTime: time.Now().Add(-time.Hour * 10).Unix(),
		Hash:       "abb84b151a49eca5a6e107c51a1f1b7da73454bf",
		Author:     "Jon Doe",
	}
	endCommit = &tiling.Commit{
		CommitTime: time.Now().Unix(),
		Hash:       "51465f0ed60ce2cacff3653c7d1d70317679fc06",
		Author:     "Jane Doe",
	}

	masterBaseline = &baseline.Baseline{
		StartCommit: startCommit,
		EndCommit:   endCommit,
		Expectations: types.Expectations{
			"test-1": map[types.Digest]types.Label{"d1": types.POSITIVE},
		},
		Issue: 0,
	}

	issueBaseline = &baseline.Baseline{
		StartCommit: endCommit,
		EndCommit:   endCommit,
		Expectations: types.Expectations{
			"test-3": map[types.Digest]types.Label{"d2": types.POSITIVE},
		},
		Issue: issueID,
	}
)

func TestWritingHashes(t *testing.T) {
	unittest.LargeTest(t)
	gsClient, opt := initGSClient(t)

	knownDigests := types.DigestSlice{
		"c003788f8d306ff1226e2a460835dae4",
		"885b31941c25efc313b0fd66d55b86d9",
		"264d0d87b12ba337f796fc592cd5357d",
		"69c2fbf8e89a48058b2f45ad4ea46a35",
		"2c4d605c16e7d5b23294c0433fa3ed17",
		"782717cf6ed9329fc43cb5a6c830cbce",
		"e143ca619f2172d06bb0dcc4d72af414",
		"26aff0619c829bc149f7c0171fcca442",
		"72d61ae8e232c3a279cc3cdbf6ef73e5",
		"f1eb049dac1cfa3c70aac8fc6ad5496f",
	}
	assert.NoError(t, gsClient.WriteKnownDigests(knownDigests))
	removePaths := []string{opt.HashesGSPath}
	defer func() {
		for _, path := range removePaths {
			_ = gsClient.RemoveForTestingOnly(path)
		}
	}()

	found := loadKnownHashes(t, gsClient)
	assert.Equal(t, knownDigests, found)
}

func TestWritingBaselines(t *testing.T) {
	unittest.LargeTest(t)

	gsClient, _ := initGSClient(t)
	removePaths := []string{}
	defer func() {
		for _, path := range removePaths {
			_ = gsClient.RemoveForTestingOnly(path)
		}
	}()

	path, err := gsClient.WriteBaseline(masterBaseline)
	assert.NoError(t, err)
	removePaths = append(removePaths, strings.TrimPrefix(path, "gs://"))

	foundBaseline, err := gsClient.ReadBaseline(endCommit.Hash, 0)
	assert.NoError(t, err)
	assert.Equal(t, masterBaseline, foundBaseline)

	// Add a baseline for an issue
	path, err = gsClient.WriteBaseline(issueBaseline)
	assert.NoError(t, err)
	removePaths = append(removePaths, strings.TrimPrefix(path, "gs://"))

	foundBaseline, err = gsClient.ReadBaseline("", issueID)
	assert.NoError(t, err)
	assert.Equal(t, issueBaseline, foundBaseline)
}

func TestBaselineRobustness(t *testing.T) {
	unittest.LargeTest(t)

	gsClient, _ := initGSClient(t)

	removePaths := []string{}
	defer func() {
		for _, path := range removePaths {
			_ = gsClient.RemoveForTestingOnly(path)
		}
	}()

	// Read the master baseline that has not been written
	foundBaseline, err := gsClient.ReadBaseline("", 5344)
	assert.NoError(t, err)
	assert.Nil(t, foundBaseline)

	// Test reading a non-existing baseline for an issue
	foundBaseline, err = gsClient.ReadBaseline("", 5344)
	assert.NoError(t, err)
	assert.Nil(t, foundBaseline)

	path, err := gsClient.WriteBaseline(masterBaseline)
	assert.NoError(t, err)
	removePaths = append(removePaths, strings.TrimPrefix(path, "gs://"))
}

func initGSClient(t *testing.T) (GCSClient, GCSClientOptions) {
	timeStamp := fmt.Sprintf("%032d", time.Now().UnixNano())
	opt := GCSClientOptions{
		HashesGSPath:   TEST_HASHES_GS_PATH + "-" + timeStamp,
		BaselineGSPath: TEST_BASELINE_GS_PATH + "-" + timeStamp,
	}
	gsClient, err := NewGCSClient(nil, opt)
	assert.NoError(t, err)
	return gsClient, opt
}

func loadKnownHashes(t *testing.T, gsClient GCSClient) types.DigestSlice {
	var buf bytes.Buffer
	assert.NoError(t, gsClient.LoadKnownDigests(&buf))

	scanner := bufio.NewScanner(&buf)
	ret := types.DigestSlice{}
	for scanner.Scan() {
		ret = append(ret, types.Digest(scanner.Text()))
	}
	return ret
}
