package samplestats

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/perf/go/ingest/parser"
)

func TestCalculateMetrics_EmptySamples_SuccessWithNaNs(t *testing.T) {
	m := calculateMetrics(Config{}, parser.Samples{})
	assert.True(t, math.IsNaN(m.Mean))
	assert.True(t, math.IsNaN(m.Percent))
	assert.True(t, math.IsNaN(m.StdDev))
}

func TestCalculateMetrics_SimpleSamples_Success(t *testing.T) {
	m := calculateMetrics(Config{}, parser.Samples{
		Values: []float64{1, 2, 1, 1},
	})
	assert.Equal(t, 1.25, m.Mean)
	assert.Equal(t, float64(40), m.Percent)
	assert.Equal(t, float64(0.5), m.StdDev)
}

func TestCalculateMetrics_AlmostOutliersOnTheEdge_OutliersAreNotRemoved(t *testing.T) {
	// The stats package uses  the interpolation method R8 from Hyndman and Fan
	// (1996). So the following values: {3, 3, 6, 9, 9, 12, 15, 15}, have
	// quartiles of: {4.25, 13.75}
	// and thus outlier limits of: {-10, 28}

	m := calculateMetrics(Config{IQRR: true}, parser.Samples{
		Values: []float64{-10, 3, 6, 9, 9, 12, 15, 28},
	})
	assert.Equal(t, []float64{-10, 3, 6, 9, 9, 12, 15, 28}, m.Values)
}

func TestCalculateMetrics_OutliersOverTheEdge_OutliersAreBothRemoved(t *testing.T) {
	// The stats package uses  the interpolation method R8 from Hyndman and Fan
	// (1996). So the following values: {3, 3, 6, 9, 9, 12, 15, 15}, have
	// quartiles of: {4.25, 13.75}
	// and thus outlier limits of: {-10, 28}

	m := calculateMetrics(Config{IQRR: true}, parser.Samples{
		Values: []float64{-10.1, 3, 6, 9, 9, 12, 15, 28.1},
	})
	assert.Equal(t, []float64{3, 6, 9, 9, 12, 15}, m.Values)
}
