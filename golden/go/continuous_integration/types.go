// Package continuous_integration defines some types for getting tryjob-related data
// into and out of Continuous Integration Systems (e.g. BuildBucket, CirrusCI).
package continuous_integration

import (
	"context"
	"errors"
	"time"
)

// The Client interface is an abstraction around a Continuous Integration System.
type Client interface {
	// GetTryjob returns the TryJob corresponding to the given id.
	// Returns NotFound if it doesn't exist.
	GetTryJob(ctx context.Context, id string) (TryJob, error)
}

var NotFound = errors.New("not found")

type TryJob struct {
	// SystemID is expected to be unique between all TryJobs.
	SystemID string

	Name    string
	Status  TJStatus
	Updated time.Time
}

type TJStatus int

const (
	Running TJStatus = iota
	Complete
)
