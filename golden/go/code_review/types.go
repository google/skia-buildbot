// Package code_review defines some types for getting data into and out of
// Code Review Systems (e.g. Gerrit, GitHub).
package code_review

import (
	"context"
	"errors"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

// The Client interface is an abstraction around a Code Review System
type Client interface {
	// GetChangelist returns the Changelist corresponding to the given id.
	// Returns ErrNotFound if it doesn't exist.
	GetChangelist(ctx context.Context, id string) (Changelist, error)

	// GetPatchset returns the Patchset belonging to the changelist and the provided id or order.
	// One of psID or psOrder should be set - the non-zero version will be used.
	// Returns ErrNotFound if the Patchset or Changelist doesn't exist.
	GetPatchset(ctx context.Context, clID, psID string, psOrder int) (Patchset, error)

	// GetChangelistIDForCommit returns the Changelist id corresponding to the given git commit.
	// Returns ErrNotFound if one could not be identified.
	GetChangelistIDForCommit(ctx context.Context, commit *vcsinfo.LongCommit) (string, error)

	// CommentOn creates a comment on the CRS for the given CL with the given message.
	CommentOn(ctx context.Context, clID, message string) error
}

// The ChangelistLandedUpdater interface is an abstraction around the code that tracks Changelists
// which land.
type ChangelistLandedUpdater interface {
	// UpdateChangelistsAsLanded goes through the given commits and marks any Changelist
	// objects as Landed. For those that are marked as landed, it should update the master
	// branch's Expectations as well.
	UpdateChangelistsAsLanded(ctx context.Context, commits []*vcsinfo.LongCommit) error
}

// The ChangelistCommenter interface is an abstraction around the code which comments on CLs in
// the Code Review System to which they belong.
type ChangelistCommenter interface {
	// CommentOnChangelistsWithUntriagedDigests comments (exactly once per patchset) on a Changelist
	// with unknown
	CommentOnChangelistsWithUntriagedDigests(ctx context.Context) error
}

// ErrNotFound is an error used to indicate something could not be found.
// TODO(kjlubick) This model of err checking is potentially brittle, perhaps something like
//   https://golang.org/pkg/os/#IsExist is better, or using errors.Is
//   https://dave.cheney.net/2016/04/27/dont-just-check-errors-handle-them-gracefully
var ErrNotFound = errors.New("not found")

type Changelist struct {
	// SystemID is expected to be unique between all Changelists for a given system and repo.
	SystemID string

	Owner   string
	Status  CLStatus
	Subject string
	Updated time.Time
}

type CLStatus int

const (
	Open CLStatus = iota
	Abandoned
	Landed
)

func (c CLStatus) String() string {
	switch c {
	case Open:
		return "Open"
	case Abandoned:
		return "Abandoned"
	case Landed:
		return "Landed"
	}
	return "<unknown>"
}

type Patchset struct {
	// SystemID may or may not be unique for all Patchsets for a given system.
	// Definitely unique for a given Changelist.
	SystemID string

	// ChangelistID is the id that the Patchset belongs to.
	ChangelistID string
	// It is convenient to think about Patchsets starting at 1 and increasing
	// monotonically. This gives some measure of time/progress. Order is the
	// index of this Patchset relative to all other Patchsets on this CL.
	Order   int
	GitHash string
	// CommentedOnCL are used to keep track of "Do we need to notify
	// the user about UntriagedDigests for this code?" and "Did we notify them?".
	CommentedOnCL                 bool
	LastCheckedIfCommentNecessary time.Time
}
