package expstorage

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

// insertChunkSize is the number of records to insert with a single insert statement. The value
// was determined to work via the test. The exact value is not important since all inserts
// end up in the same transaction.
const insertChunkSize = 1000

// chunkPlaceholder is the placeholder string needed to insert a complete chunk of records.
var chunkPlaceholders = strings.TrimRight(strings.Repeat("(?,?,?,?),", insertChunkSize), ",")

// Stores expectations in an SQL database without any caching.
type SQLExpectationsStore struct {
	vdb *database.VersionedDB
}

func NewSQLExpectationStore(vdb *database.VersionedDB) *SQLExpectationsStore {
	return &SQLExpectationsStore{
		vdb: vdb,
	}
}

// See ExpectationsStore interface.
func (s *SQLExpectationsStore) Get() (exp types.TestExpBuilder, err error) {
	// Load the newest record from the database.
	const stmt = `SELECT t1.name, t1.digest, t1.label
	         FROM exp_test_change AS t1
	         JOIN (
	         	SELECT name, digest, MAX(changeid) as changeid
	         	FROM exp_test_change
	         	GROUP BY name, digest ) AS t2
				ON (t1.name = t2.name AND t1.digest = t2.digest AND t1.changeid = t2.changeid)
				WHERE t1.removed IS NULL`

	rows, err := s.vdb.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer util.Close(rows)

	result := types.TestExp{}
	for rows.Next() {
		var testName types.TestName
		var digest types.Digest
		var label string
		if err = rows.Scan(&testName, &digest, &label); err != nil {
			return nil, err
		}
		result.AddDigest(testName, digest, types.LabelFromString(label))
	}
	return types.NewTestExpBuilder(result), nil
}

// See ExpectationsStore interface.
func (s *SQLExpectationsStore) AddChange(changedTests types.TestExp, userId string) error {
	return s.AddChangeWithTimeStamp(changedTests, userId, 0, util.TimeStampMs())
}

// TODO(stephana): Remove the AddChangeWithTimeStamp if we remove the
// migration code that calls it.

// AddChangeWithTimeStamp adds changed tests to the database with the
// given time stamp. This is primarily for migration purposes.
func (s *SQLExpectationsStore) AddChangeWithTimeStamp(changedTests types.TestExp, userId string, undoID int64, timeStamp int64) (retErr error) {
	defer timer.New("adding exp change").Stop()

	// Count the number of values to add.
	changeCount := 0
	for _, digests := range changedTests {
		changeCount += len(digests)
	}

	const (
		insertChange = `INSERT INTO exp_change (userid, ts, undo_changeid) VALUES (?, ?, ?)`
		insertDigest = `INSERT INTO exp_test_change (changeid, name, digest, label) VALUES`
	)

	// start a transaction
	tx, err := s.vdb.DB.Begin()
	if err != nil {
		return err
	}

	defer func() { retErr = database.CommitOrRollback(tx, retErr) }()

	// create the change record
	result, err := tx.Exec(insertChange, userId, timeStamp, undoID)
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
	chunks := [][]interface{}{}
	remainderValuesStr := ""
	current := make([]interface{}, 0, insertChunkSize)
	for testName, digests := range changedTests {
		for d, label := range digests {
			remainderValuesStr += "(?, ?, ?, ?),"
			current = append(current, changeId, testName, d, label.String())

			if (len(current) / 4) >= insertChunkSize {
				chunks = append(chunks, current)
				current = make([]interface{}, 0, insertChunkSize)
				remainderValuesStr = ""
			}
		}
	}
	remainderValuesStr = remainderValuesStr[:len(remainderValuesStr)-1]

	// Insert all the chunks
	if len(chunks) > 0 {
		if err := insertWithPrep(insertDigest+chunkPlaceholders, tx, chunks...); err != nil {
			return err
		}
	}

	// Insert the remainder.
	if len(current) > 0 {
		if err := insertWithPrep(insertDigest+remainderValuesStr, tx, current); err != nil {
			return err
		}
	}

	return nil
}

