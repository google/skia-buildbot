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

	// LoadDiffMetrics loads diff metrics from disk.
	LoadDiffMetrics(ctx context.Context, id string) (*diff.DiffMetrics, error)
}
