package regression

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/db"
	"go.skia.org/infra/perf/go/ds"
	"google.golang.org/api/iterator"
)

var (
	useCloudDatastore bool
)

// Init initializes regression db.
//
// useDS - Is true if regressions should be store in Google Cloud Datastore, otherwise they're stored in Cloud SQL.
func Init(useDS bool) {
	useCloudDatastore = useDS
}

// Subset is the subset of regressions we are querying for.
type Subset string

const (
	ALL_SUBSET         Subset = "all"         // Include all regressions in a range.
	REGRESSIONS_SUBSET Subset = "regressions" // Only include regressions in a range that are alerting.
	UNTRIAGED_SUBSET   Subset = "untriaged"   // All untriaged alerting regressions regardless of range.
)

// Store persists Regressions to/from an SQL database.
type Store struct {
	mutex sync.Mutex
}

// NewStore returns a new Store.
func NewStore() *Store {
	return &Store{}
}

// load Regressions stored for the given commit.
func (s *Store) load(tx *sql.Tx, cid *cid.CommitDetail) (*Regressions, error) {
	var id string
	var body string
	if err := db.DB.QueryRow("SELECT cid, body FROM regression WHERE cid=? FOR UPDATE", cid.ID()).Scan(&id, &body); err != nil {
		return nil, fmt.Errorf("Failed to read from database: %s", err)
	}
	ret := New()

	if err := json.Unmarshal([]byte(body), ret); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON body: %s", err)
	}
	return ret, nil
}

// DSRegression is used for storing Regressions in Cloud Datastore.
type DSRegression struct {
	TS      int64
	Triaged bool
	Body    string `datastore:",noindex"`
}

// load_ds loads Regressions stored for the given commit from Cloud Storage.
func (s *Store) load_ds(tx *datastore.Transaction, cid *cid.CommitDetail) (*Regressions, error) {
	key := ds.NewKey(ds.REGRESSION)
	key.Name = cid.ID()
	dsRegression := &DSRegression{}
	if err := tx.Get(key, dsRegression); err != nil {
		return nil, fmt.Errorf("Failed to read from database: %s", err)
	}
	ret := New()
	if err := json.Unmarshal([]byte(dsRegression.Body), ret); err != nil {
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

// store_ds stores Regressions for the given commit in Cloud Storage.
func (s *Store) store_ds(tx *datastore.Transaction, cid *cid.CommitDetail, r *Regressions) error {
	body, err := r.JSON()
	if err != nil {
		return fmt.Errorf("Failed to encode Regressions to JSON: %s", err)
	}
	if len(body) > 1024*1024 {
		return fmt.Errorf("Regressions is too large, >1MB.")
	}
	dsRegression := &DSRegression{
		Body:    string(body),
		Triaged: r.Triaged(),
		TS:      cid.Timestamp,
	}
	key := ds.NewKey(ds.REGRESSION)
	key.Name = cid.ID()
	_, err = tx.Put(key, dsRegression)
	if err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// Untriaged returns the number of untriaged regressions.
func (s *Store) Untriaged() (int, error) {
	if useCloudDatastore {
		q := ds.NewQuery(ds.REGRESSION).Filter("Triaged =", false).KeysOnly()
		it := ds.DS.Run(context.Background(), q)
		count := 0
		for {
			_, err := it.Next(nil)
			if err == iterator.Done {
				break
			} else if err != nil {
				return -1, fmt.Errorf("Failed to read from database: %s", err)
			}
			count += 1
		}
		return count, nil
	} else {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		var count int
		if err := db.DB.QueryRow("SELECT count(*) FROM regression WHERE triaged=false").Scan(&count); err != nil {
			return -1, fmt.Errorf("Failed to read from database: %s", err)
		}
		return count, nil
	}
}

// Range returns a map from cid.ID()'s to *Regressions that exist in the given time range.
func (s *Store) Range(begin, end int64, subset Subset) (map[string]*Regressions, error) {
	if useCloudDatastore {
		ret := map[string]*Regressions{}
		q := ds.NewQuery(ds.REGRESSION).Filter("TS >=", begin).Filter("TS <=", end)
		if subset == UNTRIAGED_SUBSET {
			q = ds.NewQuery(ds.REGRESSION).Filter("Triaged =", false)
		}
		it := ds.DS.Run(context.Background(), q)
		for {
			dsRegression := &DSRegression{}
			key, err := it.Next(dsRegression)
			if err == iterator.Done {
				break
			} else if err != nil {
				return nil, fmt.Errorf("Failed to read from database: %s", err)
			}
			reg := New()
			if err := json.Unmarshal([]byte(dsRegression.Body), reg); err != nil {
				return nil, fmt.Errorf("Failed to decode JSON body: %s", err)
			}
			ret[key.Name] = reg
		}
		return ret, nil
	} else {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		ret := map[string]*Regressions{}

		rows, err := db.DB.Query("SELECT cid, timestamp, body FROM regression WHERE timestamp >= ? AND timestamp < ? ORDER BY timestamp", begin, end)
		if subset == UNTRIAGED_SUBSET {
			rows, err = db.DB.Query("SELECT cid, timestamp, body FROM regression WHERE triaged=false ORDER BY timestamp")
		}
		defer func() {
			err := rows.Close()
			sklog.Errorf("MySQL error from iterating rows: %s %s", err, rows.Err())
		}()
		sklog.Warningf("MySQL Open Connections: %d", db.DB.Stats().OpenConnections)
		if err != nil {
			return nil, fmt.Errorf("Failed to query from database: %s", err)
		}
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
}

func indstx(f func(*datastore.Transaction) error) (err error) {
	tx, err := ds.DS.NewTransaction(context.Background())
	if err != nil {
		return fmt.Errorf("Failed to start transaction: %s", err)
	}
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = fmt.Errorf("Error occurred in rollback: %s while attempting to recover from %s.", rollbackErr, err)
			}
			return
		}
		_, err = tx.Commit()
	}()

	err = f(tx)
	return err
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
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				err = fmt.Errorf("Error occurred in rollback: %s while attempting to recover from %s.", rollbackErr, err)
			}
			return
		}
		err = tx.Commit()
	}()

	err = f(tx)
	return err
}