// insertPrep assumes a statement with placeholders and one or more sets of values to insert.
func insertWithPrep(insertStmt string, tx *sql.Tx, valsArr ...[]interface{}) error {
	prepStmt, err := tx.Prepare(insertStmt)
	if err != nil {
		return err
	}
	defer util.Close(prepStmt)

	for _, vals := range valsArr {
		_, err = prepStmt.Exec(vals...)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveChange, see ExpectationsStore interface.
func (s *SQLExpectationsStore) RemoveChange(changedDigests types.TestExp) (retErr error) {
	defer timer.New("removing exp change").Stop()

	const markRemovedStmt = `UPDATE exp_test_change
	                         SET removed = IF(removed IS NULL, ?, removed)
	                         WHERE (name=?) AND (digest=?)`

	// start a transaction
	tx, err := s.vdb.DB.Begin()
	if err != nil {
		return err
	}

	defer func() { retErr = database.CommitOrRollback(tx, retErr) }()

	// Mark all the digests as removed.
	now := util.TimeStampMs()
	for testName, digests := range changedDigests {
		for digest := range digests {
			if _, err = tx.Exec(markRemovedStmt, now, testName, digest); err != nil {
				return err
			}
		}
	}

	return nil
}

// See ExpectationsStore interface.
func (s *SQLExpectationsStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	return s.queryChanges(offset, size, 0, details)
}

// getExpectationsAt returns the changes that are necessary to restore the values
// at the given triage change.
func (s *SQLExpectationsStore) getExpectationsAt(changeInfo *TriageLogEntry) (types.TestExp, error) {
	const stmtTmpl = `
		SELECT tc.name AS name, tc.digest AS digest, tc.label AS label
		FROM exp_change AS ec, exp_test_change AS tc
		WHERE ((tc.removed IS NULL) OR ((tc.removed IS NOT NULL) AND (tc.removed > ?))) AND
		      (ec.ts < ?) AND
		      (ec.id = tc.changeid) AND
					((tc.name, tc.digest) IN (%s))
		ORDER BY ec.ts ASC`

	if len(changeInfo.Details) == 0 {
		return types.TestExp{}, nil
	}

	// Extract the digests that we are interested in.
	ret := types.TestExp{}
	listArgs := []interface{}{changeInfo.TS, changeInfo.TS}
	placeHolders := []string{}
	for _, d := range changeInfo.Details {
		if _, ok := ret[d.TestName]; !ok {
			ret[d.TestName] = map[types.Digest]types.Label{}
		}
		ret[d.TestName][d.Digest] = types.UNTRIAGED
		listArgs = append(listArgs, d.TestName, d.Digest)
		placeHolders = append(placeHolders, "(?,?)")
	}

	// Add the necessary amount of placeholders to the SQL query.
	stmt := fmt.Sprintf(stmtTmpl, strings.Join(placeHolders, ","))

	// Fetch the records we are interested in.
	rows, err := s.vdb.DB.Query(stmt, listArgs...)
	if err != nil {
		return nil, err
	}
	defer util.Close(rows)

	var name types.TestName
	var digest types.Digest
	var label string
	for rows.Next() {
		if err = rows.Scan(&name, &digest, &label); err != nil {
			return nil, err
		}
		// We expect that there could be multiple results for the same name and
		// digest. They are sorted chronologically, so always overwrite earlier
		// results with later results.
		ret[name][digest] = types.LabelFromString(label)
	}

	return ret, nil
}

func (s *SQLExpectationsStore) queryChanges(offset, size int, changeID int64, details bool) ([]*TriageLogEntry, int, error) {
	const (
		stmtDetails = `SELECT ec.id, tc.name, tc.digest, tc.label
					  FROM exp_change AS ec, exp_test_change AS tc
					  WHERE (ec.id=tc.changeid) AND (ec.id IN (%s))
					  ORDER BY ec.id DESC, tc.name ASC, tc.digest ASC`

		stmtTotal = `SELECT count(*) FROM exp_change`

		stmtListTmpl = `SELECT ec.id, ec.userid, ec.ts, (IFNULL( COUNT( tc.changeid ) , 0 )) AS detailsCount, undo_changeid
					  FROM %s AS ec
						LEFT OUTER JOIN exp_test_change AS tc
							ON ec.id=tc.changeid
					  GROUP BY ec.id ORDER BY ec.ts DESC
					  LIMIT ?, ?`

		nestedQuery = `(SELECT * FROM exp_change WHERE id=?)`
	)

	// Adjust the query based on whether we are interested in finding a specific item.
	var stmtList string
	listArgs := []interface{}{offset, size}
	if changeID > 0 {
		stmtList = fmt.Sprintf(stmtListTmpl, nestedQuery)
		listArgs = append([]interface{}{changeID}, listArgs...)

	} else {
		stmtList = fmt.Sprintf(stmtListTmpl, "exp_change")
	}

	// Get the total number of records.
	row := s.vdb.DB.QueryRow(stmtTotal)
	var total int
	if err := row.Scan(&total); err != nil {
		return nil, 0, err
	}

	if total == 0 {
		return []*TriageLogEntry{}, 0, nil
	}

	// Fetch the records we are interested in.
	rows, err := s.vdb.DB.Query(stmtList, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer util.Close(rows)

	var ids []interface{}
	var placeHolders []string

	if details {
		ids = make([]interface{}, 0, size)
		placeHolders = make([]string, 0, size)
	}

	idToIdxMap := map[string]int{}
	result := make([]*TriageLogEntry, 0, size)
	for rows.Next() {
		entry := &TriageLogEntry{}
		if err = rows.Scan(&entry.ID, &entry.Name, &entry.TS, &entry.ChangeCount, &entry.UndoChangeID); err != nil {
			return nil, 0, err
		}

		result = append(result, entry)
		if details {
			idToIdxMap[entry.ID] = len(result) - 1
			ids = append(ids, entry.ID)
			placeHolders = append(placeHolders, "?")
		}
	}

	if details && len(result) > 0 {
		stmt := fmt.Sprintf(stmtDetails, strings.Join(placeHolders, ","))
		rows, err := s.vdb.DB.Query(stmt, ids...)
		if err != nil {
			return nil, 0, err
		}

		var recID int64
		for rows.Next() {
			detail := &TriageDetail{}
			if err = rows.Scan(&recID, &detail.TestName, &detail.Digest, &detail.Label); err != nil {
				return nil, 0, err
			}

			idx := idToIdxMap[strconv.FormatInt(recID, 10)]
			if result[idx].Details == nil {
				result[idx].Details = make([]*TriageDetail, 0, result[idx].ChangeCount)
			}
			result[idx].Details = append(result[idx].Details, detail)
		}
	}

	return result, total, nil
}

// See  ExpectationsStore interface.
func (s *SQLExpectationsStore) UndoChange(changeID int64, userID string) (types.TestExp, error) {
	changeInfo, err := s.loadChangeEntry(changeID)
	if err != nil {
		return nil, err
	}

	// TODO(stephana): Enable undo and redo for undos.There is a small different
	// between a redo and an undo in that the undo restores the state before the
	// undo, while the redo replays the changes that were added in the original
	// change.

	// Refuse to undo a change that is the result of on undo.
	if changeInfo.UndoChangeID != 0 {
		return nil, fmt.Errorf("Unable to undo change %d which was created as an undo of change %d.", changeID, changeInfo.UndoChangeID)
	}

	// Get the expectations of tests of interest at that time.
	changes, err := s.getExpectationsAt(changeInfo)
	if err != nil {
		return nil, err
	}

	return changes, s.AddChangeWithTimeStamp(changes, userID, changeID, util.TimeStampMs())
}

// Clear implements the ExpectationsStore interface.
func (s *SQLExpectationsStore) Clear() error {
	const stmt_ec = `DELETE FROM exp_change`
	const stmt_etc = `DELETE FROM exp_test_change`

	var egroup errgroup.Group
	egroup.Go(func() error {
		_, err := s.vdb.DB.Exec(stmt_ec)
		return err
	})

	egroup.Go(func() error {
		_, err := s.vdb.DB.Exec(stmt_etc)
		return err
	})

	return egroup.Wait()
}

// Loads a single change entry with all details from the DB.
func (s *SQLExpectationsStore) loadChangeEntry(changeID int64) (*TriageLogEntry, error) {
	changeInfo, _, err := s.queryChanges(0, 5, changeID, true)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve triage information: %s", err)
	}

	if len(changeInfo) != 1 {
		return nil, fmt.Errorf("Triage information for change id %d should only be one record.", changeID)
	}

	return changeInfo[0], nil
}

// Wraps around an ExpectationsStore and caches the expectations using
// MemExpectationsStore.
type CachingExpectationStore struct {
	store    DEPRECATED_ExpectationsStore
	cache    DEPRECATED_ExpectationsStore
	eventBus eventbus.EventBus
	refresh  bool
}

func NewCachingExpectationStore(store DEPRECATED_ExpectationsStore, eventBus eventbus.EventBus) *CachingExpectationStore {
	ret := &CachingExpectationStore{
		store:    store,
		cache:    NewMemExpectationsStore(nil),
		eventBus: eventBus,
		refresh:  true,
	}

	// Prime the cache upon creation.
	// We ignore any error returned here for a simplified function signature and
	// also because any ExpectationsStore implementation (i.e. CloudExpectationsStore)
	// will do some connection checks before being passed to this instance.
	_, _ = ret.Get()

	// Register the events to update the cache.
	ret.eventBus.SubscribeAsync(EV_EXPSTORAGE_CHANGED, ret.addChangeToCache)

	return ret
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) Get() (exp types.TestExpBuilder, err error) {
	if c.refresh {
		c.refresh = false
		tempExp, err := c.store.Get()
		if err != nil {
			return nil, err
		}

		if err = c.cache.AddChange(tempExp.TestExp().DeepCopy(), ""); err != nil {
			return nil, err
		}
	}
	return c.cache.Get()
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) AddChange(changedTests types.TestExp, userId string) error {
	if err := c.store.AddChange(changedTests, userId); err != nil {
		return err
	}
	// Fire an event that will trigger the addition to the cache and wait for it to complete.
	// This is necessary because events that change the cache could also come from the distributed
	// eventbus.
	waitCh := make(chan bool)
	c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedTests, masterIssueID, waitCh), true)
	<-waitCh
	return nil
}

