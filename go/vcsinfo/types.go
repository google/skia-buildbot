package vcsinfo

import (
	"context"
	"time"
)

var (
	// Minimum time (the epoch)
	MinTime = time.Unix(0, 0)

	// MaxTime is the maximum time we consider. It's the equivalent of approximately November 2286.
	// It is intended to be used as a value for range queries to get everything
	// after a specified start time.
	MaxTime = time.Unix(9999999999, 0)
)

// IndexCommit is information about a commit that includes the commit's index
// in the linear ancestry path obtained by following each commit's first parent
// backward in history. The first commit in a given branch has Index 0.
type IndexCommit struct {
	Hash      string
	Index     int
	Timestamp time.Time
}

// ShortCommit stores the hash, author, and subject of a git commit.
type ShortCommit struct {
	Hash    string `json:"hash"`
	Author  string `json:"author"`
	Subject string `json:"subject"`
}

// LongCommit gives more detailed information about a commit.
type LongCommit struct {
	*ShortCommit
	Parents   []string  `json:"parent"`
	Body      string    `json:"body"`
	Timestamp time.Time `json:"timestamp"`
	// Index is this Commit's index in the linear ancestry path obtained by
	// following each commit's first parent backward in history. The first
	// commit in a given branch has Index 0. This field is not set by
	// default.
	Index int `json:"-"`
	// Branches indicates which branches can reach this commit.
	Branches map[string]bool `json:"-"`
}

func NewLongCommit() *LongCommit {
	return &LongCommit{ShortCommit: &ShortCommit{}}
}

func (c *LongCommit) IndexCommit() *IndexCommit {
	return &IndexCommit{
		Hash:      c.Hash,
		Index:     c.Index,
		Timestamp: c.Timestamp,
	}
}

// LongCommitSlice represents a slice of LongCommit objects used for sorting
// commits by timestamp, most recent first.
type LongCommitSlice []*LongCommit

func (s LongCommitSlice) Len() int           { return len(s) }
func (s LongCommitSlice) Less(i, j int) bool { return s[i].Timestamp.After(s[j].Timestamp) }
func (s LongCommitSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// IndexCommitSlice represents a slice of IndexCommit objects used for sorting
// commits by index, then by timestamp, then by hash.
type IndexCommitSlice []*IndexCommit

func (s IndexCommitSlice) Len() int { return len(s) }
func (s IndexCommitSlice) Less(i, j int) bool {
	return s[i].Index < s[j].Index ||
		((s[i].Index == s[j].Index) && s[i].Timestamp.Before(s[j].Timestamp)) ||
		((s[i].Index == s[j].Index) && s[i].Timestamp.Equal(s[j].Timestamp) &&
			(s[i].Hash < s[j].Hash))
}
func (s IndexCommitSlice) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// VCS is a generic interface to the information contained in a version
// control system.
type VCS interface {
	// Update updates the local checkout of the repo.
	Update(ctx context.Context, pull, allBranches bool) error

	// From returns commit hashes for the time frame from 'start' to now.
	From(start time.Time) []string

	// Details returns the full commit information for the given hash.
	// If includeBranchInfo is true the Branches field of the returned
	// result will contain all branches that contain the given commit,
	// otherwise Branches will be empty.
	// Note: Retrieving the branch information can be expensive and should
	// only be used if the membership in branches is really needed.
	Details(ctx context.Context, hash string, includeBranchInfo bool) (*LongCommit, error)

	// DetailsMulti returns multiple details at once, which is a lot faster for some implementations,
	// e.g. the implementation based on BigTable where we can avoid multiple roundtrips to the database.
	DetailsMulti(ctx context.Context, hashes []string, includeBranchInfo bool) ([]*LongCommit, error)

	// LastNIndex returns the last N commits.
	LastNIndex(N int) []*IndexCommit

	// Range returns all commits from the half open interval ['begin', 'end'), i.e.
	// includes 'begin' and excludes 'end'.
	Range(begin, end time.Time) []*IndexCommit

	// IndexOf returns the index of the commit hash, where 0 is the index of the first commit.
	IndexOf(ctx context.Context, hash string) (int, error)

	// ByIndex returns a LongCommit describing the commit
	// at position N, as ordered in the current branch.
	ByIndex(ctx context.Context, N int) (*LongCommit, error)
}
