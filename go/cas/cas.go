package cas

// Package cas provides an abstraction layer on top of Isolate and RBE-CAS.

import (
	"context"
)

// CAS represents a content-addressed storage system.
type CAS interface {
	// Upload the given inputs to content-addressed storage and return the
	// resulting digest.
	Upload(ctx context.Context, root string, paths, excludes []string) (string, error)

	// Download the given entry from content-addressed storage.
	Download(ctx context.Context, root, digest string) error

	// Merge returns a new Entry which contains all of the given Entries.
	Merge(ctx context.Context, digests []string) (string, error)

	// Close cleans up resources used by the CAS instance.
	Close() error
}
