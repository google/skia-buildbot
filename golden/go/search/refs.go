package search

import (
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/storage"
)

const (
	REF_CLOSEST_POSTIVE  = "pos"
	REF_CLOSEST_NEGATIVE = "neg"
	REF_PREVIOUS_TRACE   = "trace"
)

type ReferenceFn func() map[string]*diff.DiffMetrics

func referenceDiffs(storage *storage.Storage) (string, map[string]*CTDiffMetrics) {

	return "", nil
}
