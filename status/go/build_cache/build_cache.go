package build_cache

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/skia-dev/glog"

	"skia.googlesource.com/buildbot.git/go/buildbot"
)

/*
	Utilities for caching buildbot data.
*/

var (
	// Patterns indicating which bots to skip.
	BOT_BLACKLIST = []*regexp.Regexp{
		regexp.MustCompile(".*-Trybot"),
		regexp.MustCompile(".*Housekeeper.*"),
	}
)

// skipBot determines whether the given bot should be skipped.
func skipBot(b string) bool {
	for _, r := range BOT_BLACKLIST {
		if r.MatchString(b) {
			return true
		}
	}
	return false
}

// BuildCache is a struct used for caching build data.
type BuildCache struct {
	byId     map[int]*buildbot.Build
	byCommit map[string]map[string]*buildbot.BuildSummary
	builders map[string]*buildbot.Builder
	mutex    sync.RWMutex
}

// loadData loads the build data for the given commits.
func loadData(commits []string) (map[int]*buildbot.Build, map[string]map[string]*buildbot.BuildSummary, map[string]*buildbot.Builder, error) {
	builds, err := buildbot.GetBuildsForCommits(commits, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	byId := map[int]*buildbot.Build{}
	byCommit := map[string]map[string]*buildbot.BuildSummary{}
	builders := map[string]*buildbot.Builder{}
	for hash, buildList := range builds {
		byBuilder := map[string]*buildbot.BuildSummary{}
		for _, b := range buildList {
			byId[b.Id] = b
			if !skipBot(b.Builder) {
				byBuilder[b.Builder] = b.GetSummary()
				builders[b.Builder] = &buildbot.Builder{
					Name:   b.Builder,
					Master: b.Master,
				}
			}
		}
		byCommit[hash] = byBuilder
	}
	return byId, byCommit, builders, nil
}

// Update reloads build data for the given commits, throwing away any others.
func (c *BuildCache) Update(commits []string) error {
	glog.Infof("Updating build cache.")
	byId, byCommit, builders, err := loadData(commits)
	if err != nil {
		return fmt.Errorf("Failed to update build cache: %v", err)
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.byId = byId
	c.byCommit = byCommit
	c.builders = builders
	glog.Infof("Finished updating build cache.")
	return nil
}

// GetBuildsForCommits returns the build data for the given commits.
func (c *BuildCache) GetBuildsForCommits(commits []string) (map[string]map[string]*buildbot.BuildSummary, map[string]*buildbot.Builder, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	builders := c.builders
	byCommit := map[string]map[string]*buildbot.BuildSummary{}
	missing := []string{}
	for _, hash := range commits {
		builds, ok := c.byCommit[hash]
		if ok {
			byCommit[hash] = builds
		} else {
			missing = append(missing, hash)
		}
	}
	if len(missing) > 0 {
		glog.Warningf("Missing build data for some commits; loading now (%v)", missing)
		_, missingByCommit, builders, err := loadData(missing)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to load missing builds: %v", err)
		}
		for hash, byBuilder := range missingByCommit {
			byCommit[hash] = byBuilder
			for builder, build := range byBuilder {
				builders[builder] = &buildbot.Builder{
					Name:   builder,
					Master: build.Master,
				}
			}
		}
	}
	return byCommit, builders, nil
}
