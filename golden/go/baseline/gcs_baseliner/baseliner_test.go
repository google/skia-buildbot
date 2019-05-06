package gcs_baseliner

import (
	"testing"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	testdata "go.skia.org/infra/golden/go/testutils"
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

	mgs.On("ReadBaseline", testCommitHash, testIssueID).Return(testdata.MakeTestBaselineThreeDevice(), nil).Once()

	baseliner, err := New(mgs, nil, nil, nil, nil)
	assert.NoError(t, err)

	b, err := baseliner.FetchBaseline(testCommitHash, testIssueID, testPatchSetID, false)
	assert.NoError(t, err)

	deepequal.AssertDeepEqual(t, testdata.MakeTestBaselineThreeDevice(), b)
}

func TestFetchBaselineCachingSunnyDay(t *testing.T) {
	testutils.SmallTest(t)

	testCommitHash := "abcd12345"
	testIssueID := int64(0)
	testPatchSetID := int64(0)

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	// ReadBaseline should only be called once despite multiple requests below
	mgs.On("ReadBaseline", testCommitHash, testIssueID).Return(testdata.MakeTestBaselineThreeDevice(), nil).Once()

	baseliner, err := New(mgs, nil, nil, nil, nil)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		b, err := baseliner.FetchBaseline(testCommitHash, testIssueID, testPatchSetID, false)
		assert.NoError(t, err)
		assert.NotNil(t, b)
		deepequal.AssertDeepEqual(t, testdata.MakeTestBaselineThreeDevice(), b)
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

	mcs.On("AllCommits").Return(testdata.MakeTestCommitsThreeDevice())
	mcs.On("DataCommits").Return(testdata.MakeTestCommitsThreeDevice())
	mcs.On("GetTile", false).Return(testdata.MakeTestTileThreeDevice())

	mes.On("Get").Return(meh, nil)

	meh.On("Classification", "test_alpha", testdata.AlphaGood1Hash).Return(types.POSITIVE)
	meh.On("Classification", "test_alpha", testdata.AlphaUntriaged1Hash).Return(types.UNTRIAGED)
	meh.On("Classification", "test_alpha", testdata.AlphaBad1Hash).Return(types.NEGATIVE)
	meh.On("Classification", "test_beta", testdata.BetaGood1Hash).Return(types.POSITIVE)
	meh.On("Classification", "test_beta", testdata.BetaUntriaged1Hash).Return(types.UNTRIAGED)

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
		case testdata.FirstCommitHash:
			assert.Equal(t, 1, b.Filled)
			assertLabel(t, b, "test_beta", testdata.BetaGood1Hash, types.POSITIVE)
		case testdata.SecondCommitHash:
			assert.Equal(t, 1, b.Filled)
			assertLabel(t, b, "test_beta", testdata.BetaGood1Hash, types.POSITIVE)
		case testdata.ThirdCommitHash:
			assert.Equal(t, 2, b.Filled)
			assertLabel(t, b, "test_alpha", testdata.AlphaGood1Hash, types.POSITIVE)
			assertLabel(t, b, "test_beta", testdata.BetaGood1Hash, types.POSITIVE)
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
