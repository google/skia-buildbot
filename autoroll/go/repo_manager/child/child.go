package child

/*
   Package child contains implementations of the Child interface.
*/

import (
	"context"

	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/go/skerr"
)

// ChildConfig provides configuration for a Child.
type ChildConfig struct {
	// Exactly one of these should be defined.
	Gitiles *GitilesConfig `json:"gitiles"`
}

func (c ChildConfig) Validate() error {
	count := 0
	if c.Gitiles != nil {
		count++
		if err := c.Gitiles.Validate(); err != nil {
			return skerr.Wrap(err)
		}
	}
	if count != 1 {
		return skerr.Fmt("Exactly one config type must be provided.")
	}
	return nil
}

// Child represents a Child (git repo or otherwise) which can be rolled into a
// Parent.
type Child interface {
	// Update updates the local view of the Child and returns the
	// tip-of-tree Revision and the list of not-yet-rolled revisions, or any
	// error which occurred, given the last-rolled revision.
	Update(context.Context, *revision.Revision) (*revision.Revision, []*revision.Revision, error)

	// GetRevision returns a Revision instance associated with the given
	// revision ID.
	GetRevision(context.Context, string) (*revision.Revision, error)
}
