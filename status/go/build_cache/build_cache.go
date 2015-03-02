package build_cache

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/buildbot"
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
	builders map[string]*buildbot.BuilderStatus
	commits  []string
	mutex    sync.RWMutex
}

// loadData loads the build data for the given commits.
func loadData(commits []string) (map[int]*buildbot.Build, map[string]map[string]*buildbot.BuildSummary, map[string]*buildbot.BuilderStatus, error) {
	builds, err := buildbot.GetBuildsForCommits(commits, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	byId := map[int]*buildbot.Build{}
	byCommit := map[string]map[string]*buildbot.BuildSummary{}
	builders := map[string]bool{}
	for hash, buildList := range builds {
		byBuilder := map[string]*buildbot.BuildSummary{}
		for _, b := range buildList {
			byId[b.Id] = b
			if !skipBot(b.Builder) {
				byBuilder[b.Builder] = b.GetSummary()
				builders[b.Builder] = true
			}
		}
		byCommit[hash] = byBuilder
	}
	builderList := make([]string, 0, len(builders))
	for b, _ := range builders {
		builderList = append(builderList, b)
	}
	builderStatuses, err := buildbot.GetBuilderStatuses(builderList)
	if err != nil {
		return nil, nil, nil, err
	}
	return byId, byCommit, builderStatuses, nil
}

// Update reloads build data for the same set of commits as before.
func (c *BuildCache) Update() error {
	glog.Infof("Updating build cache.")
	byId, byCommit, builders, err := loadData(c.commits)
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

// UpdateForCommits reloads build data for the given commits, throwing away any
// others.
func (c *BuildCache) UpdateForCommits(commits []string) error {
	if commits == nil {
		commits = []string{}
	}
	c.commits = commits
	return c.Update()
}

// GetBuildsForCommits returns the build data for the given commits.
func (c *BuildCache) GetBuildsForCommits(commits []string) (map[string]map[string]*buildbot.BuildSummary, map[string]*buildbot.BuilderStatus, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	builders := map[string]*buildbot.BuilderStatus{}
	for k, v := range c.builders {
		builders[k] = v
	}
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
	missingBuilders := map[string]bool{}
	if len(missing) > 0 {
		glog.Warningf("Missing build data for some commits; loading now (%v)", missing)
		_, missingByCommit, builders, err := loadData(missing)
		if err != nil {
			return nil, nil, fmt.Errorf("Failed to load missing builds: %v", err)
		}
		for hash, byBuilder := range missingByCommit {
			byCommit[hash] = byBuilder
			for builder, _ := range byBuilder {
				if _, ok := builders[builder]; !ok {
					missingBuilders[builder] = true
				}
			}
		}
	}
	missingBuilderList := make([]string, 0, len(missingBuilders))
	for b, _ := range missingBuilders {
		missingBuilderList = append(missingBuilderList, b)
	}
	missingStatuses, err := buildbot.GetBuilderStatuses(missingBuilderList)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to load missing builder statuses: %v", err)
	}
	for b, s := range missingStatuses {
		builders[b] = s
	}
	return byCommit, builders, nil
}

// Get returns the build with the given ID.
func (c *BuildCache) Get(id int) (*buildbot.Build, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	b, ok := c.byId[id]
	if ok {
		return b, nil
	}
	glog.Warningf("Missing build with id %d; loading now.", id)
	builds, err := buildbot.GetBuildsFromDB([]int{id})
	if err != nil {
		return nil, err
	}
	if b, ok := builds[id]; ok {
		return b, nil
	}
	return nil, fmt.Errorf("No such build: %d", id)
}
