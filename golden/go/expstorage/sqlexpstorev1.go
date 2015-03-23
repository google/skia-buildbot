package expstorage

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/database"
	"go.skia.org/infra/go/util"
)

// Stores expectations in an SQL database without any caching.
type SQLExpectationsStoreV1 struct {
	vdb *database.VersionedDB
}

// Helper struct to iterate the expecations table.
type ExpectationsRec struct {
	ID     int
	TS     int64
	Exp    *Expectations
	UserID string
	Err    error
}

func NewSQLExpectationStoreV1(vdb *database.VersionedDB) *SQLExpectationsStoreV1 {
	return &SQLExpectationsStoreV1{
		vdb: vdb,
	}
}

// IterExpectations iterates over the expecations table in time chronological
// order and returns a channel that can be iterated to get the expectations
// over time. If an error ocurs it will be set in Err field of ExpectatonsRec.
// It is inefficient in that it loads all records into memory before
// sending them to the returned channel. This was added to avoid more complicated
// DB interactions. This is used for migration only.
func (e *SQLExpectationsStoreV1) IterExpectations() <-chan *ExpectationsRec {
	resultCh := make(chan *ExpectationsRec, 1)
	go func() {
		defer close(resultCh)

		const getRecStmt = `SELECT id, userid, ts, expectations
							FROM expectations
							ORDER BY ts ASC`

		// Get all recores.
		allRecs := []*ExpectationsRec{}
		rows, err := e.vdb.DB.Query(getRecStmt)
		if err != nil {
			resultCh <- &ExpectationsRec{Err: err}
			return
		}

		defer util.Close(rows)

		var last *ExpectationsRec = nil
		for rows.Next() {
			rec := &ExpectationsRec{}
			var expJSON string
			err = rows.Scan(&rec.ID, &rec.UserID, &rec.TS, &expJSON)
			if err == nil {
				err = json.Unmarshal([]byte(expJSON), &rec.Exp)
			}

			if err != nil {
				rec.Err = err
				resultCh <- rec
				return
			}

			// Make sure timestamps and ids are strictly monotonically increasing.
			if last != nil {
				if (last.ID >= rec.ID) || (last.TS >= rec.TS) {
					rec.Err = fmt.Errorf("Records are not increasing: %d->%d : %d -> %d", last.ID, rec.ID, last.TS, rec.TS)
					resultCh <- rec
					return
				}
			}
			last = rec
			allRecs = append(allRecs, rec)
		}

		glog.Infof("Read %d database records. Starting to fill channel.", len(allRecs))

		for _, rec := range allRecs {
			resultCh <- rec
		}
	}()

	return resultCh
}

// See ExpectationsStore interface.
func (e *SQLExpectationsStoreV1) Get() (exp *Expectations, err error) {
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
func (e *SQLExpectationsStoreV1) Put(exp *Expectations, userId string) error {
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
func (e *SQLExpectationsStoreV1) Changes() <-chan []string {
	glog.Fatal("SQLExpectationsStore doesn't really support Changes.")
	return nil
}
