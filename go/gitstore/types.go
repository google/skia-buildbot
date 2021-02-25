package gitstore

import (
	"context"
	"time"

	"go.skia.org/infra/go/vcsinfo"
)

const (
	// ALL_BRANCHES is a placeholder which can be used to retrieve IndexCommits
	// for every branch, as opposed to just one.
	ALL_BRANCHES = "@all-commits"

	// DELETE_BRANCH is a placeholder which can be used as a value in the
	// branch map passed to GitStore.PutBranches to signify that the branch
	// should be deleted.
	DELETE_BRANCH = "@DELETE"
)

// GitStore defines the functions of a data store for Git metadata (aka vcsinfo.LongCommit)
// Each GitStore instance relates to one repository that is defined in the constructor of the
// implementation.
type GitStore interface {
	// Put stores the given commits. They can be retrieved in order of timestamps by using
	// RangeByTime or RangeN (no topological ordering). The Index and Branch information
	// on the commits must be correct, or the results of RangeN and RangeByTime will not
	// be correct.
	Put(ctx context.Context, commits []*vcsinfo.LongCommit) error

	// Get retrieves the commits identified by 'hashes'. The return value will always have the
	// length of the input value and the results will line up by index. If a commit does not exist
	// the corresponding entry in the result is nil.
	// The function will only return an error if the retrieval operation (the I/O) fails, not
	// if the given hashes do not exist or are invalid.
	Get(ctx context.Context, hashes []string) ([]*vcsinfo.LongCommit, error)

	// PutBranches updates the given branch heads in the GitStore. The 'branches' parameter
	// maps branch name to commit hash to indicate the head of a branch. All of the referenced
	// commits must already exist in the GitStore. If the DELETE_BRANCH string is used instead
	// of a commit hash, then the branch is removed. Any existing branches which are not
	// included in the call to PutBranches are left unchanged.
	PutBranches(ctx context.Context, branches map[string]string) error

	// GetBranches returns the current branches in the store. It maps[branchName]->BranchPointer.
	// A BranchPointer contains the HEAD commit and also the Index of the HEAD commit, which is
	// usually the total number of commits in the branch minus 1.
	GetBranches(ctx context.Context) (map[string]*BranchPointer, error)

	// RangeN returns all commits in the half open index range [startIndex, endIndex), thus not
	// including endIndex. It returns the commits in the given branch sorted in ascending
	// order by Index, which only includes commits on the first-parent ancestry chain, per the
	// definition of vcsinfo.IndexCommit. This does not make sense for branch == ALL_BRANCHES,
	// because different lines of history may use the same indexes. Therefore, the results of
	// RangeN for ALL_BRANCHES may not be complete or correct.
	RangeN(ctx context.Context, startIndex, endIndex int, branch string) ([]*vcsinfo.IndexCommit, error)

	// RangeByTime returns all commits in the half open time range [start, end), thus not
	// including commits at 'end' time. Set branch = ALL_BRANCHES to retrieve all commits
	// for every branch within the specified range.
	// Caveat: The returned results will match the requested range, but will be sorted by Index.
	// So if the timestamps of the commits within a branch are not in order they will be
	// unordered in the results. In the case of branch == ALL_BRANCHES, some indexes may be
	// repeated, because different lines of history may use the same indexes.
	RangeByTime(ctx context.Context, start, end time.Time, branch string) ([]*vcsinfo.IndexCommit, error)
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
