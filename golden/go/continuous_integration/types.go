// Package continuous_integration defines some types for getting tryjob-related data
// into and out of Continuous Integration Systems (e.g. BuildBucket, CirrusCI).
package continuous_integration

import (
	"context"
	"errors"
	"sort"
	"time"
)

// The Client interface is an abstraction around a Continuous Integration System.
type Client interface {
	// GetTryjob returns the TryJob corresponding to the given id.
	// Returns ErrNotFound if it doesn't exist.
	GetTryJob(ctx context.Context, id string) (TryJob, error)
}

var ErrNotFound = errors.New("not found")

type TryJob struct {
	// SystemID is expected to be unique between all TryJobs for a given System.
	SystemID    string
	DisplayName string
	Updated     time.Time
}

// SortTryJobsByName sorts the given slice of TryJobs by DisplayName.
func SortTryJobsByName(xtj []TryJob) {
	sort.Slice(xtj, func(i, j int) bool {
		return xtj[i].DisplayName < xtj[j].DisplayName
	})
}
