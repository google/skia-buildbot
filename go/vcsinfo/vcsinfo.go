package vcsinfo

import "time"

// IndexCommit is information about a commit that includes the offset from
// the first commit.
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
	Parents   []string        `json:"parent"`
	Body      string          `json:"body"`
	Timestamp time.Time       `json:"timestamp"`
	Branches  map[string]bool `json:"-"`
}

// VCS is a generic interface to the information contained in a version
// control system.
type VCS interface {
	// Update updates the local checkout of the repo.
	Update(pull, allBranches bool) error

	// From returns commit hashes for the time frame from 'start' to now.
	From(start time.Time) []string

	// Details returns the full commit information for the given hash.
	// If includeBranchInfo is true the Branches field of the returned
	// result will contain all branches that contain the given commit,
	// otherwise Branches will be empty.
	Details(hash string, includeBranchInfo bool) (*LongCommit, error)

	// LastNIndex returns the last N commits.
	LastNIndex(N int) []*IndexCommit

	// Range returns all commits from 'begin' to 'end', exclusive of the endpoints.
	Range(begin, end time.Time) []*IndexCommit
}
