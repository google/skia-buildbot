package expstorage

import (
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/eventbus"
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
func (m *SQLExpectationsStore) QueryLog(offset, size int) ([]*TriageLogEntry, int, error) {
	const stmtList = `SELECT ec.userid, ec.ts, count(*)
					  FROM exp_change AS ec
						LEFT OUTER JOIN exp_test_change AS tc
							ON ec.id=tc.changeid
					  GROUP BY ec.id ORDER BY ec.ts DESC
					  LIMIT ?, ?`

	const stmtTotal = `SELECT count(*) FROM exp_change`

	// Get the total number of records.
	row := m.vdb.DB.QueryRow(stmtTotal)
	var total int
	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []*TriageLogEntry{}, 0, nil
	}

	// Fetch the records we are interested in.
	rows, err := m.vdb.DB.Query(stmtList, offset, size)
	if err != nil {
		return nil, 0, err
	}
	defer util.Close(rows)

	result := make([]*TriageLogEntry, 0, size)
	for rows.Next() {
		entry := &TriageLogEntry{}
		if err = rows.Scan(&entry.Name, &entry.TS, &entry.ChangeCount); err != nil {
			return nil, 0, err
		}
		result = append(result, entry)
	}
	return result, total, nil
}

// Wraps around an ExpectationsStore and caches the expectations using
// MemExpecationsStore.
type CachingExpectationStore struct {
	store    ExpectationsStore
	cache    ExpectationsStore
	eventBus *eventbus.EventBus
	refresh  bool
}

func NewCachingExpectationStore(store ExpectationsStore, eventBus *eventbus.EventBus) ExpectationsStore {
	return &CachingExpectationStore{
		store:    store,
		cache:    NewMemExpectationsStore(nil),
		eventBus: eventBus,
		refresh:  true,
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

	ret := c.cache.AddChange(changedTests, userId)
	if ret == nil {
		testNames := make([]string, 0, len(changedTests))
		for testName := range changedTests {
			testNames = append(testNames, testName)
		}
		c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, testNames)
	}
	return ret
}

func (c *CachingExpectationStore) RemoveChange(changedDigests map[string][]string) error {
	if err := c.store.RemoveChange(changedDigests); err != nil {
		return err
	}

	err := c.cache.RemoveChange(changedDigests)
	if err == nil {
		testNames := make([]string, 0, len(changedDigests))
		for testName := range changedDigests {
			testNames = append(testNames, testName)
		}
		c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, testNames)
	}
	return err
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) QueryLog(offset, size int) ([]*TriageLogEntry, int, error) {
	return c.store.QueryLog(offset, size)
}
