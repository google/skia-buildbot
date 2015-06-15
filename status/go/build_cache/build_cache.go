package build_cache

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/timer"
)

/*
	Utilities for caching buildbot data.
*/

var (
	// Patterns indicating which bots to skip.
	BOT_BLACKLIST = []*regexp.Regexp{
		regexp.MustCompile(".*-Trybot"),
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
	builders map[string][]*buildbot.BuilderComment
	mutex    sync.RWMutex
}

// LoadData loads the build data for the given commits.
func LoadData(commits []string) (map[int]*buildbot.Build, map[string]map[string]*buildbot.BuildSummary, map[string][]*buildbot.BuilderComment, error) {
	defer timer.New("build_cache.LoadData()").Stop()
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
	builderComments, err := buildbot.GetBuildersComments(builderList)
	if err != nil {
		return nil, nil, nil, err
	}
	return byId, byCommit, builderComments, nil
}

// UpdateWithData replaces the contents of the BuildCache with the given
// data. Not intended to be used by consumers of BuildCache, but exists to
// allow for loading and storing the cache data separately so that the cache
// may be locked for the minimum amount of time.
func (c *BuildCache) UpdateWithData(byId map[int]*buildbot.Build, byCommit map[string]map[string]*buildbot.BuildSummary, builders map[string][]*buildbot.BuilderComment) {
	defer timer.New("  BuildCache locked").Stop()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.byId = byId
	c.byCommit = byCommit
	c.builders = builders
}

// GetBuildsForCommits returns the build data for the given commits.
func (c *BuildCache) GetBuildsForCommits(commits []string) (map[string]map[string]*buildbot.BuildSummary, map[string][]*buildbot.BuilderComment, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	builders := map[string][]*buildbot.BuilderComment{}
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
		_, missingByCommit, builders, err := LoadData(missing)
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
	missingComments, err := buildbot.GetBuildersComments(missingBuilderList)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to load missing builder comments: %v", err)
	}
	for b, s := range missingComments {
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

// AddBuilderComment adds a comment for the given builder.
func (c *BuildCache) AddBuilderComment(builder string, comment *buildbot.BuilderComment) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := comment.InsertIntoDB(); err != nil {
		return err
	}
	c.builders[builder] = append(c.builders[builder], comment)
	return nil
}

// DeleteBuilderComment deletes the given comment.
func (c *BuildCache) DeleteBuilderComment(builder string, commentId int) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	idx := -1
	for i, comment := range c.builders[builder] {
		if comment.Id == commentId {
			idx = i
		}
	}
	if idx == -1 {
		return fmt.Errorf("No such comment")
	}
	if err := buildbot.DeleteBuilderComment(commentId); err != nil {
		return err
	}
	c.builders[builder] = append(c.builders[builder][:idx], c.builders[builder][idx+1:]...)
	return nil
}

// UpdateBuild updates the given build, inserting it into the DB and refreshing
// it in the cache.
func (c *BuildCache) UpdateBuild(buildId int) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	b, ok := c.byId[buildId]
	if !ok {
		return fmt.Errorf("No such build %d", buildId)
	}
	if err := b.ReplaceIntoDB(); err != nil {
		return err
	}
	summary := b.GetSummary()
	for _, hash := range b.Commits {
		c.byCommit[hash][b.Builder] = summary
	}
	return nil
}
