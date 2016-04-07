package buildbot

import (
	"fmt"
	"sync"
	"time"

	"github.com/skia-dev/glog"
)

// ingestCache is an implementation of the DB interface used for inserting
// builds into the database in batches as opposed to one at a time. It provides
// a layer so that builds which have not yet been inserted into the database may
// still be found by query functions. Only the DB interface functions needed for
// ingestion are implemented; the others return an error.
type ingestCache struct {
	buildNumsByCommit map[string]map[string]map[string]int
	builds            map[string]*Build
	db                *localDB
	maxBuildNums      map[string]map[string]int
	mtx               sync.RWMutex
}

// newIngestCache returns an ingestCache instance and starts a goroutine which
// periodically inserts builds into the database.
func newIngestCache(db *localDB) *ingestCache {
	c := &ingestCache{
		buildNumsByCommit: map[string]map[string]map[string]int{},
		builds:            map[string]*Build{},
		db:                db,
		maxBuildNums:      map[string]map[string]int{},
		mtx:               sync.RWMutex{},
	}
	go func() {
		for _ = range time.Tick(time.Second) {
			if err := c.insertBuilds(); err != nil {
				glog.Errorf("Failed to insert builds: %s", err)
			}
		}
	}()
	return c
}

// insertBuilds inserts all builds in the cache into the database.
func (c *ingestCache) insertBuilds() error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if len(c.builds) == 0 {
		return nil
	}
	glog.Infof("Inserting %d builds.", len(c.builds))
	builds := make([]*Build, 0, len(c.builds))
	for _, b := range c.builds {
		builds = append(builds, b)
	}
	if err := c.db.PutBuilds(builds); err != nil {
		return err
	}
	// Empty the cache.
	c.buildNumsByCommit = map[string]map[string]map[string]int{}
	c.builds = map[string]*Build{}
	c.maxBuildNums = map[string]map[string]int{}
	return nil
}

// See documentation for DB interface.
func (c *ingestCache) GetBuild(id BuildID) (*Build, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	b, ok := c.builds[string(id)]
	if !ok {
		return c.db.GetBuild(id)
	}
	return b, nil
}

// See documentation for DB interface.
func (c *ingestCache) GetBuildFromDB(master, builder string, number int) (*Build, error) {
	return c.GetBuild(MakeBuildID(master, builder, number))
}

// See documentation for DB interface.
func (c *ingestCache) GetBuildNumberForCommit(master, builder, commit string) (int, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	m, ok := c.buildNumsByCommit[master]
	if !ok {
		return c.db.GetBuildNumberForCommit(master, builder, commit)
	}
	b, ok := m[builder]
	if !ok {
		return c.db.GetBuildNumberForCommit(master, builder, commit)
	}
	n, ok := b[commit]
	if !ok {
		return c.db.GetBuildNumberForCommit(master, builder, commit)
	}
	return n, nil
}

// See documentation for DB interface.
func (c *ingestCache) GetLastProcessedBuilds(master string) ([]BuildID, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	ids, err := c.db.GetLastProcessedBuilds(master)
	if err != nil {
		return nil, err
	}
	rv := make([]BuildID, 0, len(ids))
	for _, id := range ids {
		m, b, n, err := ParseBuildID(id)
		if err != nil {
			return nil, err
		}
		if _, ok := c.maxBuildNums[m]; !ok {
			rv = append(rv, id)
			continue
		}
		max, ok := c.maxBuildNums[m][b]
		if !ok {
			rv = append(rv, id)
			continue
		}
		if max > n {
			rv = append(rv, MakeBuildID(m, b, max))
		} else {
			rv = append(rv, id)
		}
	}
	return rv, nil
}

// See documentation for DB interface.
func (c *ingestCache) GetUnfinishedBuilds(master string) ([]*Build, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	builds, err := c.db.GetUnfinishedBuilds(master)
	if err != nil {
		return nil, err
	}
	unfinished := map[string]*Build{}
	for _, b := range builds {
		if updated, ok := c.builds[string(b.Id())]; ok {
			b = updated
		}
		if !b.IsFinished() {
			unfinished[string(b.Id())] = b
		}
	}
	for _, b := range c.builds {
		if !b.IsFinished() && b.Master == master {
			unfinished[string(b.Id())] = b
		}
	}
	rv := make([]*Build, 0, len(unfinished))
	for _, b := range unfinished {
		rv = append(rv, b)
	}
	return rv, nil
}