// addChangeToCache updates the cache and fires the change event.
func (c *CachingExpectationStore) addChangeToCache(evtChangedTests interface{}) {
	evtData := evtChangedTests.(*EventExpectationChange)
	changedTests := evtData.TestChanges

	// Split the changes into removal and addition.
	forRemoval := make(types.TestExp, len(changedTests))
	forAddition := make(types.TestExp, len(changedTests))
	for test, digests := range changedTests {
		for digest, label := range digests {
			if label == types.UNTRIAGED {
				if foundTest, ok := forRemoval[test]; ok {
					foundTest[digest] = label
				} else {
					forRemoval[test] = types.TestClassification{digest: label}
				}
			} else {
				if foundTest, ok := forAddition[test]; ok {
					foundTest[digest] = label
				} else {
					forAddition[test] = types.TestClassification{digest: label}
				}
			}
		}
	}

	if len(forRemoval) > 0 {
		if err := c.cache.RemoveChange(forRemoval); err != nil {
			sklog.Errorf("Error removing changed expectations to cache: %s", err)
		}
	}

	if len(forAddition) > 0 {
		if err := c.cache.AddChange(forAddition, ""); err != nil {
			sklog.Errorf("Error adding changed expectations to cache: %s", err)
		}
	}
	if evtData.waitCh != nil {
		evtData.waitCh <- true
	}
	sklog.Infof("Expectations change has been added to the cache.")
}

