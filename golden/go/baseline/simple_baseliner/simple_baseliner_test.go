package simple_baseliner

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/mocks"
	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

const masterBranch = ""
const noCRS = ""

// Test that the baseline fetcher produces a master baseline.
func TestFetchBaselineSunnyDay(t *testing.T) {
	unittest.SmallTest(t)

	mes := &mocks.ExpectationsStore{}
	defer mes.AssertExpectations(t)

	mes.On("Get").Return(three_devices.MakeTestExpectations(), nil).Once()

	baseliner := New(mes)

	b, err := baseliner.FetchBaseline(masterBranch, "github", false)
	assert.NoError(t, err)

	expectedBaseline := three_devices.MakeTestExpectations().AsBaseline()

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

	additionalTriages := types.Expectations{
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
	}

	mes := &mocks.ExpectationsStore{}
	mesCL := &mocks.ExpectationsStore{}
	defer mes.AssertExpectations(t)
	defer mesCL.AssertExpectations(t)

	mes.On("Get").Return(three_devices.MakeTestExpectations(), nil).Once()
	mes.On("ForChangeList", clID, crs).Return(mesCL).Once()
	// mock the expectations that a user would have applied to their CL (that
	// are not live on master yet).
	mesCL.On("Get").Return(additionalTriages, nil).Once()

	baseliner := New(mes)

	b, err := baseliner.FetchBaseline(clID, crs, false)
	assert.NoError(t, err)

	assert.Equal(t, clID, b.ChangeListID)
	assert.Equal(t, crs, b.CodeReviewSystem)
	// The expectation should be the master baseline merged in with the additionalTriages
	// with additionalTriages overwriting existing expectations, if applicable.
	assert.Equal(t, types.Expectations{
		"brand-new-test": map[types.Digest]types.Label{
			IotaNewDigest: types.POSITIVE,
		},
		// AlphaTest should be unchanged from the master baseline.
		three_devices.AlphaTest: map[types.Digest]types.Label{
			three_devices.AlphaGood1Digest: types.POSITIVE,
		},
		three_devices.BetaTest: map[types.Digest]types.Label{
			LambdaNewDigest:                    types.POSITIVE,
			three_devices.BetaUntriaged1Digest: types.POSITIVE,
		},
	}, b.Expectations)

	mes.On("Get").Return(three_devices.MakeTestExpectations(), nil).Once()

	// Ensure that reading the issue branch does not impact the master branch
	b, err = baseliner.FetchBaseline(masterBranch, noCRS, false)
	assert.NoError(t, err)
	assert.Equal(t, three_devices.MakeTestBaseline(), b)
}
