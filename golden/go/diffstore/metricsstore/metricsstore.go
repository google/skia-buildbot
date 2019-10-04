package metricsstore

import (
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/types"
)

type MetricsStore interface {
	// PurgeDiffMetrics removes all diff metrics based on specific digests.
	PurgeDiffMetrics(digests types.DigestSlice) error

	// SaveDiffMetrics stores diff metrics.
	SaveDiffMetrics(id string, diffMetrics *diff.DiffMetrics) error

	// LoadDiffMetrics loads diff metrics from disk.
	LoadDiffMetrics(id string) (*diff.DiffMetrics, error)
}
