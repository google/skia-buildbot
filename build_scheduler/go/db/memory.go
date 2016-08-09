package db

import (
	"sort"
	"sync"
	"time"

	"github.com/satori/go.uuid"
	"github.com/skia-dev/glog"
)

type BuildSlice []*Build

func (s BuildSlice) Len() int { return len(s) }

func (s BuildSlice) Less(i, j int) bool {
	ts1, err := s[i].Created()
	if err != nil {
		glog.Errorf("Failed to parse CreatedTimestamp: %v", s[i])
	}
	ts2, err := s[j].Created()
	if err != nil {
		glog.Errorf("Failed to parse CreatedTimestamp: %v", s[j])
	}
	return ts1.Before(ts2)
}

func (s BuildSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

type inMemoryDB struct {
	builds    map[string]*Build
	buildsMtx sync.RWMutex
	modBuilds map[string]map[string]*Build
	modExpire map[string]time.Time
	modMtx    sync.RWMutex
}

// See docs for DB interface.
func (db *inMemoryDB) Close() error {
	return nil
}

// See docs for DB interface.
func (db *inMemoryDB) GetBuildsFromDateRange(start, end time.Time) ([]*Build, error) {
	db.buildsMtx.RLock()
	defer db.buildsMtx.RUnlock()

	rv := []*Build{}
	// TODO(borenet): Binary search.
	for _, b := range db.builds {
		created, err := b.Created()
		if err != nil {
			return nil, err
		}
		if (created.Equal(start) || created.After(start)) && created.Before(end) {
			rv = append(rv, b.Copy())
		}
	}
	sort.Sort(BuildSlice(rv))
	return rv, nil
}

// See docs for DB interface.
func (db *inMemoryDB) GetModifiedBuilds(id string) ([]*Build, error) {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	modifiedBuilds, ok := db.modBuilds[id]
	if !ok {
		return nil, ErrUnknownId
	}
	rv := make([]*Build, 0, len(modifiedBuilds))
	for _, b := range modifiedBuilds {
		rv = append(rv, b.Copy())
	}
	db.modExpire[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	db.modBuilds[id] = map[string]*Build{}
	sort.Sort(BuildSlice(rv))
	return rv, nil
}

func (db *inMemoryDB) clearExpiredModifiedUsers() {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	for id, t := range db.modExpire {
		if time.Now().After(t) {
			delete(db.modBuilds, id)
			delete(db.modExpire, id)
		}
	}
}

func (db *inMemoryDB) modify(b *Build) {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	for _, modBuilds := range db.modBuilds {
		modBuilds[b.Id] = b.Copy()
	}
}

// See docs for DB interface.
func (db *inMemoryDB) PutBuild(build *Build) error {
	db.buildsMtx.Lock()
	defer db.buildsMtx.Unlock()

	// TODO(borenet): Keep builds in a sorted slice.
	db.builds[build.Id] = build
	db.modify(build)
	return nil
}

// See docs for DB interface.
func (db *inMemoryDB) PutBuilds(builds []*Build) error {
	for _, b := range builds {
		if err := db.PutBuild(b); err != nil {
			return err
		}
	}
	return nil
}

// See docs for DB interface.
func (db *inMemoryDB) StartTrackingModifiedBuilds() (string, error) {
	db.modMtx.Lock()
	defer db.modMtx.Unlock()
	if len(db.modBuilds) >= MAX_MODIFIED_BUILDS_USERS {
		return "", ErrTooManyUsers
	}
	id := uuid.NewV5(uuid.NewV1(), uuid.NewV4().String()).String()
	db.modBuilds[id] = map[string]*Build{}
	db.modExpire[id] = time.Now().Add(MODIFIED_BUILDS_TIMEOUT)
	return id, nil
}

// NewInMemoryDB returns an extremely simple, inefficient, in-memory DB implementation.
func NewInMemoryDB() DB {
	db := &inMemoryDB{
		builds:    map[string]*Build{},
		modBuilds: map[string]map[string]*Build{},
		modExpire: map[string]time.Time{},
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			db.clearExpiredModifiedUsers()
		}
	}()
	return db
}
