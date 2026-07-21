package dfbuilder

import (
	"math"

	"go.skia.org/infra/perf/go/types"
)

const (
	maxGrowthExponent = 10

	// windowCalculatorSafetyMultiplier is the safety buffer multiplier applied to the
	// estimated commits needed to find the remaining data points in the calculator.
	windowCalculatorSafetyMultiplier = 1.5
)

// commitWindowCalculator manages sliding commit window boundaries, exponential growth, and step-size density estimation.
type commitWindowCalculator struct {
	tileSize             int32
	maxSearchSteps       int
	endIndex             types.CommitNumber
	beginIndex           types.CommitNumber
	prevScannedRange     int32
	consecutiveZeroSteps int
	steps                int
	numStepsNoData       int
	targetN              int32
	totalFound           int32
}

func newCommitWindowCalculator(endIndex types.CommitNumber, targetN int32, tileSize int32, maxSearchSteps int) *commitWindowCalculator {
	beginIndex := endIndex.Add(-(tileSize - 1))
	if beginIndex < 0 {
		beginIndex = 0
	}
	return &commitWindowCalculator{
		tileSize:         tileSize,
		maxSearchSteps:   maxSearchSteps,
		endIndex:         endIndex,
		beginIndex:       beginIndex,
		prevScannedRange: int32(endIndex - beginIndex + 1),
		targetN:          targetN,
		steps:            1,
	}
}

// CurrentBounds returns the current beginIndex and endIndex.
func (c *commitWindowCalculator) CurrentBounds() (types.CommitNumber, types.CommitNumber) {
	return c.beginIndex, c.endIndex
}

// ShouldStop checks if the search should terminate (target achieved, out of bounds, or too many empty steps).
func (c *commitWindowCalculator) ShouldStop() bool {
	if c.totalFound >= c.targetN {
		return true
	}
	if c.endIndex < 0 {
		return true
	}
	if c.numStepsNoData > c.maxSearchSteps {
		return true
	}
	return false
}

// RecordEmptyStep grows the window exponentially when no commits/traces are found.
func (c *commitWindowCalculator) RecordEmptyStep() {
	c.numStepsNoData++
	c.consecutiveZeroSteps++
	if c.consecutiveZeroSteps > maxGrowthExponent {
		c.consecutiveZeroSteps = maxGrowthExponent
	}
	growthFactor := int(math.Pow(2, float64(c.consecutiveZeroSteps)))
	nextScanSize := float64(c.tileSize) * float64(growthFactor)

	searchTiles := (int(nextScanSize) + int(c.tileSize-1)) / int(c.tileSize)
	nextScanSize = float64(searchTiles * int(c.tileSize))

	c.steps++
	c.endIndex = c.beginIndex - 1
	c.beginIndex = c.endIndex - types.CommitNumber(nextScanSize) + 1
	if c.beginIndex < 0 {
		c.beginIndex = 0
	}
	c.prevScannedRange = int32(c.endIndex - c.beginIndex + 1)
}

// RecordSuccessStep dynamically adjusts the next scan size based on observed density.
func (c *commitWindowCalculator) RecordSuccessStep(commitsWithData int) {
	c.totalFound += int32(commitsWithData)
	c.consecutiveZeroSteps = 0
	c.steps++

	if c.totalFound >= c.targetN {
		return
	}

	// Calculate density of commits with actual data points
	density := float64(commitsWithData) / float64(c.prevScannedRange)
	if density < 0.01 {
		density = 0.01
	}

	neededCommits := c.targetN - c.totalFound
	estimatedCommits := float64(neededCommits) / density
	nextScanSize := estimatedCommits * windowCalculatorSafetyMultiplier

	searchTiles := (int(nextScanSize) + int(c.tileSize-1)) / int(c.tileSize)
	nextScanSize = float64(searchTiles * int(c.tileSize))

	c.endIndex = c.beginIndex - 1
	c.beginIndex = c.endIndex - types.CommitNumber(nextScanSize) + 1
	if c.beginIndex < 0 {
		c.beginIndex = 0
	}
	c.prevScannedRange = int32(c.endIndex - c.beginIndex + 1)
}
