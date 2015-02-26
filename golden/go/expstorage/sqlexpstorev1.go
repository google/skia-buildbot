package expstorage

import (
	"database/sql"
	"encoding/json"

	"github.com/skia-dev/glog"
	"skia.googlesource.com/buildbot.git/go/database"
	"skia.googlesource.com/buildbot.git/go/util"
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
func (e *SQLExpectationsStoreV1) IterExpectations() <-chan *ExpectationsRec {
	resultCh := make(chan *ExpectationsRec)
	go func() {
		defer close(resultCh)

		const stmt = `SELECT id, userid, ts, expectations
				  FROM expectations
				  ORDER BY ts ASC`

		rows, err := e.vdb.DB.Query(stmt)
		if err != nil {
			rec := &ExpectationsRec{
				Err: err,
			}
			resultCh <- rec
			return
		}

		for rows.Next() && (err == nil) {
			var err error = nil
			var expJSON string

			rec := &ExpectationsRec{}
			err = rows.Scan(&rec.ID, &rec.UserID, &rec.TS, &expJSON)
			if err == nil {
				err = json.Unmarshal([]byte(expJSON), &rec.Exp)
			}
			rec.Err = err
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
