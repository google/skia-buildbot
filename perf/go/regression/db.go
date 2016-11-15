package regression

import (
	"encoding/json"
	"fmt"
	"sync"

	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/db"
)

// Store persists Regressions to/from an SQL database.
type Store struct {
	// mutex serializes all access to SQL since these operations
	// read/modify/write rows and we don't want a lost update problem.
	mutex sync.Mutex
}

// NewStore returns a new Store. Only one of these should be created.
func NewStore() *Store {
	return &Store{}
}

// load Regressions stored for the given commit.
func (s *Store) load(cid *cid.CommitDetail) (*Regressions, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	row := db.DB.QueryRow("SELECT cid, body FROM regression WHERE cid=?", cid.ID())
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
func (s *Store) store(cid *cid.CommitDetail, r *Regressions) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	body, err := r.JSON()
	if err != nil {
		return fmt.Errorf("Failed to encode Regressions to JSON: %s", err)
	}
	_, err = db.DB.Exec(
		"INSERT INTO regression (cid, timestamp, triaged, body) VALUES (?, ?, ?, ?)",
		cid.ID(), cid.Timestamp, r.Triaged(), body)

	if err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// Range returns a map from cid.ID()'s to *Regressions that exist in the given time range.
func (s *Store) Range(begin, end int64) (map[string]*Regressions, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	ret := map[string]*Regressions{}
	rows, err := db.DB.Query("SELECT cid, timestamp, body FROM regression WHERE timestamp >= ? AND timestamp < ? ORDER BY timestamp", begin, end)
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
	return ret, nil
}

// SetHigh sets the cluster for a high regression at the given commit and query.
func (s *Store) SetHigh(cid *cid.CommitDetail, query string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	r, err := s.load(cid)
	if err != nil {
		return fmt.Errorf("Failed to load Regressions: %s", err)
	}
	r.SetHigh(query, df, high)
	return s.store(cid, r)
}

// SetLow sets the cluster for a low regression at the given commit and query.
func (s *Store) SetLow(cid *cid.CommitDetail, query string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	r, err := s.load(cid)
	if err != nil {
		return fmt.Errorf("Failed to load Regressions: %s", err)
	}
	r.SetLow(query, df, low)
	return s.store(cid, r)
}

// TriageLow sets the triage status for the low cluster at the given commit and query.
func (s *Store) TriageLow(cid *cid.CommitDetail, query string, tr TriageStatus) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	r, err := s.load(cid)
	if err != nil {
		return fmt.Errorf("Failed to load Regressions: %s", err)
	}
	if err := r.TriageLow(query, tr); err != nil {
		return fmt.Errorf("Failed to update Regressions: %s", err)
	}
	return s.store(cid, r)
}

// TriageHigh sets the triage status for the high cluster at the given commit and query.
func (s *Store) TriageHigh(cid *cid.CommitDetail, query string, tr TriageStatus) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	r, err := s.load(cid)
	if err != nil {
		return fmt.Errorf("Failed to load Regressions: %s", err)
	}
	if err := r.TriageHigh(query, tr); err != nil {
		return fmt.Errorf("Failed to update Regressions: %s", err)
	}
	return s.store(cid, r)
}
