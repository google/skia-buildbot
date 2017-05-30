package build_cache

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"

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

const (
	// How long to keep builds in the cache.
	BUILD_EXPIRATION = 14 * 24 * time.Hour

	// Time period of builds to load at a time.
	BUILD_LOADING_CHUNK = 24 * time.Hour

	// Load all builds for this time period when starting up.
	BUILD_LOADING_PERIOD = 14 * 24 * time.Hour
)

// BuildCache is a struct used for caching build data.
type BuildCache struct {
	byId            map[string]*buildbot.Build
	byCommit        map[string]map[string]*buildbot.BuildSummary
	byTime          *TimeRangeTree
	builders        map[string]bool
	builderComments map[string][]*buildbot.BuilderComment
	lastLoad        time.Time
	mutex           sync.RWMutex
	db              buildbot.DB
	dbId            string
}

// NewBuildCache creates a new BuildCache instance.
func NewBuildCache(db buildbot.DB) (*BuildCache, error) {
	// Start tracking build changes in the DB.
	dbId, err := db.StartTrackingModifiedBuilds()
	if err != nil {
		return nil, err
	}
	bc := &BuildCache{
		builders:        map[string]bool{},
		builderComments: map[string][]*buildbot.BuilderComment{},
		byId:            map[string]*buildbot.Build{},
		byCommit:        map[string]map[string]*buildbot.BuildSummary{},
		byTime:          NewTimeRangeTree(),
		lastLoad:        time.Now(),
		db:              db,
		dbId:            dbId,
	}
	// Populate the cache with data.
	to := time.Now()
	from := to.Add(BUILD_LOADING_CHUNK)
	for time.Now().Sub(from) < BUILD_LOADING_PERIOD {
		sklog.Infof("Loading builds from %s to %s", from, to)
		builds, err := db.GetBuildsFromDateRange(from, to)
		if err != nil {
			return nil, err
		}
		if err := bc.updateWithBuilds(builds); err != nil {
			return nil, err
		}
		to = from
		from = to.Add(-BUILD_LOADING_CHUNK)
	}
	if err := bc.update(); err != nil {
		return nil, err
	}
	go func() {
		for range time.Tick(time.Minute) {
			if err := bc.update(); err != nil {
				sklog.Error(err)
			}
		}
	}()
	return bc, nil
}

// update obtains the set of modified builds since the last update
// and inserts them into the cache.
func (c *BuildCache) update() error {
	builds, err := c.db.GetModifiedBuilds(c.dbId)
	if err != nil {
		if time.Now().Sub(c.lastLoad) >= 10*time.Minute {
			sklog.Errorf("Failed to GetModifiedBuilds. Attempting to re-establish connection to database.")
			id, err := c.db.StartTrackingModifiedBuilds()
			if err != nil {
				return err
			}
			c.dbId = id
			b1, err := c.db.GetBuildsFromDateRange(c.lastLoad, time.Now())
			if err != nil {
				return err
			}
			b2, err := c.db.GetModifiedBuilds(c.dbId)
			if err != nil {
				return err
			}
			builds = append(b1, b2...)
			sklog.Errorf("Re-connected successfully.")
		} else {
			return err
		}
	}
	if err := c.updateWithBuilds(builds); err != nil {
		return err
	}
	c.evictExpiredBuilds()
	return c.updateBuilderComments()
}

// updateBuilderComments updates the comments for all builders.
func (c *BuildCache) updateBuilderComments() error {
	defer timer.New("BuildCache.updateBuilderComments").Stop()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	builderList := make([]string, 0, len(c.builders))
	for b := range c.builders {
		builderList = append(builderList, b)
	}
	builderComments, err := c.db.GetBuildersComments(builderList)
	if err != nil {
		return err
	}
	c.builderComments = builderComments
	return nil
}

// insert adds the given build to the cache. Assumes the caller holds a lock.
func (c *BuildCache) insert(b *buildbot.Build) {
	idStr := string(b.Id())
	c.byId[idStr] = b
	summary := b.GetSummary()
	for _, h := range b.Commits {
		if _, ok := c.byCommit[h]; !ok {
			c.byCommit[h] = map[string]*buildbot.BuildSummary{}
		}
		c.byCommit[h][b.Builder] = summary
	}
	if b.IsFinished() {
		c.byTime.Insert(b.Finished, idStr)
	}
	c.builders[b.Builder] = true
}

// get retrieves the given build from the cache. Assumes the caller holds a lock.
func (c *BuildCache) get(id string) *buildbot.Build {
	return c.byId[id]
}

// delete removes the given build from the cache. Assumes the caller holds a lock.
func (c *BuildCache) delete(id string) {
	b := c.byId[id]
	for _, h := range b.Commits {
		if _, ok := c.byCommit[h][b.Builder]; ok {
			delete(c.byCommit[h], b.Builder)
		}
	}
	c.byTime.Delete(b.Finished, id)
	delete(c.byId, id)
}

