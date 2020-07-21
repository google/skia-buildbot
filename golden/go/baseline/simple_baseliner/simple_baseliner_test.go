package simple_baseliner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/expectations"
	mock_expectations "go.skia.org/infra/golden/go/expectations/mocks"
	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

const masterBranch = ""
const noCRS = ""

// Test that the baseline fetcher produces a master baseline.
func TestFetchBaselineSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)

	mes.On("GetCopy", testutils.AnyContext).Return(three_devices.MakeTestExpectations(), nil).Once()

	baseliner := New(mes)

	b, err := baseliner.FetchBaseline(context.Background(), masterBranch, "github", false)
	assert.NoError(t, err)

	exp := three_devices.MakeTestExpectations()
	expectedBaseline := exp.AsBaseline()

	assert.Equal(t, expectedBaseline, b.Expectations)
	assert.Equal(t, masterBranch, b.ChangeListID)
	assert.Equal(t, noCRS, b.CodeReviewSystem)
	assert.NotEqual(t, "", b.MD5)
}

// TestFetchBaselineChangeListSunnyDay tests that the baseline fetcher behaves differently
// when requesting a baseline for a given tryjob.
func TestFetchBaselineChangeListSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	clID := "1234"
	crs := "gerrit"

	// These are valid, but arbitrary md5 hashes
	IotaNewDigest := types.Digest("1115fba4ce5b4cb9ffd595beb63e7389")
	KappaNewDigest := types.Digest("222d894f5b680a9f7bd74c8004b7d88d")
	LambdaNewDigest := types.Digest("3333fe3127b984e4ff39f4885ddb0d98")
	MuNewDigest := types.Digest("444494bf9ae7f94bf9ae7f94bf9ae7f8")
	NuNewDigest := types.Digest("5555c0dab629ef092bc0dab629ef092b")

	var additionalTriages expectations.Expectations
	additionalTriages.Set("brand-new-test", IotaNewDigest, expectations.PositiveStr)
	additionalTriages.Set("brand-new-test", KappaNewDigest, expectations.NegativeStr)
	additionalTriages.Set("brand-new-test", LambdaNewDigest, expectations.UntriagedStr)
	additionalTriages.Set(three_devices.BetaTest, MuNewDigest, expectations.PositiveStr)
	additionalTriages.Set(three_devices.BetaTest, NuNewDigest, expectations.UntriagedStr)
	additionalTriages.Set(three_devices.BetaTest, three_devices.BetaPositiveDigest, expectations.NegativeStr)
	additionalTriages.Set(three_devices.BetaTest, three_devices.BetaUntriagedDigest, expectations.PositiveStr)

	mes := &mock_expectations.Store{}
	mesCL := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)
	defer mesCL.AssertExpectations(t)

	mes.On("GetCopy", testutils.AnyContext).Return(three_devices.MakeTestExpectations(), nil).Once()
	mes.On("ForChangeList", clID, crs).Return(mesCL).Once()
	// mock the expectations that a user would have applied to their CL (that
	// are not live on master yet).
	mesCL.On("GetCopy", testutils.AnyContext).Return(&additionalTriages, nil).Once()

	baseliner := New(mes)

	b, err := baseliner.FetchBaseline(context.Background(), clID, crs, false)
	assert.NoError(t, err)

	assert.Equal(t, clID, b.ChangeListID)
	assert.Equal(t, crs, b.CodeReviewSystem)
	// The expectation should be the master baseline merged in with the additionalTriages
	// with additionalTriages overwriting existing expectations, if applicable.
	assert.Equal(t, expectations.Baseline{
		"brand-new-test": {
			IotaNewDigest:  expectations.Positive,
			KappaNewDigest: expectations.Negative,
		},
		// AlphaTest should be unchanged from the master baseline.
		three_devices.AlphaTest: {
			three_devices.AlphaPositiveDigest: expectations.Positive,
			three_devices.AlphaNegativeDigest: expectations.Negative,
		},
		three_devices.BetaTest: {
			MuNewDigest:                       expectations.Positive,
			three_devices.BetaPositiveDigest:  expectations.Negative,
			three_devices.BetaUntriagedDigest: expectations.Positive,
		},
	}, b.Expectations)

	mes.On("GetCopy", testutils.AnyContext).Return(three_devices.MakeTestExpectations(), nil).Once()

	// Ensure that reading the issue branch does not impact the master branch
	b, err = baseliner.FetchBaseline(context.Background(), masterBranch, noCRS, false)
	assert.NoError(t, err)
	assert.Equal(t, three_devices.MakeTestBaseline(), b)
}