// See documentation for DB interface.
func (c *ingestCache) putBuild_Locked(b *Build) error {
	c.builds[string(b.Id())] = b

	// by commit
	if _, ok := c.buildNumsByCommit[b.Master]; !ok {
		c.buildNumsByCommit[b.Master] = map[string]map[string]int{}
	}
	if _, ok := c.buildNumsByCommit[b.Master][b.Builder]; !ok {
		c.buildNumsByCommit[b.Master][b.Builder] = map[string]int{}
	}
	for _, commit := range b.Commits {
		c.buildNumsByCommit[b.Master][b.Builder][commit] = b.Number
	}

	// max build numbers
	if _, ok := c.maxBuildNums[b.Master]; !ok {
		c.maxBuildNums[b.Master] = map[string]int{}
	}
	if n, ok := c.maxBuildNums[b.Master][b.Builder]; !ok || n < b.Number {
		c.maxBuildNums[b.Master][b.Builder] = n
	}
	return nil
}

// See documentation for DB interface.
func (c *ingestCache) PutBuild(b *Build) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	return c.putBuild_Locked(b)
}

// See documentation for DB interface.
func (c *ingestCache) PutBuilds(builds []*Build) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	for _, b := range builds {
		if err := c.putBuild_Locked(b); err != nil {
			return err
		}
	}
	return nil
}

// The remaining functions are unimplemented.

func notImplemented(fn string) error {
	return fmt.Errorf("%q not implemented for ingestCache!", fn)
}

// See documentation for DB interface.
func (c *ingestCache) Close() error {
	return notImplemented("Close")
}

// See documentation for DB interface.
func (c *ingestCache) BuildExists(string, string, int) (bool, error) {
	return false, notImplemented("BuildExists")
}

// See documentation for DB interface.
func (c *ingestCache) GetBuildsForCommits([]string, map[string]bool) (map[string][]*Build, error) {
	return nil, notImplemented("GetBuildsForCommits")
}

// See documentation for DB interface.
func (c *ingestCache) GetBuildsFromDateRange(time.Time, time.Time) ([]*Build, error) {
	return nil, notImplemented("GetBuildsFromDateRange")
}

// See documentation for DB interface.
func (c *ingestCache) GetMaxBuildNumber(string, string) (int, error) {
	return -1, notImplemented("GetMaxBuildNumber")
}

// See documentation for DB interface.
func (c *ingestCache) GetModifiedBuilds(string) ([]*Build, error) {
	return nil, notImplemented("GetModifiedBuilds")
}

// See documentation for DB interface.
func (c *ingestCache) StartTrackingModifiedBuilds() (string, error) {
	return "", notImplemented("StartTrackingModifiedBuilds")
}

// See documentation for DB interface.
func (c *ingestCache) NumIngestedBuilds() (int, error) {
	return -1, notImplemented("NumIngestedBuilds")
}

// See documentation for DB interface.
func (c *ingestCache) PutBuildComment(string, string, int, *BuildComment) error {
	return notImplemented("PutBuildComment")
}

// See documentation for DB interface.
func (c *ingestCache) DeleteBuildComment(string, string, int, int64) error {
	return notImplemented("DeleteBuildComment")
}

// See documentation for DB interface.
func (c *ingestCache) GetBuilderComments(string) ([]*BuilderComment, error) {
	return nil, notImplemented("GetBuilderComments")
}

// See documentation for DB interface.
func (c *ingestCache) GetBuildersComments([]string) (map[string][]*BuilderComment, error) {
	return nil, notImplemented("GetBuildersComments")
}

// See documentation for DB interface.
func (c *ingestCache) PutBuilderComment(*BuilderComment) error {
	return notImplemented("PutBuilderComment")
}

// See documentation for DB interface.
func (c *ingestCache) DeleteBuilderComment(int64) error {
	return notImplemented("DeleteBuilderComment")
}

// See documentation for DB interface.
func (c *ingestCache) GetCommitComments(string) ([]*CommitComment, error) {
	return nil, notImplemented("GetCommitComments")
}

// See documentation for DB interface.
func (c *ingestCache) GetCommitsComments([]string) (map[string][]*CommitComment, error) {
	return nil, notImplemented("GetCommitsComments")
}

// See documentation for DB interface.
func (c *ingestCache) PutCommitComment(*CommitComment) error {
	return notImplemented("PutCommitComment")
}

// See documentation for DB interface.
func (c *ingestCache) DeleteCommitComment(int64) error {
	return notImplemented("DeleteCommitComment")
}
