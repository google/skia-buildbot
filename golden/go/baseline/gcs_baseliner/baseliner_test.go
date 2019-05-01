package gcs_baseliner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
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

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	mgs.On("ReadBaseline", testCommit, testIssueID).Return(&testCommitableBaseline, nil).Once()

	b, err := New(mgs, nil, nil, nil, nil)
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

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	// ReadBaseline should only be called once despite multiple requests below
	mgs.On("ReadBaseline", testCommit, testIssueID).Return(&testCommitableBaseline, nil).Once()

	b, err := New(mgs, nil, nil, nil, nil)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		baseline, err := b.FetchBaseline(testCommit, testIssueID, testPatchSetID, false)
		assert.NoError(t, err)
		assert.NotNil(t, baseline)
		deepequal.AssertDeepEqual(t, &testCommitableBaseline, baseline)
	}
}

// TODO(kjlubick): TestFetchBaseline which returns nil (difficult because it shells out to PushMasterBaseline)

// TestPushMasterBaselineSunnyDay verifies that the baseliner pulls in the latest tile
// (which has the traces and known images) and combines it with the triage status from
// ExpectationsStorage to create a baseline. This baseline should then be pushed
// to GCS.
func TestPushMasterBaselineSunnyDay(t *testing.T) {
	testutils.SmallTest(t)

	mgs := makeMockGCSStorage()
	mcs := &mocks.CommitSource{}
	mes := &mocks.ExpectationsStore{}
	meh := &mocks.ExpectationsHandler{}

	defer mgs.AssertExpectations(t)
	defer mcs.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer meh.AssertExpectations(t)

	mcs.On("AllCommits").Return(testCommits)
	mcs.On("DataCommits").Return(testCommits)
	mcs.On("GetTile", false).Return(&testTile)

	mes.On("Get").Return(meh, nil)

	meh.On("Classification", "test_alpha", alphaGood1Hash).Return(types.POSITIVE)
	meh.On("Classification", "test_alpha", alphaUntriaged1Hash).Return(types.UNTRIAGED)
	meh.On("Classification", "test_alpha", alphaBad1Hash).Return(types.NEGATIVE)
	meh.On("Classification", "test_beta", betaGood1Hash).Return(types.POSITIVE)
	meh.On("Classification", "test_beta", betaUntriaged1Hash).Return(types.UNTRIAGED)

	mgs.On("WriteBaseLine", mock.AnythingOfType("*baseline.CommitableBaseLine")).Run(func(args mock.Arguments) {
		baseline := args.Get(0).(*baseline.CommitableBaseLine)
		assert.NotNil(t, baseline)
		deepequal.AssertDeepEqual(t, &testCommitableBaseline, baseline)
	}).Return("gs://test-bucket/baselines/foo-baseline.json", nil)

	b, err := New(mgs, mes, nil, nil, nil)
	assert.NoError(t, err)

	baseline, err := b.PushMasterBaselines(mcs, "")
	assert.NoError(t, err)
	assert.Nil(t, baseline) // baseline should be nil because commit is ""
}

func makeMockGCSStorage() *mocks.GCStorageClient {
	mgs := mocks.GCStorageClient{}
	mgs.On("Options").Return(storage.GCSClientOptions{
		HashesGSPath:   "gs://test-bucket/hashes",
		BaselineGSPath: "gs://test-bucket/baselines/",
	}).Maybe()
	return &mgs
}

