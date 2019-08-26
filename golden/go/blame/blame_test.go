package blame

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
	three_devices "go.skia.org/infra/golden/go/testutils/data_three_devices"
)

func TestBlamerGetBlamesForTestThreeDevices(t *testing.T) {
	unittest.SmallTest(t)

	blamer := blamerWithCalculate(t)

	bd := blamer.GetBlamesForTest(three_devices.AlphaTest)
	assert.Len(t, bd, 1)
	b := bd[0]
	assert.NotNil(t, b)

	// The AlphaTest becomes untriaged in exactly the third commit for exactly
	// one trace, so blame is able to identify that author as the one
	// and only culprit.
	assert.Equal(t, WeightedBlame{
		Author: three_devices.ThirdCommitAuthor,
		Prob:   1,
	}, b)

	bd = blamer.GetBlamesForTest(three_devices.BetaTest)
	assert.Len(t, bd, 1)
	b = bd[0]
	assert.NotNil(t, b)

	// The BetaTest has an untriaged digest in the first commit and is missing
	// data after that, so this is the best that blamer can do.
	assert.Equal(t, WeightedBlame{
		Author: three_devices.FirstCommitAuthor,
		Prob:   1,
	}, b)

	bd = blamer.GetBlamesForTest("test_that_does_not_exist")
	assert.Len(t, bd, 0)
}

func TestBlamerGetBlameThreeDevices(t *testing.T) {
	unittest.SmallTest(t)

	blamer := blamerWithCalculate(t)
	commits := three_devices.MakeTestCommits()

	// In the first two commits, this untriaged image doesn't show up
	// so GetBlame should return empty.
	bd := blamer.GetBlame(three_devices.AlphaTest, three_devices.AlphaUntriaged1Digest, commits[0:2])
	assert.NotNil(t, bd)
	assert.Equal(t, BlameDistribution{
		Freq: []int{},
	}, bd)

	// Searching in the whole range should indicates that
	// the commit with index 2 (i.e. the third and last one)
	// has the blame
	bd = blamer.GetBlame(three_devices.AlphaTest, three_devices.AlphaUntriaged1Digest, commits[0:3])
	assert.NotNil(t, bd)
	assert.Equal(t, BlameDistribution{
		Freq: []int{2},
	}, bd)

	// The BetaUntriaged1Digest only shows up in the first commit (index 0)
	bd = blamer.GetBlame(three_devices.BetaTest, three_devices.BetaUntriaged1Digest, commits[0:3])
	assert.NotNil(t, bd)
	assert.Equal(t, BlameDistribution{
		Freq: []int{0},
	}, bd)

	// Good digests have no blame ever
	bd = blamer.GetBlame(three_devices.BetaTest, three_devices.BetaGood1Digest, commits[0:3])
	assert.NotNil(t, bd)
	assert.Equal(t, BlameDistribution{
		Freq: []int{},
	}, bd)

	// Negative digests have no blame ever
	bd = blamer.GetBlame(three_devices.AlphaTest, three_devices.AlphaBad1Digest, commits[0:3])
	assert.NotNil(t, bd)
	assert.Equal(t, BlameDistribution{
		Freq: []int{},
	}, bd)
}

// Returns a Blamer filled out with the data from three_devices.
func blamerWithCalculate(t *testing.T) Blamer {
	exp := three_devices.MakeTestExpectations()

	blamer, err := New(three_devices.MakeTestTile(), exp)
	assert.NoError(t, err)

	return blamer
}
