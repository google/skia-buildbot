package vcsinfo

import "time"

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
}

// VCS is a generic interface to the information contained in a version
// control system.
type VCS interface {
	// Update updates the local checkout of the repo.
	Update(pull, allBranches bool) error

	// From returns commit hashes for the time frame from 'start' to now.
	From(start time.Time) []string

	// Details returns the full commit information for the given hash.
	Details(hash string) (*LongCommit, error)
}
