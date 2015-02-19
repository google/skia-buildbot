package expstorage

import (
	"database/sql"
	"encoding/json"

	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/util"
	"skia.googlesource.com/buildbot.git/golden/go/types"
)

// Stores expectations in an SQL database without any caching.
type SQLExpectationsStore struct {
	vdb *database.VersionedDB
}

func NewSQLExpectationStore(vdb *database.VersionedDB) ExpectationsStore {
	return &SQLExpectationsStore{
		vdb: vdb,
	}
}

// See ExpectationsStore interface.
func (e *SQLExpectationsStore) Get() (exp *Expectations, err error) {
	// Load the newest record from the database.
	stmt := `SELECT expectations
	         FROM expectations
	         ORDER BY ts DESC
	         LIMIT 1`

	// Read the expectations. If there are no rows, that means we have no
	// expectations yet.
	var expJSON string
	switch err := e.vdb.DB.QueryRow(stmt).Scan(&expJSON); {
	case err == sql.ErrNoRows:
		return NewExpectations(), nil
	case err != nil:
		return nil, err
	}

	var result Expectations
	err = json.Unmarshal([]byte(expJSON), &result)

	return &result, nil
}

// See ExpectationsStore interface.
func (e *SQLExpectationsStore) AddChange(changedTests map[string]types.TestClassification, userId string) error {
	exp, err := e.Get()
	if err != nil {
		return err
	}
	exp.AddDigests(changedTests)

	// Sererialize the expectations to JSON and store them in the DB.
	expectationsJSON, err := json.Marshal(exp)
	if err != nil {
		return err
	}
	ts := util.TimeStampMs()

	stmt := `INSERT INTO expectations (userid, ts, expectations)
	         VALUES(?, ?, ?)`
	_, err = e.vdb.DB.Exec(stmt, userId, ts, expectationsJSON)
	return err
}

// See ExpectationsStore interface.
func (e *SQLExpectationsStore) Changes() <-chan []string {
	glog.Fatal("SQLExpectationsStore doesn't really support Changes.")
	return nil
}

// Wraps around an ExpectationsStore and caches the expectations using
// MemExpecationsStore.
type CachingExpectationStore struct {
	store   ExpectationsStore
	cache   ExpectationsStore
	refresh bool
}

func NewCachingExpectationStore(store ExpectationsStore) ExpectationsStore {
	return &CachingExpectationStore{
		store:   store,
		cache:   NewMemExpectationsStore(),
		refresh: true,
	}
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) Get() (exp *Expectations, err error) {
	if c.refresh {
		c.refresh = false
		tempExp, err := c.store.Get()
		if err != nil {
			return nil, err
		}
		if err = c.cache.AddChange(tempExp.Tests, ""); err != nil {
			return nil, err
		}
	}
	return c.cache.Get()
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) AddChange(changedTests map[string]types.TestClassification, userId string) error {
	if err := c.store.AddChange(changedTests, userId); err != nil {
		return err
	}

	return c.cache.AddChange(changedTests, userId)
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) Changes() <-chan []string {
	return c.cache.Changes()
}
