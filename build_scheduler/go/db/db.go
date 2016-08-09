package db

import (
	"errors"
	"time"

	"go.skia.org/infra/go/buildbucket"
)

const (
	// Maximum number of simultaneous GetModifiedBuilds users.
	MAX_MODIFIED_BUILDS_USERS = 10

	// Expiration for GetModifiedBuilds users.
	MODIFIED_BUILDS_TIMEOUT = 10 * time.Minute
)

var (
	ErrTooManyUsers = errors.New("Too many users")
	ErrUnknownId    = errors.New("Unknown ID")
)

func IsTooManyUsers(e error) bool {
	return e != nil && e.Error() == ErrTooManyUsers.Error()
}

func IsUnknownId(e error) bool {
	return e != nil && e.Error() == ErrUnknownId.Error()
}

type Build struct {
	*buildbucket.Build
	Builder  string
	Commits  []string
	Revision string
}

func (b *Build) Copy() *Build {
	commits := make([]string, len(b.Commits))
	copy(commits, b.Commits)
	rv := &Build{
		Build:    b.Build.Copy(),
		Builder:  b.Builder,
		Commits:  commits,
		Revision: b.Revision,
	}
	return rv
}

type DB interface {
	// Close the [connection to the] DB.
	Close() error

	// GetBuildsFromDateRange retrieves all builds which started in the given date range.
	GetBuildsFromDateRange(time.Time, time.Time) ([]*Build, error)

	// GetModifiedBuilds returns all builds modified since the last time
	// GetModifiedBuilds was run with the given id.
	GetModifiedBuilds(string) ([]*Build, error)

	// PutBuild inserts or updates the Build in the database.
	PutBuild(*Build) error

	// PutBuilds inserts or updates the Builds in the database.
	PutBuilds([]*Build) error

	// StartTrackingModifiedBuilds initiates tracking of modified builds for
	// the current caller. Returns a unique ID which can be used by the caller
	// to retrieve builds which have been modified since the last query. The ID
	// expires after a period of inactivity.
	StartTrackingModifiedBuilds() (string, error)
}
