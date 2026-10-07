package db

import (
	"context"
	"sync"
	"time"

	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/sync/errgroup"
)

type AutoGardenerDB interface {
	// GetTaskSummary retrieves the Summary for the given task ID, if it exists.
	// If not, it returns nil with no error.
	GetTaskSummary(ctx context.Context, taskID string) (*types.TaskSummary, error)

	// PutTaskSummary sets the Summary for the given task ID, replacing any
	// existing entry.
	PutTaskSummary(ctx context.Context, taskID string, summary *types.TaskSummary) error

	// GetUnclassifiedTaskSummaries retrieves any TaskSummary which has not been
	// classified, keyed by task ID.
	GetUnclassifiedTaskSummaries(ctx context.Context, limit int) (map[string]*types.TaskSummary, error)

	// GetReport retrieves the latest Report for the given repo and branch, if
	// it exists. If not, it returns nil with no error.
	GetReport(ctx context.Context, repo, branch string) (*types.Report, error)

	// PutReport sets the latest Report for the given repo and branch, replacing
	// any existing entry.
	PutReport(ctx context.Context, repo, branch string, report *types.Report) error

	// GetFailureClass retrieves the FailureClass for the given ID, if it exists.
	// If not, it returns nil with no error.
	GetFailureClass(ctx context.Context, id string) (*types.FailureClass, error)

	// PutFailureClass sets the FailureClass for the given ID, replacing any
	// existing entry.
	PutFailureClass(ctx context.Context, fc *types.FailureClass) error

	// GetRecentFailureClasses retrieves FailureClasses seen after the given
	// timestamp, up to the specified limit.
	GetRecentFailureClasses(ctx context.Context, repo string, since time.Time, limit int) ([]*types.FailureClass, error)

	// ModifiedFailureClassesCh returns a channel which produces slices of
	// FailureClasses seen within the given window as they are modified in the DB.
	ModifiedFailureClassesCh(ctx context.Context, window time.Duration) <-chan []*types.FailureClass
}

// FetchLinkedFailureClasses retrieves any linked DuplicateOf or Duplicates
// FailureClasses that are not already present in the given slice, using the
// provided fetchFailureClass function, and returns the combined slice.
func FetchLinkedFailureClasses(ctx context.Context, fcs []*types.FailureClass, fetchFailureClass func(context.Context, string) (*types.FailureClass, error)) ([]*types.FailureClass, error) {
	var mtx sync.Mutex
	seen := make(map[string]bool, len(fcs))
	for _, fc := range fcs {
		seen[fc.Id] = true
	}

	rv := append([]*types.FailureClass(nil), fcs...)
	var eg errgroup.Group
	var enqueueLinked func(fc *types.FailureClass)
	enqueueLinked = func(fc *types.FailureClass) {
		var toFetch []string
		mtx.Lock()
		if dupOf := fc.DuplicateOf(); dupOf != "" && !seen[dupOf] {
			seen[dupOf] = true
			toFetch = append(toFetch, dupOf)
		}
		for _, dupID := range fc.Duplicates() {
			if dupID != "" && !seen[dupID] {
				seen[dupID] = true
				toFetch = append(toFetch, dupID)
			}
		}
		mtx.Unlock()

		for _, id := range toFetch {
			id := id // https://golang.org/doc/faq#closures_and_goroutines
			eg.Go(func() error {
				linked, err := fetchFailureClass(ctx, id)
				if err != nil {
					return skerr.Wrap(err)
				}
				if linked != nil {
					mtx.Lock()
					rv = append(rv, linked)
					mtx.Unlock()
					enqueueLinked(linked)
				}
				return nil
			})
		}
	}

	for _, fc := range fcs {
		enqueueLinked(fc)
	}
	if err := eg.Wait(); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}
