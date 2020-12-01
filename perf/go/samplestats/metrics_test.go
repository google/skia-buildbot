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
	m := calculateMetrics(Config{IQRR: true}, parser.Samples{
		Values: []float64{-3.33, 1, 2, 3, 3, 4, 5, 9.33},
	})
	assert.Equal(t, []float64{-3.33, 1, 2, 3, 3, 4, 5, 9.33}, m.Values)
}

func TestCalculateMetrics_OutliersOverTheEdge_OutliersAreBothRemoved(t *testing.T) {
	m := calculateMetrics(Config{IQRR: true}, parser.Samples{
		Values: []float64{-3.4, 1, 2, 3, 3, 4, 5, 9.4},
	})
	assert.Equal(t, []float64{1, 2, 3, 3, 4, 5}, m.Values)
}
