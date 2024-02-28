package internal

import (
	"context"

	"go.skia.org/infra/pinpoint/go/compare"
)

// ComparePerformanceActivity wraps compare.ComparePerformance as activity
func ComparePerformanceActivity(ctx context.Context, valuesA, valuesB []float64, magnitude float64) (*compare.CompareResults, error) {
	return compare.ComparePerformance(valuesA, valuesB, magnitude)
}