// SetHigh sets the cluster for a high regression at the given commit and alertID.
func (s *Store) SetHigh(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
	if useCloudDatastore {
		isNew := false
		err := indstx(func(tx *datastore.Transaction) error {
			r, err := s.load_ds(tx, cid)
			if err != nil {
				r = New()
			}
			isNew = r.SetHigh(alertID, df, high)
			return s.store_ds(tx, cid, r)
		})
		return isNew, err
	} else {
		s.mutex.Lock()
		defer s.mutex.Unlock()
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
}

// SetLow sets the cluster for a low regression at the given commit and alertID.
func (s *Store) SetLow(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
	if useCloudDatastore {
		isNew := false
		err := indstx(func(tx *datastore.Transaction) error {
			r, err := s.load_ds(tx, cid)
			if err != nil {
				r = New()
			}
			isNew = r.SetLow(alertID, df, low)
			return s.store_ds(tx, cid, r)
		})
		return isNew, err
	} else {
		s.mutex.Lock()
		defer s.mutex.Unlock()
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
}

// TriageLow sets the triage status for the low cluster at the given commit and alertID.
func (s *Store) TriageLow(cid *cid.CommitDetail, alertID string, tr TriageStatus) error {
	if useCloudDatastore {
		return indstx(func(tx *datastore.Transaction) error {
			r, err := s.load_ds(tx, cid)
			if err != nil {
				return fmt.Errorf("Failed to load Regressions: %s", err)
			}
			if err = r.TriageLow(alertID, tr); err != nil {
				return fmt.Errorf("Failed to update Regressions: %s", err)
			}
			return s.store_ds(tx, cid, r)
		})
	} else {
		s.mutex.Lock()
		defer s.mutex.Unlock()
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
}

// TriageHigh sets the triage status for the high cluster at the given commit and alertID.
func (s *Store) TriageHigh(cid *cid.CommitDetail, alertID string, tr TriageStatus) error {
	if useCloudDatastore {
		return indstx(func(tx *datastore.Transaction) error {
			r, err := s.load_ds(tx, cid)
			if err != nil {
				return fmt.Errorf("Failed to load Regressions: %s", err)
			}
			if err = r.TriageHigh(alertID, tr); err != nil {
				return fmt.Errorf("Failed to update Regressions: %s", err)
			}
			return s.store_ds(tx, cid, r)
		})
	} else {
		s.mutex.Lock()
		defer s.mutex.Unlock()
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
}