// This baseline represents the following case: There are 3 devices
// (angler, bullhead, crosshatch, each running 2 tests (test_alpha, test_beta)
//
// All 3 devices drew test_alpha incorrectly as digest 92eb at StartCommit.
// Devices angler and crosshatch drew test_alpha correctly as digest 0cc1 at EndCommit.
// Device bullhead drew test_alpha as digest 4a8a at EndCommit which still needs to be triaged.
//
// Devices angler and bullhead drew test_beta the same (digest 7277)
// and device crosshatch the remaining case. crosshatch is missing two digests (maybe
// that test hasn't run yet?)
// The baseline is on the master branch.
var testCommitableBaseline = baseline.CommitableBaseLine{
	StartCommit: &tiling.Commit{
		Hash:       "a3f82d283f72b5d51ecada8ec56ec8ff4aa81c6c",
		CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
		Author:     "alpha@example.com",
	},
	EndCommit: &tiling.Commit{
		Hash:       "cd77adf52094181356d60845ee5cf1d83aec6d2a",
		CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
		Author:     "gamma@example.com",
	},
	MD5: "hashOfBaseline",
	Baseline: types.TestExp{
		"test_alpha": map[string]types.Label{
			// These hashes are arbitrarily made up and have no real-world meaning.
			alphaGood1Hash:      types.POSITIVE,
			alphaUntriaged1Hash: types.UNTRIAGED,
			alphaBad1Hash:       types.NEGATIVE,
		},
		"test_beta": map[string]types.Label{
			// These hashes are arbitrarily made up and have no real-world meaning.
			betaGood1Hash:      types.POSITIVE,
			betaUntriaged1Hash: types.UNTRIAGED,
		},
	},
	Filled: 6,
	Total:  8,
	Issue:  0, // 0 means master branch, by definition
}

// Three commits, with completely arbitrary data
var testCommits = []*tiling.Commit{
	{
		Hash:       "a3f82d283f72b5d51ecada8ec56ec8ff4aa81c6c",
		CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
		Author:     "alpha@example.com",
	},
	{
		Hash:       "b52f7829a2384b001cc12b0c2613c756454a1f6a",
		CommitTime: time.Date(2019, time.April, 26, 12, 10, 18, 0, time.UTC).Unix(),
		Author:     "beta@example.com",
	},
	{
		Hash:       "cd77adf52094181356d60845ee5cf1d83aec6d2a",
		CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
		Author:     "gamma@example.com",
	},
}

var testTile = tiling.Tile{
	Commits:   testCommits,
	Scale:     1,
	TileIndex: 0,

	Traces: map[string]tiling.Trace{
		// Reminder that the ids for the traces are created by concatenating
		// all the values in alphabetical order of the keys.
		"angler:test_alpha:gm": &types.GoldenTrace{
			Digests: []string{alphaBad1Hash, alphaBad1Hash, alphaGood1Hash},
			Keys: map[string]string{
				"device":           "angler",
				"name":             "test_alpha",
				types.CORPUS_FIELD: "gm",
			},
		},
		"angler:test_beta:gm": &types.GoldenTrace{
			Digests: []string{betaGood1Hash, betaGood1Hash, betaGood1Hash},
			Keys: map[string]string{
				"device":           "angler",
				"name":             "test_beta",
				types.CORPUS_FIELD: "gm",
			},
		},

		"bullhead:test_alpha:gm": &types.GoldenTrace{
			Digests: []string{alphaBad1Hash, alphaBad1Hash, alphaUntriaged1Hash},
			Keys: map[string]string{
				"device":           "bullhead",
				"name":             "test_alpha",
				types.CORPUS_FIELD: "gm",
			},
		},
		"bullhead:test_beta:gm": &types.GoldenTrace{
			Digests: []string{betaGood1Hash, betaGood1Hash, betaGood1Hash},
			Keys: map[string]string{
				"device":           "bullhead",
				"name":             "test_beta",
				types.CORPUS_FIELD: "gm",
			},
		},

		"crosshatch:test_alpha:gm": &types.GoldenTrace{
			Digests: []string{alphaBad1Hash, alphaBad1Hash, alphaGood1Hash},
			Keys: map[string]string{
				"device":           "crosshatch",
				"name":             "test_alpha",
				types.CORPUS_FIELD: "gm",
			},
		},
		"crosshatch:test_beta:gm": &types.GoldenTrace{
			Digests: []string{betaUntriaged1Hash, types.MISSING_DIGEST, types.MISSING_DIGEST},
			Keys: map[string]string{
				"device":           "crosshatch",
				"name":             "test_beta",
				types.CORPUS_FIELD: "gm",
			},
		},
	},
}

// human-readable variable names for the hashes (values are arbitrary, but valid md5 hashes)
var alphaGood1Hash = "0cc175b9c0f1b6a831c399e269772661"
var alphaBad1Hash = "92eb5ffee6ae2fec3ad71c777531578f"
var alphaUntriaged1Hash = "4a8a08f09d37b73795649038408b5f33"

var betaGood1Hash = "7277e0910d750195b448797616e091ad"
var betaUntriaged1Hash = "8fa14cdd754f91cc6554c9e71929cce7"
