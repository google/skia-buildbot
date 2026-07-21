package dfbuilder

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/types"
)

func TestCommitWindowCalculator_InitialState(t *testing.T) {
	calc := newCommitWindowCalculator(types.CommitNumber(1000), 200, 256, 10)
	begin, end := calc.CurrentBounds()
	assert.Equal(t, types.CommitNumber(1000), end)
	assert.Equal(t, types.CommitNumber(745), begin)
	assert.False(t, calc.ShouldStop())
}

func TestCommitWindowCalculator_EmptyStepGrowth(t *testing.T) {
	calc := newCommitWindowCalculator(types.CommitNumber(1000), 200, 256, 3)

	// Step 1: No data. Should grow search window by 2x (consecutiveZeroSteps=1)
	calc.RecordEmptyStep()
	begin, end := calc.CurrentBounds()
	assert.Equal(t, types.CommitNumber(744), end)
	// prevScannedRange was 256. 256 * 2 = 512 commits.
	assert.Equal(t, types.CommitNumber(233), begin)
	assert.False(t, calc.ShouldStop())

	// Step 2: No data. Should grow window by 4x (consecutiveZeroSteps=2)
	calc.RecordEmptyStep()
	begin, end = calc.CurrentBounds()
	assert.Equal(t, types.CommitNumber(232), end)
	// tileSize was 256. 256 * 4 = 1024 commits. Capped by begin=0.
	assert.Equal(t, types.CommitNumber(0), begin)
	assert.False(t, calc.ShouldStop())

	// Step 3: No data. Should hit maxSearchSteps limit
	calc.RecordEmptyStep()
	assert.True(t, calc.ShouldStop())
}

func TestCommitWindowCalculator_SuccessStepDensityEstimation(t *testing.T) {
	// Requesting 200 points. Tile size 256.
	calc := newCommitWindowCalculator(types.CommitNumber(10000), 200, 256, 10)

	// Step 1: Found 10 commits with data.
	// Scanned range: 256. Density: 10 / 256 = ~0.039.
	// Needed: 190. Estimated needed commits: 190 / 0.039 = ~4864 commits.
	// nextScanSize: estimated * 1.5 (safety multiplier) = ~7296 commits.
	// searchTiles: (7296 + 255) / 256 = 29 tiles -> 29 * 256 = 7424 commits.
	calc.RecordSuccessStep(10)
	begin, end := calc.CurrentBounds()
	assert.Equal(t, types.CommitNumber(9744), end)   // 10000 - 256
	assert.Equal(t, types.CommitNumber(2321), begin) // 9744 - 7424 + 1
	assert.False(t, calc.ShouldStop())
}

func TestCommitWindowCalculator_TargetAchieved(t *testing.T) {
	calc := newCommitWindowCalculator(types.CommitNumber(1000), 200, 256, 10)
	calc.RecordSuccessStep(200)
	assert.True(t, calc.ShouldStop())
}
