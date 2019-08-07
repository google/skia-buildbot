package gcs_baseliner

import (
	"testing"

	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/baseline"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/storage"
	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

// Test that the baseliner passes on the request to the storage.GCSClient
// for a baseline it hasn't seen before
func TestFetchBaselineSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	testCommitHash := "abcd12345"

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	mgs.On("ReadBaseline", testCommitHash, types.MasterBranch).Return(three_devices.MakeTestBaseline(), nil).Once()

	baseliner, err := New(mgs, nil, nil, nil)
	assert.NoError(t, err)

	b, err := baseliner.FetchBaseline(testCommitHash, types.MasterBranch, false)
	assert.NoError(t, err)

	deepequal.AssertDeepEqual(t, three_devices.MakeTestBaseline(), b)
}

// Test that the baseliner behaves differently when requesting a baseline
// for a given tryjob.
func TestFetchBaselineIssueSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	testCommitHash := "abcd12345"
	testIssueID := int64(1234)

	// These are valid, but arbitrary md5 hashes
	IotaNewDigest := types.Digest("1115fba4ce5b4cb9ffd595beb63e7389")
	KappaNewDigest := types.Digest("222d894f5b680a9f7bd74c8004b7d88d")
	LambdaNewDigest := types.Digest("3333fe3127b984e4ff39f4885ddb0d98")

	additionalTriages := &baseline.Baseline{
		Expectations: types.Expectations{
			"brand-new-test": map[types.Digest]types.Label{
				IotaNewDigest:  types.POSITIVE,
				KappaNewDigest: types.NEGATIVE,
			},
			three_devices.BetaTest: map[types.Digest]types.Label{
				LambdaNewDigest: types.POSITIVE,
				// Change these two pre-existing digests
				three_devices.BetaGood1Digest:      types.NEGATIVE,
				three_devices.BetaUntriaged1Digest: types.POSITIVE,
			},
		},
		Issue: testIssueID,
	}

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	// mock the master baseline
	mgs.On("ReadBaseline", testCommitHash, types.MasterBranch).Return(three_devices.MakeTestBaseline(), nil).Once()
	// mock the expectations that a user would have applied to their CL (that
	// are not live on master yet).
	mgs.On("ReadBaseline", "", testIssueID).Return(additionalTriages, nil).Once()

	baseliner, err := New(mgs, nil, nil, nil)
	assert.NoError(t, err)

	b, err := baseliner.FetchBaseline(testCommitHash, testIssueID, false)
	assert.NoError(t, err)

	assert.Equal(t, testIssueID, b.Issue)
	// The expectation should be the master baseline merged in with the additionalTriages
	// with additionalTriages overwriting existing expectations, if applicable.
	assert.Equal(t, types.Expectations{
		"brand-new-test": map[types.Digest]types.Label{
			IotaNewDigest:  types.POSITIVE,
			KappaNewDigest: types.NEGATIVE,
		},
		// AlphaTest should be unchanged from the master baseline.
		three_devices.AlphaTest: map[types.Digest]types.Label{
			three_devices.AlphaGood1Digest: types.POSITIVE,
		},
		three_devices.BetaTest: map[types.Digest]types.Label{
			LambdaNewDigest: types.POSITIVE,
			// Note that the state caused by this set of expectations overwrites what
			// was on the master branch
			three_devices.BetaGood1Digest:      types.NEGATIVE,
			three_devices.BetaUntriaged1Digest: types.POSITIVE,
		},
	}, b.Expectations)

	// Ensure that reading the issue branch does not impact the master branch
	b, err = baseliner.FetchBaseline(testCommitHash, types.MasterBranch, false)
	assert.NoError(t, err)
	assert.Equal(t, three_devices.MakeTestBaseline(), b)
}

func TestFetchBaselineCachingSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	testCommitHash := "abcd12345"

	mgs := makeMockGCSStorage()
	defer mgs.AssertExpectations(t)

	// ReadBaseline should only be called once despite multiple requests below
	mgs.On("ReadBaseline", testCommitHash, types.MasterBranch).Return(three_devices.MakeTestBaseline(), nil).Once()

	baseliner, err := New(mgs, nil, nil, nil)
	assert.NoError(t, err)

	for i := 0; i < 10; i++ {
		b, err := baseliner.FetchBaseline(testCommitHash, types.MasterBranch, false)
		assert.NoError(t, err)
		assert.NotNil(t, b)
		deepequal.AssertDeepEqual(t, three_devices.MakeTestBaseline(), b)
	}
}

// TODO(kjlubick): TestFetchBaseline which returns nil (difficult because it shells out to
// PushMasterBaseline)

// TestPushMasterBaselineSunnyDay verifies that the baseliner pulls in the latest tile
// (which has the traces and known images) and combines it with the triage status from
// ExpectationsStorage to create a baseline per commit.
func TestPushMasterBaselineSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mgs := makeMockGCSStorage()
	mcs := &mocks.TileInfo{}
	mes := &mocks.ExpectationsStore{}

	defer mgs.AssertExpectations(t)
	defer mcs.AssertExpectations(t)
	defer mes.AssertExpectations(t)

	mcs.On("AllCommits").Return(three_devices.MakeTestCommits())

	mes.On("Get").Return(three_devices.MakeTestExpectations(), nil)

	bl := three_devices.MakeTestExpectations().AsBaseline()

	mgs.On("WriteBaseline", mock.AnythingOfType("*baseline.Baseline"), mock.AnythingOfType("string")).Run(func(args mock.Arguments) {
		b := args.Get(0).(*baseline.Baseline)
		assert.NotNil(t, b)

		assert.Equal(t, bl, b.Expectations)
		assert.Equal(t, types.MasterBranch, b.Issue)
	}).Return("gs://test-bucket/baselines/foo-baseline.json", nil).Times(3) // once per commit

	baseliner, err := New(mgs, mes, nil, nil)
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

func assertLabel(t *testing.T, b *baseline.Baseline, testName types.TestName, hash types.Digest, label types.Label) {
	test, ok := b.Expectations[testName]
	if !ok {
		assert.Failf(t, "assertLabel", "Could not find test %s in baseline %#v", testName, b)
	}

	lab, ok := test[hash]
	if !ok {
		assert.Failf(t, "assertLabel", "Could not find hash %s in test %#v (name %s) (baseline %#v)", hash, test, testName, b)
	}
	assert.Equal(t, label, lab)
}