// evictExpiredBuilds removes builds which have expired from the cache.
func (c *BuildCache) evictExpiredBuilds() {
	defer timer.New("BuildCache.evictExpiredBuilds").Stop()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	expiredBefore := time.Now().Add(-BUILD_EXPIRATION)
	expiredIds := c.byTime.GetRange(util.TimeUnixZero, expiredBefore)
	for _, id := range expiredIds {
		c.delete(id)
	}
	sklog.Infof("Deleted %d expired builds.", len(expiredIds))
}

// updateWithBuilds inserts the given builds into the cache.
func (c *BuildCache) updateWithBuilds(builds []*buildbot.Build) error {
	defer timer.New("  BuildCache locked").Stop()
	c.mutex.Lock()
	defer c.mutex.Unlock()
	sklog.Infof("Inserting %d builds.", len(builds))
	for _, b := range builds {
		idStr := string(b.Id())
		if c.get(idStr) != nil {
			c.delete(idStr)
		}
		c.insert(b)
	}
	return nil
}

// GetBuildsForCommits returns the build data for the given commits.
func (c *BuildCache) GetBuildsForCommits(commits []string) (map[string]map[string]*buildbot.BuildSummary, error) {
	defer timer.New("BuildCache.GetBuildsForCommits").Stop()
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	byCommit := map[string]map[string]*buildbot.BuildSummary{}
	for _, hash := range commits {
		builds, ok := c.byCommit[hash]
		if ok {
			cpyBuilds := map[string]*buildbot.BuildSummary{}
			for k, v := range builds {
				cpyBuilds[k] = v
			}
			byCommit[hash] = cpyBuilds
		}
	}
	return byCommit, nil
}

// GetBuildsFromDateRange returns builds within the given date range.
func (c *BuildCache) GetBuildsFromDateRange(from, to time.Time) ([]*buildbot.Build, error) {
	defer timer.New("BuildCache.GetBuildsFromDateRange").Stop()
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	ids := c.byTime.GetRange(from, to)
	rv := make([]*buildbot.Build, 0, len(ids))
	for _, id := range ids {
		rv = append(rv, c.byId[id])
	}
	return rv, nil
}

// GetBuildersComments returns comments for all builders.
func (c *BuildCache) GetBuildersComments() map[string][]*buildbot.BuilderComment {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	rv := map[string][]*buildbot.BuilderComment{}
	for k, v := range c.builderComments {
		cpy := make([]*buildbot.BuilderComment, len(v))
		copy(cpy, v)
		rv[k] = cpy
	}
	return rv
}

// AddBuilderComment adds a comment for the given builder.
func (c *BuildCache) AddBuilderComment(builder string, comment *buildbot.BuilderComment) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := c.db.PutBuilderComment(comment); err != nil {
		return err
	}
	newComments, err := c.db.GetBuilderComments(builder)
	if err != nil {
		return err
	}
	c.builderComments[builder] = newComments
	return nil
}

// DeleteBuilderComment deletes the given comment.
func (c *BuildCache) DeleteBuilderComment(builder string, commentId int64) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if err := c.db.DeleteBuilderComment(commentId); err != nil {
		return err
	}
	newComments, err := c.db.GetBuilderComments(builder)
	if err != nil {
		return err
	}
	c.builderComments[builder] = newComments
	return nil
}

// refreshBuild reloads the given build from the DB.
func (c *BuildCache) refreshBuild(id buildbot.BuildID) error {
	b, err := c.db.GetBuild(id)
	if err != nil {
		return err
	}
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.delete(string(b.Id()))
	c.insert(b)
	return nil
}

// AddBuildComment adds the given comment to the given build.
func (c *BuildCache) AddBuildComment(master, builder string, number int, comment *buildbot.BuildComment) error {
	if err := c.db.PutBuildComment(master, builder, number, comment); err != nil {
		return fmt.Errorf("Failed to add comment: %s", err)
	}
	return c.refreshBuild(buildbot.MakeBuildID(master, builder, number))
}

// DeleteBuildComment deletes the given comment from the given build.
func (c *BuildCache) DeleteBuildComment(master, builder string, number int, commentId int64) error {
	if err := c.db.DeleteBuildComment(master, builder, number, commentId); err != nil {
		return fmt.Errorf("Failed to delete comment: %s", err)
	}
	return c.refreshBuild(buildbot.MakeBuildID(master, builder, number))
}

// GetBuildsForCommit returns the builds which ran at the given commit.
func (c *BuildCache) GetBuildsForCommit(hash string) ([]*buildbot.BuildSummary, error) {
	builds, err := c.GetBuildsForCommits([]string{hash})
	if err != nil {
		return nil, fmt.Errorf("Failed to get build data for commit: %v", err)
	}
	rv := make([]*buildbot.BuildSummary, 0, len(builds[hash]))
	for _, b := range builds[hash] {
		rv = append(rv, b)
	}
	return rv, nil
}
