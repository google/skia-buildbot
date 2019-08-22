package gitstore_deprecated

import (
	"context"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

const (
	// DELETE_BRANCH is a placeholder which can be used as a value in the
	// branch map passed to GitStore.PutBranches to signify that the branch
	// should be deleted.
	DELETE_BRANCH = "@DELETE"

	ALL_BRANCHES = ""
)

// GitStore defines the functions of a data store for Git metadata (aka vcsinfo.LongCommit)
// Each GitStore instance relates to one repository that is defined in the constructor of the
// implementation.
type GitStore interface {
	// Put stores the given commits. They can be retrieved in order of timestamps by using
	// RangeByTime or RangeN (no topological ordering).
	Put(ctx context.Context, commits []*vcsinfo.LongCommit) error

	// Get retrieves the commits identified by 'hashes'. The return value will always have the
	// length of the input value and the results will line up by index. If a commit does not exist
	// the corresponding entry in the result is nil.
	// The function will only return an error if the retrieval operation (the I/O) fails, not
	// if the given hashes do not exist or are invalid.
	Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error)

	// PutBranches updates branches in the repository. It writes indices for the branches so they
	// can be retrieved via RangeN and RangeByTime. These are ordered in toplogical order with only
	// first-parents included.
	// 'branches' maps branchName -> commit_hash to indicate the head of a branch. The store then
	// calculates the commits of the branch and updates the indices accordingly. Branches which
	// already exist in the GitStore are not removed if not present in 'branches'; if DELETE_BRANCH
	// string is used as the head instead of a commit hash, then the branch is removed.
	PutBranches(ctx context.Context, branches map[string]string) error

	// GetBranches returns the current branches in the store. It maps[branchName]->BranchPointer.
	// A BranchPointer contains the HEAD commit and also the Index of the HEAD commit, which is
	// usually the total number of commits in the branch minus 1.
	GetBranches(ctx context.Context) (map[string]*BranchPointer, error)

	// RangeN returns all commits in the half open index range [startIndex, endIndex).
	// Thus not including endIndex. It returns the commits in the given branch sorted in ascending
	// order by Index and the commits are topologically sorted only including first-parent commits.
	RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error)

	// RangeByTime returns all commits in the half open time range [start, end). Thus not
	// including commits at 'end' time.
	// Caveat: The returned results will match the requested range, but will be sorted by Index.
	// So if the timestamps within a commit are not in order they will be unordered in the results.
	RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error)

	// GetGraph returns the commit graph of the entire repository.
	GetGraph(ctx context.Context) (*CommitGraph, error)
}

// GitStoreBased is an interface that tags an object as being based on GitStore and the
// underlying GitStore instance can be retrieved. e.g.
//
// if gsb, ok := someInstance.(GitStoreBased); ok {
//   gitStore := gsb.GetGitStore()
//   ...  do something with gitStore
// }
//
type GitStoreBased interface {
	// GetGitStore returns the underlying GitStore instances.
	GetGitStore() GitStore
}

// BranchPointer captures the HEAD of a branch and the index of that commit.
type BranchPointer struct {
	Head  string
	Index int
}

// RepoInfo contains information about one repo in the GitStore.
type RepoInfo struct {
	// Numeric id of the repo. This is unique within all repos in a BT table. This ID is uniquely
	// assigned whenever a new repo is added.
	ID int64

	// RepoURL contains the URL of the repo as returned by git.NormalizeURL(...).
	RepoURL string

	// Branches contain all the branches in the repo, mapping branch_name -> branch_pointer.
	Branches map[string]*BranchPointer
}
