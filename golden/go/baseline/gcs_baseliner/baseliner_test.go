package gcs_baseliner

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	"go.skia.org/infra/golden/go/types"
)

// Test that the baseliner passes on the request to the storage.GCStorageClient
// for a baseline it hasn't seen before
func TestFetchBaselineSunnyDay(t *testing.T) {
	testutils.SmallTest(t)

	testCommit := "abcd12345"
	testIssueID := int64(0)
	testPatchSetID := int64(0)

	ms := makeMockGCSStorage()
	defer ms.AssertExpectations(t)

	ms.On("ReadBaseline", testCommit, testIssueID).Return(&testCommitableBaseline, nil).Once()

	b, err := New(&ms, nil, nil, nil, nil)
	assert.NoError(t, err)

	baseline, err := b.FetchBaseline(testCommit, testIssueID, testPatchSetID, false)
	assert.NoError(t, err)

	deepequal.AssertDeepEqual(t, &testCommitableBaseline, baseline)
}

func TestFetchBaselineCachingSunnyDay(t *testing.T) {
	testutils.SmallTest(t)

	testCommit := "abcd12345"
	testIssueID := int64(0)
	testPatchSetID := int64(0)

	ms := makeMockGCSStorage()
	defer ms.AssertExpectations(t)

	// ReadBaseline should only be called once despite multiple requests below
	ms.On("ReadBaseline", testCommit, testIssueID).Return(&testCommitableBaseline, nil).Once()

	b, err := New(&ms, nil, nil, nil, nil)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		baseline, err := b.FetchBaseline(testCommit, testIssueID, testPatchSetID, false)
		assert.NoError(t, err)
		assert.NotNil(t, baseline)
		deepequal.AssertDeepEqual(t, &testCommitableBaseline, baseline)
	}
}

func makeMockGCSStorage() mocks.GCStorageClient {
	ms := mocks.GCStorageClient{}
	ms.On("Options").Return(storage.GCSClientOptions{
		HashesGSPath:   "gs://test-bucket/hashes",
		BaselineGSPath: "gs://test-bucket/baselines/",
	}).Maybe()
	return ms
}

// This baseline represents the following case: There are 4 devices, each running
// 2 tests. Device #4 hasn't finished yet, so isn't in the baseline. Devices 1-3
// all drew testAlpha slightly differently. Devices 1 and 2 drew testBeta the same
// (perhaps the digest starting with 727) and Device 3 drew the remaining case.
// The baseline is on the master branch.
var testCommitableBaseline = baseline.CommitableBaseLine{
	StartCommit: &tiling.Commit{
		Hash:       "beginningHash",
		CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
		Author:     "alpha@example.com",
	},
	EndCommit: &tiling.Commit{
		Hash:       "endingHash",
		CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
		Author:     "beta@example.com",
	},
	MD5: "hashOfBaseline",
	Baseline: types.TestExp{
		"testAlpha": map[string]types.Label{
			// These hashes are arbitrarily made up and have no real-world meaning.
			"0cc175b9c0f1b6a831c399e269772661": types.POSITIVE,
			"4a8a08f09d37b73795649038408b5f33": types.UNTRIAGED,
			"92eb5ffee6ae2fec3ad71c777531578f": types.NEGATIVE,
		},
		"testBeta": map[string]types.Label{
			// These hashes are arbitrarily made up and have no real-world meaning.
			"7277e0910d750195b448797616e091ad": types.POSITIVE,
			"8fa14cdd754f91cc6554c9e71929cce7": types.UNTRIAGED,
		},
	},
	Filled: 6,
	Total:  8,
	Issue:  0, // 0 means master branch, by definition
}
