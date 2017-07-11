package regression

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/db"
)

// Subset is the subset of regressions we are querying for.
type Subset string

const (
	ALL_SUBSET         Subset = "all"         // Include all regressions in a range.
	REGRESSIONS_SUBSET Subset = "regressions" // Only include regressions in a range that are alerting.
	UNTRIAGED_SUBSET   Subset = "untriaged"   // All untriaged alerting regressions regardless of range.
)

// Store persists Regressions to/from an SQL database.
type Store struct {
}

// NewStore returns a new Store.
func NewStore() *Store {
	return &Store{}
}

// load Regressions stored for the given commit.
func (s *Store) load(tx *sql.Tx, cid *cid.CommitDetail) (*Regressions, error) {
	row := tx.QueryRow("SELECT cid, body FROM regression WHERE cid=?", cid.ID())
	if row == nil {
		return nil, fmt.Errorf("Failed to query database for %q.", cid.ID())
	}
	var id string
	var body string
	if err := row.Scan(&id, &body); err != nil {
		return nil, fmt.Errorf("Failed to read from database: %s", err)
	}
	ret := New()

	if err := json.Unmarshal([]byte(body), ret); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON body: %s", err)
	}
	return ret, nil
}

// store Regressions for the given commit.
func (s *Store) store(tx *sql.Tx, cid *cid.CommitDetail, r *Regressions) error {
	body, err := r.JSON()
	// MEDIUMTEXT is only 16MB, and will silently be truncated.
	if len(body) > 16777215 {
		return fmt.Errorf("Regressions is too large, >16 MB.")
	}
	if err != nil {
		return fmt.Errorf("Failed to encode Regressions to JSON: %s", err)
	}
	_, err = tx.Exec("INSERT INTO regression (cid, timestamp, triaged, body) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE  cid=?, timestamp=?, triaged=?, body=?",
		cid.ID(), cid.Timestamp, r.Triaged(), body,
		cid.ID(), cid.Timestamp, r.Triaged(), body)

	if err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// Untriaged returns the number of untriaged regressions.
func (s *Store) Untriaged() (int, error) {
	row := db.DB.QueryRow("SELECT count(*) FROM regression WHERE triaged=false")
	if row == nil {
		return -1, fmt.Errorf("Failed to query database for count of untriaged regressions.")
	}
	var count int
	if err := row.Scan(&count); err != nil {
		return -1, fmt.Errorf("Failed to read from database: %s", err)
	}
	return count, nil
}

// Range returns a map from cid.ID()'s to *Regressions that exist in the given time range.
func (s *Store) Range(begin, end int64, subset Subset) (map[string]*Regressions, error) {
	ret := map[string]*Regressions{}

	rows, err := db.DB.Query("SELECT cid, timestamp, body FROM regression WHERE timestamp >= ? AND timestamp < ? ORDER BY timestamp", begin, end)
	if subset == UNTRIAGED_SUBSET {
		rows, err = db.DB.Query("SELECT cid, timestamp, body FROM regression WHERE triaged=false ORDER BY timestamp")
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to query from database: %s", err)
	}
	defer util.Close(rows)
	for rows.Next() {
		var id string
		var timestamp int64
		var body string
		if err := rows.Scan(&id, &timestamp, &body); err != nil {
			return nil, fmt.Errorf("Failed to read from database: %s", err)
		}
		reg := New()
		if err := json.Unmarshal([]byte(body), reg); err != nil {
			return nil, fmt.Errorf("Failed to decode JSON body: %s", err)
		}
		ret[id] = reg
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("Error while iterating rows: %s", err)
	}
	return ret, nil
}

// intx runs f within a database transaction.
//
func intx(f func(tx *sql.Tx) error) (err error) {
	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("Failed to start transaction: %s", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	err = f(tx)
	return err
}

// SetHigh sets the cluster for a high regression at the given commit and alertID.
func (s *Store) SetHigh(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
	isNew := false
	err := intx(func(tx *sql.Tx) error {
		r, err := s.load(tx, cid)
		if err != nil {
			r = New()
		}
		isNew = r.SetHigh(alertID, df, high)
		return s.store(tx, cid, r)
	})
	return isNew, err
}

// SetLow sets the cluster for a low regression at the given commit and alertID.
func (s *Store) SetLow(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
	isNew := false
	err := intx(func(tx *sql.Tx) error {
		r, err := s.load(tx, cid)
		if err != nil {
			r = New()
		}
		isNew = r.SetLow(alertID, df, low)
		return s.store(tx, cid, r)
	})
	return isNew, err
}

// TriageLow sets the triage status for the low cluster at the given commit and alertID.
func (s *Store) TriageLow(cid *cid.CommitDetail, alertID string, tr TriageStatus) error {
	return intx(func(tx *sql.Tx) error {
		r, err := s.load(tx, cid)
		if err != nil {
			return fmt.Errorf("Failed to load Regressions: %s", err)
		}
		if err = r.TriageLow(alertID, tr); err != nil {
			return fmt.Errorf("Failed to update Regressions: %s", err)
		}
		return s.store(tx, cid, r)
	})
}

// TriageHigh sets the triage status for the high cluster at the given commit and alertID.
func (s *Store) TriageHigh(cid *cid.CommitDetail, alertID string, tr TriageStatus) error {
	return intx(func(tx *sql.Tx) error {
		r, err := s.load(tx, cid)
		if err != nil {
			return fmt.Errorf("Failed to load Regressions: %s", err)
		}
		if err := r.TriageHigh(alertID, tr); err != nil {
			return fmt.Errorf("Failed to update Regressions: %s", err)
		}
		return s.store(tx, cid, r)
	})
}
