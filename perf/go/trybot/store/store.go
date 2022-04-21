// Package store stores the results from trybot runs.
package store // import "go.skia.org/infra/perf/go/trybot/store"

import (
	"context"
	"time"

	"go.skia.org/infra/perf/go/trybot"
	"go.skia.org/infra/perf/go/types"
)

// TryBotStore stores trybot results.
type TryBotStore interface {
	// Write a single file into the store.
	Write(ctx context.Context, tryFile trybot.TryFile) error

	// List returns all the unique CL/patch combinations
	// that have arrived since 'since'.
	List(ctx context.Context, since time.Time) ([]ListResult, error)

	// Get all the results for a given cl and patch number.
	Get(ctx context.Context, cl types.CL, patch int) ([]GetResult, error)
}

// ListResult is returned from TryBotStore.List().
type ListResult struct {
	CL    string
	Patch int
}

// GetResult is returned from TryBotStore.Get() and represents a single trace
// result.
type GetResult struct {
	TraceName string
	Value     float32
}
