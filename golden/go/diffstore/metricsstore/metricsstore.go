package metricsstore

import (
	"context"

	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

type MetricsStore interface {
	// PurgeDiffMetrics removes all diff metrics based on specific digests.
	PurgeDiffMetrics(ctx context.Context, digests types.DigestSlice) error

	// SaveDiffMetrics stores diff metrics.
	SaveDiffMetrics(ctx context.Context, id string, diffMetrics *diff.DiffMetrics) error

	// LoadDiffMetrics loads diff metrics from the store. If any were not found, the index of
	// that element will be nil.
	LoadDiffMetrics(ctx context.Context, ids []string) ([]*diff.DiffMetrics, error)
}
