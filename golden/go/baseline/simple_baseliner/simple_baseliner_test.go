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

const primaryBranch = ""
const noCRS = ""

// Test that the baseline fetcher produces a baseline for the primary branch.
func TestFetchBaselineSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)

	mes.On("GetCopy", testutils.AnyContext).Return(three_devices.MakeTestExpectations(), nil).Once()

	baseliner := New(mes)

	b, err := baseliner.FetchBaseline(context.Background(), primaryBranch, "github", false)
	assert.NoError(t, err)

	exp := three_devices.MakeTestExpectations()
	expectedBaseline := exp.AsBaseline()

	assert.Equal(t, expectedBaseline, b.DeprecatedExpectations)
	assert.Equal(t, primaryBranch, b.ChangelistID)
	assert.Equal(t, noCRS, b.CodeReviewSystem)
	assert.NotEqual(t, "", b.MD5)
}

// TestFetchBaselineChangelistSunnyDay tests that the baseline fetcher behaves differently
// when requesting a baseline for a given tryjob.
func TestFetchBaselineChangelistSunnyDay(t *testing.T) {
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
	additionalTriages.Set("brand-new-test", IotaNewDigest, expectations.Positive)
	additionalTriages.Set("brand-new-test", KappaNewDigest, expectations.Negative)
	additionalTriages.Set("brand-new-test", LambdaNewDigest, expectations.Untriaged)
	additionalTriages.Set(three_devices.BetaTest, MuNewDigest, expectations.Positive)
	additionalTriages.Set(three_devices.BetaTest, NuNewDigest, expectations.Untriaged)
	additionalTriages.Set(three_devices.BetaTest, three_devices.BetaPositiveDigest, expectations.Negative)
	additionalTriages.Set(three_devices.BetaTest, three_devices.BetaUntriagedDigest, expectations.Positive)

	mes := &mock_expectations.Store{}
	mesCL := &mock_expectations.Store{}
	defer mes.AssertExpectations(t)
	defer mesCL.AssertExpectations(t)

	mes.On("GetCopy", testutils.AnyContext).Return(three_devices.MakeTestExpectations(), nil).Once()
	mes.On("ForChangelist", clID, crs).Return(mesCL).Once()
	// mock the expectations that a user would have applied to their CL (that
	// are not live on the primary branch yet).
	mesCL.On("GetCopy", testutils.AnyContext).Return(&additionalTriages, nil).Once()

	baseliner := New(mes)

	b, err := baseliner.FetchBaseline(context.Background(), clID, crs, false)
	assert.NoError(t, err)

	assert.Equal(t, clID, b.ChangelistID)
	assert.Equal(t, crs, b.CodeReviewSystem)
	// The expectation should be the baseline for the primary branch merged in with the
	// additionalTriages, which overwrite any existing expectations.
	assert.Equal(t, expectations.Baseline{
		"brand-new-test": {
			IotaNewDigest:  expectations.Positive,
			KappaNewDigest: expectations.Negative,
		},
		// AlphaTest should be unchanged from the baseline for the primary branch.
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
	assert.Equal(t, expectations.Baseline{
		"brand-new-test": {
			IotaNewDigest:  expectations.Positive,
			KappaNewDigest: expectations.Negative,
		},
		// AlphaTest should be unchanged from the baseline for the primary branch.
		three_devices.AlphaTest: {
			three_devices.AlphaPositiveDigest: expectations.Positive,
			three_devices.AlphaNegativeDigest: expectations.Negative,
		},
		three_devices.BetaTest: {
			MuNewDigest:                       expectations.Positive,
			three_devices.BetaPositiveDigest:  expectations.Negative,
			three_devices.BetaUntriagedDigest: expectations.Positive,
		},
	}, b.DeprecatedExpectations)

	mes.On("GetCopy", testutils.AnyContext).Return(three_devices.MakeTestExpectations(), nil).Once()

	// Ensure that reading the issue branch does not impact the primary branch
	b, err = baseliner.FetchBaseline(context.Background(), primaryBranch, noCRS, false)
	assert.NoError(t, err)
	assert.Equal(t, three_devices.MakeTestBaseline(), b)
}
