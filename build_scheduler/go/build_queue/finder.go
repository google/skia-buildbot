package build_queue

import (
	"fmt"

	"go.skia.org/infra/go/buildbot"
)

// buildFinder is a struct used as an intermediary between the buildbot
// ingestion code and the database (see the BuildFinder interface in
// go.skia.org/infra/go/buildbot package). It allows the BuildQueue to
// pretend to insert builds so that it can select the best build
// candidate at every step.
type buildFinder struct {
	buildsByCommit map[string]*buildbot.Build
	buildsByNumber map[int]*buildbot.Build
	Builder        string
	Master         string
	MaxBuildNum    int
	Repo           string
}

// GetBuildForCommit returns the build number of the build which included the
// given commit, or -1 if no such build exists. It is used by the buildbot
// package's FindCommitsForBuild function.
func (bf *buildFinder) GetBuildForCommit(builder, master, hash string) (int, error) {
	if b, ok := bf.buildsByCommit[hash]; ok {
		return b.Number, nil
	}
	// Fall back on getting the build from the database.
	b, err := buildbot.GetBuildForCommit(builder, master, hash)
	if err != nil {
		return -1, fmt.Errorf("Failed to get build for %s at %s: %v", builder, hash[0:7], err)
	}
	return b, nil
}

// getBuildForCommit returns a buildbot.Build instance for the build which
// included the given commit, or nil if no such build exists.
func (bf *buildFinder) getBuildForCommit(hash string) (*buildbot.Build, error) {
	num, err := bf.GetBuildForCommit(bf.Builder, bf.Master, hash)
	if err != nil {
		return nil, err
	}
	b, err := bf.getByNumber(num)
	if err != nil {
		return nil, fmt.Errorf("Failed to get build for %s at %s: %v", bf.Builder, hash[0:7], err)
	}
	return b, nil
}

// getByNumber returns a buildbot.Build instance for the build with the
// given number.
func (bf *buildFinder) getByNumber(number int) (*buildbot.Build, error) {
	b, ok := bf.buildsByNumber[number]
	if !ok {
		b, err := buildbot.GetBuildFromDB(bf.Builder, bf.Master, number)
		if err != nil {
			return nil, err
		}
		if b != nil {
			bf.add(b)
		}
		return b, nil
	}
	return b, nil
}

// add inserts the given build into the buildFinder so that it will be found
// when any of the getter functions are called. It does not insert the build
// into the database.
func (bf *buildFinder) add(b *buildbot.Build) {
	build := &(*b) // Copy the build.
	for _, c := range b.Commits {
		bf.buildsByCommit[c] = build
	}
	bf.buildsByNumber[b.Number] = build
	if build.Number > bf.MaxBuildNum {
		bf.MaxBuildNum = build.Number
	}
}

// newBuildFinder returns a buildFinder instance for the given
// builder/master/repo combination.
func newBuildFinder(builder, master, repo string) (*buildFinder, error) {
	maxBuild, err := buildbot.GetMaxBuildNumber(builder)
	if err != nil {
		return nil, err
	}
	return &buildFinder{
		buildsByCommit: map[string]*buildbot.Build{},
		buildsByNumber: map[int]*buildbot.Build{},
		Builder:        builder,
		Master:         master,
		MaxBuildNum:    maxBuild,
		Repo:           repo,
	}, nil
}
