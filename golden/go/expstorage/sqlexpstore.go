package expstorage

import (
	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
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
	const stmt = `SELECT t1.name, t1.digest, t1.label
	         FROM exp_test_change AS t1
	         JOIN (
	         	SELECT name, digest, MAX(changeid) as changeid
	         	FROM exp_test_change
	         	GROUP BY name, digest ) AS t2
				ON (t1.name = t2.name AND t1.digest = t2.digest AND t1.changeid = t2.changeid)
				WHERE t1.removed IS NULL`

	rows, err := e.vdb.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer util.Close(rows)

	result := map[string]types.TestClassification{}
	for rows.Next() {
		var testName, digest, label string
		if err = rows.Scan(&testName, &digest, &label); err != nil {
			return nil, err
		}
		if _, ok := result[testName]; !ok {
			result[testName] = types.TestClassification(map[string]types.Label{})
		}
		result[testName][digest] = types.LabelFromString(label)
	}

	return &Expectations{
		Tests: result,
	}, nil
}

// See ExpectationsStore interface.
func (e *SQLExpectationsStore) AddChange(changedTests map[string]types.TestClassification, userId string) error {
	return e.AddChangeWithTimeStamp(changedTests, userId, util.TimeStampMs())
}

// TOOD(stephana): Remove the AddChangeWithTimeStamp if we remove the
// migration code that calls it.

// AddChangeWithTimeStamp adds changed tests to the database with the
// given time stamp. This is primarily for migration purposes.
func (e *SQLExpectationsStore) AddChangeWithTimeStamp(changedTests map[string]types.TestClassification, userId string, timeStamp int64) (retErr error) {
	defer timer.New("adding exp change").Stop()

	// Count the number of values to add.
	changeCount := 0
	for _, digests := range changedTests {
		changeCount += len(digests)
	}

	const (
		insertChange = `INSERT INTO exp_change (userid, ts) VALUES (?, ?)`
		insertDigest = `INSERT INTO exp_test_change (changeid, name, digest, label) VALUES`
	)

	// start a transaction
	tx, err := e.vdb.DB.Begin()
	if err != nil {
		return err
	}

	defer func() { retErr = database.CommitOrRollback(tx, retErr) }()

	// create the change record
	result, err := tx.Exec(insertChange, userId, timeStamp)
	if err != nil {
		return err
	}
	changeId, err := result.LastInsertId()
	if err != nil {
		return err
	}

	// If there are not changed records then we stop here.
	if changeCount == 0 {
		return nil
	}

	// Assemble the INSERT values.
	valuesStr := ""
	vals := []interface{}{}
	for testName, digests := range changedTests {
		for d, label := range digests {
			valuesStr += "(?, ?, ?, ?),"
			vals = append(vals, changeId, testName, d, label.String())
		}
	}
	valuesStr = valuesStr[:len(valuesStr)-1]

	// insert all the changes
	prepStmt, err := tx.Prepare(insertDigest + valuesStr)
	if err != nil {
		return err
	}
	defer util.Close(prepStmt)

	_, err = prepStmt.Exec(vals...)
	if err != nil {
		return err
	}
	return nil
}

// RemoveChange, see ExpectationsStore interface.
func (e *SQLExpectationsStore) RemoveChange(changedDigests map[string][]string) (retErr error) {
	defer timer.New("removing exp change").Stop()

	const markRemovedStmt = `UPDATE exp_test_change
	                         SET removed = IF(removed IS NULL, ?, removed)
	                         WHERE (name=?) AND (digest=?)`

	// start a transaction
	tx, err := e.vdb.DB.Begin()
	if err != nil {
		return err
	}

	defer func() { retErr = database.CommitOrRollback(tx, retErr) }()

	// Mark all the digests as removed.
	now := util.TimeStampMs()
	for testName, digests := range changedDigests {
		for _, digest := range digests {
			if _, err = tx.Exec(markRemovedStmt, now, testName, digest); err != nil {
				return err
			}
		}
	}

	return nil
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

func (c *CachingExpectationStore) RemoveChange(changedDigests map[string][]string) error {
	if err := c.store.RemoveChange(changedDigests); err != nil {
		return err
	}

	return c.cache.RemoveChange(changedDigests)
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) Changes() <-chan []string {
	return c.cache.Changes()
}