// RemoveChange implements the DEPRECATED_ExpectationsStore interface.
func (c *CachingExpectationStore) RemoveChange(changedDigests types.TestExp) error {
	if err := c.store.RemoveChange(changedDigests); err != nil {
		return err
	}

	// Fire an event that will trigger the addition to the cache.
	waitCh := make(chan bool)
	c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedDigests, masterIssueID, waitCh), true)
	<-waitCh
	return nil
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	return c.store.QueryLog(offset, size, details)
}

// See  ExpectationsStore interface.
func (c *CachingExpectationStore) UndoChange(changeID int64, userID string) (types.TestExp, error) {
	changedTests, err := c.store.UndoChange(changeID, userID)
	if err != nil {
		return nil, err
	}

	// Fire an event that will trigger the addition to the cache.
	waitCh := make(chan bool)
	c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedTests, masterIssueID, waitCh), true)
	<-waitCh
	return changedTests, nil
}

// Clear implements the ExpectationsStore interface.
func (c *CachingExpectationStore) Clear() error {
	if err := c.store.Clear(); err != nil {
		return err
	}
	return c.cache.Clear()
}

// Make sure SQLExpectationsStore fulfills the ExpectationsStore interface
var _ ExpectationsStore = (*SQLExpectationsStore)(nil)

// Make sure SQLExpectationsStore fulfills the DEPRECATED_ExpectationsStore interface
var _ DEPRECATED_ExpectationsStore = (*SQLExpectationsStore)(nil)

// Make sure CachingExpectationStore fulfills the ExpectationsStore interface
var _ ExpectationsStore = (*CachingExpectationStore)(nil)

// Make sure CachingExpectationStore fulfills the DEPRECATED_ExpectationsStore interface
var _ DEPRECATED_ExpectationsStore = (*CachingExpectationStore)(nil)
