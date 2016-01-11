package buildbot

import (
	"fmt"
	"strings"
	"time"
)

// BuildID is a unique identifier for a Build.
type BuildID []byte

// MakeBuildID creates a BuildID from the given master name, builder name,
// and build number.
func MakeBuildID(master, builder string, number int) BuildID {
	rv := []byte(fmt.Sprintf("%s|%s|", master, builder))
	rv = append(rv, intToBytesBigEndian(int64(number))...)
	return rv
}

// ParseBuildID parses the BuildID and returns the master name, builder name,
// and build number it refers to.
func ParseBuildID(id BuildID) (string, string, int, error) {
	parts := strings.SplitN(string(id), "|", 3)
	if len(parts) != 3 {
		return "", "", -1, fmt.Errorf("Invalid build ID `%s`: Must be of the form: myMaster|myBuilder|42", string(id))
	}
	n, err := bytesToIntBigEndian([]byte(parts[2]))
	if err != nil {
		return "", "", -1, fmt.Errorf("Invalid build ID: build number is not valid: %s", err)
	}
	return parts[0], parts[1], int(n), nil
}

func (id BuildID) String() string {
	m, b, n, err := ParseBuildID(id)
	if err != nil {
		return fmt.Sprintf("<invalid BuildID: %q>", err)
	}
	return fmt.Sprintf("%s|%s|%019d", m, b, n)
}

// DB is an interface used for interacting with the buildbot database.
type DB interface {
	Close() error

	// Builds.

	// BuildExists returns true if the given build exists in the database, and false otherwise.
	BuildExists(string, string, int) (bool, error)

	// GetBuildsForCommits retrieves all builds which first included each of the
	// given commits.
	GetBuildsForCommits([]string, map[string]bool) (map[string][]*Build, error)

	// GetBuild retrieves the given build from the database.
	GetBuild(BuildID) (*Build, error)

	// GetBuildFromDB retrieves the given build from the database as specified by
	// the given master, builder, and build number.
	GetBuildFromDB(string, string, int) (*Build, error)

	// GetBuildsFromDateRange retrieves all builds which finished in the given date range.
	GetBuildsFromDateRange(time.Time, time.Time) ([]*Build, error)

	// GetBuildNumberForCommit retrieves the build number of the build which first
	// included the given commit, or -1 if no build has yet included the commit.
	GetBuildNumberForCommit(string, string, string) (int, error)

	// GetLastProcessedBuilds returns a slice of BuildIDs where each build
	// is the one with the greatest build number for its builder.
	GetLastProcessedBuilds(string) ([]BuildID, error)

	// GetMaxBuildNumber returns the highest known build number for the given builder.
	GetMaxBuildNumber(string, string) (int, error)

	// GetUnfinishedBuilds returns a slice of Builds which are not finished.
	GetUnfinishedBuilds(string) ([]*Build, error)

	// PutBuild inserts the Build in the database.
	PutBuild(*Build) error

	// PutBuilds inserts or updates the Builds in the database.
	PutBuilds([]*Build) error

	// NumIngestedBuilds returns the total number of builds which have been
	// ingested into the database.
	NumIngestedBuilds() (int, error)

	// Build comments.

	// PutBuildComment inserts the BuildComment into the database.
	PutBuildComment(string, string, int, *BuildComment) error

	// DeleteBuildComment deletes the BuildComment from the database.
	DeleteBuildComment(string, string, int, int64) error

	// Builder comments.

	// GetBuilderComments returns the comments for the given builder.
	GetBuilderComments(string) ([]*BuilderComment, error)

	// GetBuildersComments returns the comments for each of the given builders.
	GetBuildersComments([]string) (map[string][]*BuilderComment, error)

	// PutBuilderComment inserts the BuilderComment into the database.
	PutBuilderComment(*BuilderComment) error

	// DeleteBuilderComment deletes the BuilderComment from the database.
	DeleteBuilderComment(int64) error

	// Commit comments.

	// GetCommitComments returns the comments on the given commit.
	GetCommitComments(string) ([]*CommitComment, error)

	// GetCommitsComments returns the comments on each of the given commits.
	GetCommitsComments([]string) (map[string][]*CommitComment, error)

	// PutCommitComment inserts the CommitComment into the database.
	PutCommitComment(*CommitComment) error

	// DeleteCommitComment deletes the given CommitComment from the database.
	DeleteCommitComment(int64) error
}
