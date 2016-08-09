package db

import (
	"sync"
	"time"
)

type BuildCache struct {
	builds         map[string]*Build
	buildsByCommit map[string]map[string]*Build
	db             DB
	lastUpdate     time.Time
	mtx            sync.RWMutex
	queryId        string
}

// GetBuildsForCommits retrieves all builds which included[1] each of the
// given commits. Returns a map whose keys are commit hashes and values are
// sub-maps whose keys are builder names and values are builds.
//
// 1) Blamelist calculation is outside the scope of the BuildCache, but the
//    implied assumption here is that there is at most one build for each
//    builder which has a given commit in its blamelist. The user is responsible
//    for inserting builds into the database so that this invariant is
//    maintained. Generally, a more recent build will "steal" commits from an
//    earlier build's blamelist, if the blamelists overlap. There are three
//    cases to consider:
//       1. The newer build ran at a newer commit than the older build. Its
//          blamelist consists of all commits not covered by the previous build,
//          and therefore does not overlap with the older build's blamelist.
//       2. The newer build ran at the same commit as the older build. Its
//          blamelist is the same as the previous build's blamelist, and
//          therefore it "steals" all commits from the previous build, whose
//          blamelist becomes empty.
//       3. The newer build ran at a commit which was in the previous build's
//          blamelist. Its blamelist consists of the commits in the previous
//          build's blamelist which it also covered. Those commits move out of
//          the previous build's blamelist and into the newer build's blamelist.
func (c *BuildCache) GetBuildsForCommits(commits []string) (map[string]map[string]*Build, error) {
	c.mtx.RLock()
	defer c.mtx.RUnlock()

	rv := make(map[string]map[string]*Build, len(commits))
	for _, commit := range commits {
		if builds, ok := c.buildsByCommit[commit]; ok {
			rv[commit] = make(map[string]*Build, len(builds))
			for k, v := range builds {
				rv[commit][k] = v.Copy()
			}
		} else {
			rv[commit] = map[string]*Build{}
		}
	}
	return rv, nil
}

// update inserts the new/updated builds into the cache. Assumes the caller
// holds a lock.
func (c *BuildCache) update(builds []*Build) error {
	for _, b := range builds {
		// If we already know about this build, the blamelist might,
		// have changed, so we need to remove it from buildsByCommit
		// and re-insert where needed.
		if old, ok := c.builds[b.Id]; ok {
			for _, commit := range old.Commits {
				delete(c.buildsByCommit[commit], b.Builder)
			}
		}

		// Insert the new build into the main map.
		c.builds[b.Id] = b.Copy()

		// Insert the build into buildsByCommits.
		for _, commit := range b.Commits {
			if _, ok := c.buildsByCommit[commit]; !ok {
				c.buildsByCommit[commit] = map[string]*Build{}
			}
			c.buildsByCommit[commit][b.Builder] = c.builds[b.Id]
		}
	}
	return nil
}

// Load new builds from the database.
func (c *BuildCache) Update() error {
	now := time.Now()
	newBuilds, err := c.db.GetModifiedBuilds(c.queryId)
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if err != nil {
		if err.Error() == ErrUnknownId.Error() {
			// The database may have restarted. Attempt to re-establish connection.
			queryId, err := c.db.StartTrackingModifiedBuilds()
			if err != nil {
				return err
			}
			c.queryId = queryId
			// We may have missed something. Query for builds since the last
			// successful query.
			builds, err := c.db.GetBuildsFromDateRange(c.lastUpdate, now)
			if err != nil {
				return err
			}
			if err := c.update(builds); err == nil {
				c.lastUpdate = now
				return nil
			} else {
				return err
			}
		} else {
			return err
		}
	}
	if err := c.update(newBuilds); err == nil {
		c.lastUpdate = now
		return nil
	} else {
		return err
	}
}

// NewBuildCache returns a local cache which provides more convenient views of
// build data than the database can provide.
func NewBuildCache(db DB, timePeriod time.Duration) (*BuildCache, error) {
	queryId, err := db.StartTrackingModifiedBuilds()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	start := now.Add(-timePeriod)
	builds, err := db.GetBuildsFromDateRange(start, now)
	if err != nil {
		return nil, err
	}
	bc := &BuildCache{
		builds:         map[string]*Build{},
		buildsByCommit: map[string]map[string]*Build{},
		db:             db,
		lastUpdate:     now,
		queryId:        queryId,
	}
	if err := bc.update(builds); err != nil {
		return nil, err
	}
	return bc, nil
}
