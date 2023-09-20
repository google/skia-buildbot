package git

import (
	"context"
	"time"

	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/types"
)

// Git is the interface for the minimal functionality Perf needs to interface to
// a git repo.
type Git interface {
	// StartBackgroundPolling starts a background process that periodically adds
	// new commits to the database.
	StartBackgroundPolling(ctx context.Context, duration time.Duration)

	// Update finds all the new commits added to the repo since our last Update.
	Update(ctx context.Context) error

	// GetCommitNumber looks up the commit number from Commits table given a git hash or commit number
	GetCommitNumber(ctx context.Context, githash string, commitNumber types.CommitNumber) (types.CommitNumber, error)

	// CommitNumberFromGitHash looks up the commit number given the git hash.
	CommitNumberFromGitHash(ctx context.Context, githash string) (types.CommitNumber, error)

	// CommitFromCommitNumber returns all the stored details for a given CommitNumber.
	CommitFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (provider.Commit, error)

	// CommitSliceFromCommitNumberSlice returns all the stored details for a given slice of CommitNumbers.
	CommitSliceFromCommitNumberSlice(ctx context.Context, commitNumberSlice []types.CommitNumber) ([]provider.Commit, error)

	// CommitNumberFromTime finds the index of the closest commit with a commit time
	// less than or equal to 't'.
	//
	// Pass in zero time, i.e. time.Time{} to indicate to just get the most recent
	// commit.
	CommitNumberFromTime(ctx context.Context, t time.Time) (types.CommitNumber, error)

	// CommitSliceFromTimeRange returns a slice of Commits that fall in the range
	// [begin, end), i.e  inclusive of begin and exclusive of end.
	CommitSliceFromTimeRange(ctx context.Context, begin, end time.Time) ([]provider.Commit, error)

	// CommitSliceFromCommitNumberRange returns a slice of Commits that fall in the range
	// [begin, end], i.e  inclusive of both begin and end.
	CommitSliceFromCommitNumberRange(ctx context.Context, begin, end types.CommitNumber) ([]provider.Commit, error)

	// GitHashFromCommitNumber returns the git hash of the given commit number.
	GitHashFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (string, error)

	// PreviousGitHashFromCommitNumber returns the previous git hash of the given commit number.
	PreviousGitHashFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (string, error)

	// PreviousCommitNumberFromCommitNumber returns the previous commit number of the given commit number.
	PreviousCommitNumberFromCommitNumber(ctx context.Context, commitNumber types.CommitNumber) (types.CommitNumber, error)

	// CommitNumbersWhenFileChangesInCommitNumberRange returns a slice of commit
	// numbers when the given file has changed between [begin, end], i.e. the given
	// range is exclusive of the begin commit and inclusive of the end commit.
	CommitNumbersWhenFileChangesInCommitNumberRange(ctx context.Context, begin, end types.CommitNumber, filename string) ([]types.CommitNumber, error)

	// LogEntry returns the full log entry of a commit (minus the diff) as a string.
	LogEntry(ctx context.Context, commit types.CommitNumber) (string, error)

	// RepoSuppliedCommitNumber returns true if the CommitNumber is actually
	// specified by information in the git commit messages.
	RepoSuppliedCommitNumber() bool
}
