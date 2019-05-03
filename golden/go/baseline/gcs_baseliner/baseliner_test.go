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

// Test that the baseliner passes on the request to the storage.GCSClient
// for a baseline it hasn't seen before
func TestFetchBaselineSunnyDay(t *testing.T) {
	testutils.SmallTest(t)

	testCommitHash := "abcd12345"
	testIssueID := int64(0)
	testPatchSetID := int64(0)

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	mgs.On("ReadBaseline", testCommitHash, testIssueID).Return(makeTestBaseline(), nil).Once()

	baseliner, err := New(mgs, nil, nil, nil, nil)
	assert.NoError(t, err)

	b, err := baseliner.FetchBaseline(testCommitHash, testIssueID, testPatchSetID, false)
	assert.NoError(t, err)

	deepequal.AssertDeepEqual(t, makeTestBaseline(), b)
}

func TestFetchBaselineCachingSunnyDay(t *testing.T) {
	testutils.SmallTest(t)

	testCommitHash := "abcd12345"
	testIssueID := int64(0)
	testPatchSetID := int64(0)

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	// ReadBaseline should only be called once despite multiple requests below
	mgs.On("ReadBaseline", testCommitHash, testIssueID).Return(makeTestBaseline(), nil).Once()

	baseliner, err := New(mgs, nil, nil, nil, nil)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		b, err := baseliner.FetchBaseline(testCommitHash, testIssueID, testPatchSetID, false)
		assert.NoError(t, err)
		assert.NotNil(t, b)
		deepequal.AssertDeepEqual(t, makeTestBaseline(), b)
	}
}

// TODO(kjlubick): TestFetchBaseline which returns nil (difficult because it shells out to
// PushMasterBaseline)

// TestPushMasterBaselineSunnyDay verifies that the baseliner pulls in the latest tile
// (which has the traces and known images) and combines it with the triage status from
// ExpectationsStorage to create a baseline per commit.
func TestPushMasterBaselineSunnyDay(t *testing.T) {
	testutils.SmallTest(t)

	mgs := makeMockGCSStorage()
	mcs := &mocks.TileInfo{}
	mes := &mocks.ExpectationsStore{}
	meh := &mocks.TestExpBuilder{}

	defer mgs.AssertExpectations(t)
	defer mcs.AssertExpectations(t)
	defer mes.AssertExpectations(t)
	defer meh.AssertExpectations(t)

	mcs.On("AllCommits").Return(makeTestCommits())
	mcs.On("DataCommits").Return(makeTestCommits())
	mcs.On("GetTile", false).Return(makeTestTile())

	mes.On("Get").Return(meh, nil)

	meh.On("Classification", "test_alpha", alphaGood1Hash).Return(types.POSITIVE)
	meh.On("Classification", "test_alpha", alphaUntriaged1Hash).Return(types.UNTRIAGED)
	meh.On("Classification", "test_alpha", alphaBad1Hash).Return(types.NEGATIVE)
	meh.On("Classification", "test_beta", betaGood1Hash).Return(types.POSITIVE)
	meh.On("Classification", "test_beta", betaUntriaged1Hash).Return(types.UNTRIAGED)

	mgs.On("WriteBaseline", mock.AnythingOfType("*baseline.CommitableBaseline")).Run(func(args mock.Arguments) {
		b := args.Get(0).(*baseline.CommitableBaseline)
		assert.NotNil(t, b)
		assert.NotNil(t, b.StartCommit)
		// These commits are per-commit baselines, thus the start and end are the same
		deepequal.AssertDeepEqual(t, *b.StartCommit, *b.EndCommit)

		assert.Equal(t, 6, b.Total)

		// These per-commit baselines keep track of only the positive images we have seen,
		// so make sure we see betaGood1Hash for all 3 and alphaGood1Hash show up in the
		// third commit
		switch b.StartCommit.Hash {
		default:
			assert.Fail(t, "Bad hash", b.StartCommit.Hash)
		case firstCommitHash:
			assert.Equal(t, 1, b.Filled)
			assertLabel(t, b, "test_beta", betaGood1Hash, types.POSITIVE)
		case secondCommitHash:
			assert.Equal(t, 1, b.Filled)
			assertLabel(t, b, "test_beta", betaGood1Hash, types.POSITIVE)
		case thirdCommitHash:
			assert.Equal(t, 2, b.Filled)
			assertLabel(t, b, "test_alpha", alphaGood1Hash, types.POSITIVE)
			assertLabel(t, b, "test_beta", betaGood1Hash, types.POSITIVE)
		}
	}).Return("gs://test-bucket/baselines/foo-baseline.json", nil).Times(3) // once per commit

	baseliner, err := New(mgs, mes, nil, nil, nil)
	assert.NoError(t, err)

	b, err := baseliner.PushMasterBaselines(mcs, "")
	assert.NoError(t, err)
	assert.Nil(t, b) // baseline should be nil because commit is ""
}

func makeMockGCSStorage() *mocks.GCSClient {
	mgs := mocks.GCSClient{}
	mgs.On("Options").Return(storage.GCSClientOptions{
		HashesGSPath:   "gs://test-bucket/hashes",
		BaselineGSPath: "gs://test-bucket/baselines/",
	}).Maybe()
	return &mgs
}

