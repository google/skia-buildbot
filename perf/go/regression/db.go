package regression

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"google.golang.org/api/iterator"
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
	mutex sync.Mutex
}

// NewStore returns a new Store.
func NewStore() *Store {
	return &Store{}
}

// DSRegression is used for storing Regressions in Cloud Datastore.
type DSRegression struct {
	TS      int64
	Triaged bool
	Body    string `datastore:",noindex"`
}

// load_ds loads Regressions stored for the given commit from Cloud Datastore.
func (s *Store) load_ds(tx *datastore.Transaction, cid *cid.CommitDetail) (*Regressions, error) {
	key := ds.NewKey(ds.REGRESSION)
	key.Name = cid.ID()
	dsRegression := &DSRegression{}
	if err := tx.Get(key, dsRegression); err != nil {
		return nil, err
	}
	ret := New()
	if err := json.Unmarshal([]byte(dsRegression.Body), ret); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON body: %s", err)
	}
	return ret, nil
}

// store_ds stores Regressions for the given commit in Cloud Datastore.
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
	q := ds.NewQuery(ds.REGRESSION).Filter("Triaged =", false).KeysOnly()
	it := ds.DS.Run(context.TODO(), q)
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
}

type DetailLookup func(c *cid.CommitID) (*cid.CommitDetail, error)

func (s *Store) Write(regressions map[string]*Regressions, lookup DetailLookup) error {
	i := 0
	for cidString, reg := range regressions {
		i += 1
		if i%100 == 0 {
			fmt.Printf(".")
		}
		if i%1000 == 0 {
			fmt.Printf(" %d\n", i)
		}
		c, err := cid.FromID(cidString)
		if err != nil {
			return fmt.Errorf("Got an invalid cid %q: %s", cidString, err)
		}
		commitDetail, err := lookup(c)
		if err != nil {
			return fmt.Errorf("Could not find details for cid %q: %s", cidString, err)
		}
		_, err = ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
			return s.store_ds(tx, commitDetail, reg)
		})
		if err != nil {
			return fmt.Errorf("Could not store regressions for cid %q: %s", cidString, err)
		}
	}
	return nil
}

// Range returns a map from cid.ID()'s to *Regressions that exist in the given time range,
// or for all time if subset is UNTRIAGED_SUBSET.
func (s *Store) Range(begin, end int64, subset Subset) (map[string]*Regressions, error) {
	ret := map[string]*Regressions{}
	q := ds.NewQuery(ds.REGRESSION).Filter("TS >=", begin).Filter("TS <", end)
	it := ds.DS.Run(context.TODO(), q)
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
}

// SetHigh sets the cluster for a high regression at the given commit and alertID.
func (s *Store) SetHigh(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
	isNew := false
	_, err := ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
		r, err := s.load_ds(tx, cid)
		if err == datastore.ErrNoSuchEntity {
			r = New()
		} else if err != nil {
			return err
		}
		isNew = r.SetHigh(alertID, df, high)
		return s.store_ds(tx, cid, r)
	})
	return isNew, err
}

// SetLow sets the cluster for a low regression at the given commit and alertID.
func (s *Store) SetLow(cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
	isNew := false
	_, err := ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
		r, err := s.load_ds(tx, cid)
		if err == datastore.ErrNoSuchEntity {
			r = New()
		} else if err != nil {
			return err
		}
		isNew = r.SetLow(alertID, df, low)
		return s.store_ds(tx, cid, r)
	})
	return isNew, err
}

// TriageLow sets the triage status for the low cluster at the given commit and alertID.
func (s *Store) TriageLow(cid *cid.CommitDetail, alertID string, tr TriageStatus) error {
	_, err := ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
		r, err := s.load_ds(tx, cid)
		if err != nil {
			return fmt.Errorf("Failed to load Regressions: %s", err)
		}
		if err = r.TriageLow(alertID, tr); err != nil {
			return fmt.Errorf("Failed to update Regressions: %s", err)
		}
		return s.store_ds(tx, cid, r)
	})
	return err
}

// TriageHigh sets the triage status for the high cluster at the given commit and alertID.
func (s *Store) TriageHigh(cid *cid.CommitDetail, alertID string, tr TriageStatus) error {
	_, err := ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
		r, err := s.load_ds(tx, cid)
		if err != nil {
			return fmt.Errorf("Failed to load Regressions: %s", err)
		}
		if err = r.TriageHigh(alertID, tr); err != nil {
			return fmt.Errorf("Failed to update Regressions: %s", err)
		}
		return s.store_ds(tx, cid, r)
	})
	return err
}
