package build_queue

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/buildbot"
)

// buildCache is a struct used as an intermediary between the buildbot
// ingestion code and the database. It implements the buildbot.DB interface
// and allows the BuildQueue to pretend to insert builds so that it can select
// the best build candidate at every step.
type buildCache struct {
	buildsByCommit map[string]*buildbot.Build
	buildsByNumber map[int]*buildbot.Build
	db             buildbot.DB
	Builder        string
	Master         string
	MaxBuildNum    int
	Repo           string
}

// See documentation for DB interface.
func (bc *buildCache) Close() error {
	return nil
}

// See documentation for DB interface.
func (bc *buildCache) BuildExists(string, string, int) (bool, error) {
	return false, fmt.Errorf("BuildExists not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetBuildsForCommits([]string, map[string]bool) (map[string][]*buildbot.Build, error) {
	return nil, fmt.Errorf("GetBuildsForCommits not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetBuild(buildbot.BuildID) (*buildbot.Build, error) {
	return nil, fmt.Errorf("GetBuild not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetBuildFromDB(master, builder string, number int) (*buildbot.Build, error) {
	return bc.getByNumber(number)
}

// See documentation for DB interface.
func (bc *buildCache) GetBuildsFromDateRange(time.Time, time.Time) ([]*buildbot.Build, error) {
	return nil, fmt.Errorf("GetBuildsFromDateRange not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetBuildNumberForCommit(master, builder, hash string) (int, error) {
	if b, ok := bc.buildsByCommit[hash]; ok {
		return b.Number, nil
	}
	// Fall back on getting the build from the database.
	b, err := bc.db.GetBuildNumberForCommit(master, builder, hash)
	if err != nil {
		return -1, fmt.Errorf("Failed to get build for %s at %s: %v", builder, hash[0:7], err)
	}
	return b, nil
}

// See documentation for DB interface.
func (bc *buildCache) GetLastProcessedBuilds(string) ([]buildbot.BuildID, error) {
	return nil, fmt.Errorf("GetLastProcessedBuilds not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetMaxBuildNumber(string, string) (int, error) {
	return -1, fmt.Errorf("GetMaxBuildNumber not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetModifiedBuilds(string) ([]*buildbot.Build, error) {
	return nil, fmt.Errorf("GetModifiedBuilds not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) StartTrackingModifiedBuilds() (string, error) {
	return "", fmt.Errorf("StartTrackingModifiedBuilds not implemented.")
}

// See documentation for DB interface.
func (bc *buildCache) GetUnfinishedBuilds(string) ([]*buildbot.Build, error) {
	return nil, fmt.Errorf("GetUnfinishedBuilds not implemented")
}

// getBuildForCommit returns a buildbot.Build instance for the build which
// included the given commit, or nil if no such build exists.
func (bc *buildCache) getBuildForCommit(hash string) (*buildbot.Build, error) {
	num, err := bc.GetBuildNumberForCommit(bc.Master, bc.Builder, hash)
	if err != nil {
		return nil, err
	}
	if num == -1 {
		return nil, nil
	}
	b, err := bc.getByNumber(num)
	if err != nil {
		return nil, fmt.Errorf("Failed to get build for %s at %s: %v", bc.Builder, hash[0:7], err)
	}
	return b, nil
}

// getByNumber returns a buildbot.Build instance for the build with the
// given number.
func (bc *buildCache) getByNumber(number int) (*buildbot.Build, error) {
	b, ok := bc.buildsByNumber[number]
	if !ok {
		b, err := bc.db.GetBuildFromDB(bc.Master, bc.Builder, number)
		if err != nil {
			return nil, err
		}
		if b != nil {
			if err := bc.PutBuild(b); err != nil {
				return nil, err
			}
		}
		return b, nil
	}
	return b, nil
}

// PutBuild inserts the given build into the buildCache so that it will be found
// when any of the getter functions are called. It does not insert the build
// into the database.
func (bc *buildCache) PutBuild(b *buildbot.Build) error {
	// Copy the build.
	build := new(buildbot.Build)
	*build = *b
	for _, c := range b.Commits {
		bc.buildsByCommit[c] = build
	}
	bc.buildsByNumber[b.Number] = build
	if build.Number > bc.MaxBuildNum {
		bc.MaxBuildNum = build.Number
	}
	return nil
}

// PutBuilds inserts all of the given builds into the buildCache.
func (bc *buildCache) PutBuilds(builds []*buildbot.Build) error {
	for _, b := range builds {
		if err := bc.PutBuild(b); err != nil {
			return err
		}
	}
	return nil
}

// See documentation for DB interface.
func (bc *buildCache) NumIngestedBuilds() (int, error) {
	return -1, fmt.Errorf("NumIngestedBuilds not implemented")
}

// newBuildCache returns a buildCache instance for the given
// builder/master/repo combination.
func newBuildCache(master, builder, repo string, db buildbot.DB) (*buildCache, error) {
	maxBuild, err := db.GetMaxBuildNumber(master, builder)
	if err != nil {
		return nil, err
	}
	return &buildCache{
		buildsByCommit: map[string]*buildbot.Build{},
		buildsByNumber: map[int]*buildbot.Build{},
		db:             db,
		Builder:        builder,
		Master:         master,
		MaxBuildNum:    maxBuild,
		Repo:           repo,
	}, nil
}

// See documentation for DB interface.
func (bc *buildCache) PutBuildComment(string, string, int, *buildbot.BuildComment) error {
	return fmt.Errorf("PutBuildComment not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) DeleteBuildComment(string, string, int, int64) error {
	return fmt.Errorf("DeleteBuildComment not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetBuilderComments(string) ([]*buildbot.BuilderComment, error) {
	return nil, fmt.Errorf("GetBuilderComments not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetBuildersComments([]string) (map[string][]*buildbot.BuilderComment, error) {
	return nil, fmt.Errorf("GetBuildersComments not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) PutBuilderComment(*buildbot.BuilderComment) error {
	return fmt.Errorf("PutBuilderComment not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) DeleteBuilderComment(int64) error {
	return fmt.Errorf("DeleteBuilderComment not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetCommitComments(string) ([]*buildbot.CommitComment, error) {
	return nil, fmt.Errorf("GetCommitComments not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) GetCommitsComments([]string) (map[string][]*buildbot.CommitComment, error) {
	return nil, fmt.Errorf("GetCommitsComments not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) PutCommitComment(*buildbot.CommitComment) error {
	return fmt.Errorf("PutCommitComment not implemented")
}

// See documentation for DB interface.
func (bc *buildCache) DeleteCommitComment(int64) error {
	return fmt.Errorf("DeleteCommitComment not implemented")
}