func assertLabel(t *testing.T, b *baseline.CommitableBaseline, testName, hash string, label types.Label) {
	test, ok := b.Baseline[testName]
	if !ok {
		assert.Failf(t, "assertLabel", "Could not find test %s in baseline %#v", testName, b)
	}

	lab, ok := test[hash]
	if !ok {
		assert.Failf(t, "assertLabel", "Could not find hash %s in test %#v (name %s) (baseline %#v)", hash, test, testName, b)
	}
	assert.Equal(t, label, lab)
}

// This baseline represents the following case: There are 3 devices
// (angler, bullhead, crosshatch, each running 2 tests (test_alpha, test_beta)
//
// All 3 devices drew test_alpha incorrectly as digest alphaBad1Hash at StartCommit.
// Devices angler and crosshatch drew test_alpha correctly as digest alphaGood1Hash at EndCommit.
// Device bullhead drew test_alpha as digest alphaUntriaged1Hash at EndCommit.
//
// Devices angler and bullhead drew test_beta the same (digest betaGood1Hash)
// and device crosshatch the remaining case betaUntriaged1Hash.
// crosshatch is missing two digests (maybe that test hasn't run yet?)
// The baseline is on the master branch.
//
// These helper functions all return a fresh copy of their objects so that
// tests can mutate them w/o impacting future tests.
func makeTestBaseline() *baseline.CommitableBaseline {
	return &baseline.CommitableBaseline{
		StartCommit: &tiling.Commit{
			Hash:       firstCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     "alpha@example.com",
		},
		EndCommit: &tiling.Commit{
			Hash:       thirdCommitHash,
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
		Filled: 2, // two tests had at least one positive digest
		Total:  6,
		Issue:  0, // 0 means master branch, by definition
	}
}

func makeTestCommits() []*tiling.Commit {
	// Three commits, with completely arbitrary data
	return []*tiling.Commit{
		{
			Hash:       firstCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 0, 3, 0, time.UTC).Unix(),
			Author:     "alpha@example.com",
		},
		{
			Hash:       secondCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 12, 10, 18, 0, time.UTC).Unix(),
			Author:     "beta@example.com",
		},
		{
			Hash:       thirdCommitHash,
			CommitTime: time.Date(2019, time.April, 26, 13, 10, 8, 0, time.UTC).Unix(),
			Author:     "gamma@example.com",
		},
	}
}

func makeTestTile() *tiling.Tile {
	return &tiling.Tile{
		Commits:   makeTestCommits(),
		Scale:     1,
		TileIndex: 0,

		Traces: map[string]tiling.Trace{
			// Reminder that the ids for the traces are created by concatenating
			// all the values in alphabetical order of the keys.
			"angler:test_alpha:gm": &types.GoldenTrace{
				Digests: []string{alphaBad1Hash, alphaBad1Hash, alphaGood1Hash},
				Keys: map[string]string{
					"device":                "angler",
					types.PRIMARY_KEY_FIELD: "test_alpha",
					types.CORPUS_FIELD:      "gm",
				},
			},
			"angler:test_beta:gm": &types.GoldenTrace{
				Digests: []string{betaGood1Hash, betaGood1Hash, betaGood1Hash},
				Keys: map[string]string{
					"device":                "angler",
					types.PRIMARY_KEY_FIELD: "test_beta",
					types.CORPUS_FIELD:      "gm",
				},
			},

			"bullhead:test_alpha:gm": &types.GoldenTrace{
				Digests: []string{alphaBad1Hash, alphaBad1Hash, alphaUntriaged1Hash},
				Keys: map[string]string{
					"device":                "bullhead",
					types.PRIMARY_KEY_FIELD: "test_alpha",
					types.CORPUS_FIELD:      "gm",
				},
			},
			"bullhead:test_beta:gm": &types.GoldenTrace{
				Digests: []string{betaGood1Hash, betaGood1Hash, betaGood1Hash},
				Keys: map[string]string{
					"device":                "bullhead",
					types.PRIMARY_KEY_FIELD: "test_beta",
					types.CORPUS_FIELD:      "gm",
				},
			},

			"crosshatch:test_alpha:gm": &types.GoldenTrace{
				Digests: []string{alphaBad1Hash, alphaBad1Hash, alphaGood1Hash},
				Keys: map[string]string{
					"device":                "crosshatch",
					types.PRIMARY_KEY_FIELD: "test_alpha",
					types.CORPUS_FIELD:      "gm",
				},
			},
			"crosshatch:test_beta:gm": &types.GoldenTrace{
				Digests: []string{betaUntriaged1Hash, types.MISSING_DIGEST, types.MISSING_DIGEST},
				Keys: map[string]string{
					"device":                "crosshatch",
					types.PRIMARY_KEY_FIELD: "test_beta",
					types.CORPUS_FIELD:      "gm",
				},
			},
		},
	}
}

// human-readable variable names for the hashes (values are arbitrary, but valid md5 hashes)
const (
	alphaGood1Hash      = "0cc175b9c0f1b6a831c399e269772661"
	alphaBad1Hash       = "92eb5ffee6ae2fec3ad71c777531578f"
	alphaUntriaged1Hash = "4a8a08f09d37b73795649038408b5f33"

	betaGood1Hash      = "7277e0910d750195b448797616e091ad"
	betaUntriaged1Hash = "8fa14cdd754f91cc6554c9e71929cce7"

	firstCommitHash  = "a3f82d283f72b5d51ecada8ec56ec8ff4aa81c6c"
	secondCommitHash = "b52f7829a2384b001cc12b0c2613c756454a1f6a"
	thirdCommitHash  = "cd77adf52094181356d60845ee5cf1d83aec6d2a"
)
