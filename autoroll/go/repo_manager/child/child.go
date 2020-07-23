package child

/*
   Package child contains implementations of the Child interface.
*/

import (
	"context"
	"os"

	"go.skia.org/infra/autoroll/go/revision"
)

// Child represents a Child (git repo or otherwise) which can be rolled into a
// Parent.
type Child interface {
	// Update updates the local view of the Child and returns the tip-of-
	// tree Revision and the list of not-yet-rolled revisions, or any error
	// which occurred, given the last-rolled revision.
	Update(context.Context, *revision.Revision) (*revision.Revision, []*revision.Revision, error)

	// GetRevision returns a Revision instance associated with the given
	// revision ID.
	GetRevision(context.Context, string) (*revision.Revision, error)

	// Download downloads the Child at the given Revision to the given
	// destination.
	Download(context.Context, *revision.Revision, string) error

	Reader
}

// Reader provides methods for reading the contents of a Child at a
// particular Revision.
type Reader interface {
	// ReadFile retrieves the FileInfo and contents of the given file path
	// within the Child at the given Revision.
	ReadFile(context.Context, *revision.Revision, string) (string, error)

	// ReadDir retrieves the contents of the given directory of the Child at the
	// given Revision.
	ReadDir(context.Context, *revision.Revision, string) ([]os.FileInfo, error)

	// Stat returns an os.FileInfo describing the given path at the given
	// Revision.
	Stat(context.Context, *revision.Revision, string) (os.FileInfo, error)
}
