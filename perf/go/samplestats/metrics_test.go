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
