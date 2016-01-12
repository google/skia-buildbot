package build_cache

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/skia-dev/glog"

	"go.skia.org/infra/go/buildbot"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
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

// BuildCache is a struct used for caching build data.
type BuildCache struct {
	byId     map[string]*buildbot.Build
	byCommit map[string]map[string]*buildbot.BuildSummary
	builders map[string][]*buildbot.BuilderComment
	mutex    sync.RWMutex
	db       buildbot.DB
}

// NewBuildCache creates a new BuildCache instance.
func NewBuildCache(db buildbot.DB) *BuildCache {
	return &BuildCache{db: db}
}

// LoadData loads the build data for the given commits.
func LoadData(db buildbot.DB, commits []string) (map[string]*buildbot.Build, map[string]map[string]*buildbot.BuildSummary, map[string][]*buildbot.BuilderComment, error) {
	defer timer.New("build_cache.LoadData()").Stop()
	builds, err := db.GetBuildsForCommits(commits, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	byId := map[string]*buildbot.Build{}
	byCommit := map[string]map[string]*buildbot.BuildSummary{}
	builders := map[string]bool{}
	for hash, buildList := range builds {
		byBuilder := map[string]*buildbot.BuildSummary{}
		for _, b := range buildList {
			byId[string(b.Id())] = b
			if !util.AnyMatch(BOT_BLACKLIST, b.Builder) {
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
	builderComments, err := db.GetBuildersComments(builderList)
	if err != nil {
		return nil, nil, nil, err
	}
	return byId, byCommit, builderComments, nil
}

// UpdateWithData replaces the contents of the BuildCache with the given
// data. Not intended to be used by consumers of BuildCache, but exists to
// allow for loading and storing the cache data separately so that the cache
// may be locked for the minimum amount of time.
func (c *BuildCache) UpdateWithData(byId map[string]*buildbot.Build, byCommit map[string]map[string]*buildbot.BuildSummary, builders map[string][]*buildbot.BuilderComment) {
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
		_, missingByCommit, builders, err := LoadData(c.db, missing)
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
	missingComments, err := c.db.GetBuildersComments(missingBuilderList)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to load missing builder comments: %v", err)
	}
	for b, s := range missingComments {
		builders[b] = s
	}
	return byCommit, builders, nil
}

// Get returns the build with the given ID.
func (c *BuildCache) Get(id buildbot.BuildID) (*buildbot.Build, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	b, ok := c.byId[string(id)]
	if ok {
		return b, nil
	}
	glog.Warningf("Missing build with id %v; loading now.", id)
	build, err := c.db.GetBuild(id)
	if err != nil {
		return nil, err
	}
	return build, nil
}

// AddBuilderComment adds a comment for the given builder.
func (c *BuildCache) AddBuilderComment(builder string, comment *buildbot.BuilderComment) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := c.db.PutBuilderComment(comment); err != nil {
		return err
	}
	c.builders[builder] = append(c.builders[builder], comment)
	return nil
}

// DeleteBuilderComment deletes the given comment.
func (c *BuildCache) DeleteBuilderComment(builder string, commentId int64) error {
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
	if err := c.db.DeleteBuilderComment(commentId); err != nil {
		return err
	}
	c.builders[builder] = append(c.builders[builder][:idx], c.builders[builder][idx+1:]...)
	return nil
}

// RefreshBuild reloads the given build from the DB.
func (c *BuildCache) RefreshBuild(id buildbot.BuildID) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	b, ok := c.byId[string(id)]
	if !ok {
		return fmt.Errorf("No such build %d", id)
	}
	b, err := c.db.GetBuild(id)
	if err != nil {
		return err
	}
	c.byId[string(id)] = b
	summary := b.GetSummary()
	for _, hash := range b.Commits {
		c.byCommit[hash][b.Builder] = summary
	}
	return nil
}
