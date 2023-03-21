// Package provider contains types and interfaces for interacting with Git
// repos.
package provider

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/human"
	"go.skia.org/infra/perf/go/types"
)

// Commit represents a single commit stored in the database.
//
// JSON annotations make it serialize like the legacy cid.CommitDetail.
type Commit struct {
	CommitNumber types.CommitNumber `json:"offset"`
	GitHash      string             `json:"hash"`
	Timestamp    int64              `json:"ts"` // Unix timestamp, seconds from the epoch.
	Author       string             `json:"author"`
	Subject      string             `json:"message"`
	URL          string             `json:"url"`
}

// Display returns a display string that describes the commit.
func (c Commit) Display(now time.Time) string {
	return fmt.Sprintf("%s - %s - %s", c.GitHash[:7], human.Duration(now.Sub(time.Unix(c.Timestamp, 0))), c.Subject)
}

// CommitProcessor is a callback function that will be called with a Commit.
// Used in GitProvider.
type CommitProcessor func(c Commit) error

// Provider in abstraction of how we get information about a repo. This could
// be implemented by either Git or the Gitiles API.
type Provider interface {
	// CommitsFromMostRecentGitHashToHead will call the `cb` func with every
	// Commit, starting from the oldest and going to the newest. If
	// mostRecentGitHash is the empty string then the commits will start with
	// the very first commit to the repo, or from the start commit if one is
	// provided.
	CommitsFromMostRecentGitHashToHead(ctx context.Context, mostRecentGitHash string, cb CommitProcessor) error

	// GitHashesInRangeForFile returns all the git hashes when the given file
	// has changed between [begin, end], i.e. the given range is exclusive of
	// the begin commit and inclusive of the end commit. If 'begin' is the empty
	// string then the scan should go back to the initial commit of the repo.
	GitHashesInRangeForFile(ctx context.Context, begin, end, filename string) ([]string, error)

	// LogEntry returns the full log entry of a commit (minus the diff) as a string.
	LogEntry(ctx context.Context, gitHash string) (string, error)

	// Update does any necessary work, like a `git pull`, to ensure that the
	// GitProvider has the most recent commits available.
	Update(ctx context.Context) error
}
