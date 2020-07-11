// Package dsregressionstore implements regression.Store using Google Cloud Datastore.
package dsregressionstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/cid"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/types"
	"google.golang.org/api/iterator"
)

// dsEntry is used for storing regression.Regressions in a RegressionStore.
type dsEntry struct {
	TS      int64
	Triaged bool
	Body    string `datastore:",noindex"`
}

// RegressionStoreDS implements RegressionStore using Google Cloud Datastore.
type RegressionStoreDS struct {
	lookup regression.DetailLookup
	mutex  sync.Mutex
}

// NewRegressionStoreDS returns a new RegressionStoreDS.
func NewRegressionStoreDS(lookup regression.DetailLookup) *RegressionStoreDS {
	return &RegressionStoreDS{
		lookup: lookup,
	}
}

// loadFromDS loads regression.Regressions stored for the given commit from Cloud Datastore.
func (s *RegressionStoreDS) loadFromDS(tx *datastore.Transaction, cid *cid.CommitDetail) (*regression.AllRegressionsForCommit, error) {
	key := ds.NewKey(ds.REGRESSION)
	key.Name = cid.ID()
	entry := &dsEntry{}
	if err := tx.Get(key, entry); err != nil {
		return nil, err
	}
	ret := regression.New()
	if err := json.Unmarshal([]byte(entry.Body), ret); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON body: %s", err)
	}
	return ret, nil
}

// storeToDS stores regression.Regressions for the given commit in Cloud Datastore.
func (s *RegressionStoreDS) storeToDS(tx *datastore.Transaction, cid *cid.CommitDetail, r *regression.AllRegressionsForCommit) error {
	body, err := r.JSON()
	if err != nil {
		return fmt.Errorf("Failed to encode regression.Regressions to JSON: %s", err)
	}
	if len(body) > 1024*1024 {
		return fmt.Errorf("regression.Regressions is too large, >1MB.")
	}
	entry := &dsEntry{
		Body:    string(body),
		Triaged: r.Triaged(),
		TS:      cid.Timestamp,
	}
	key := ds.NewKey(ds.REGRESSION)
	key.Name = cid.ID()
	_, err = tx.Put(key, entry)
	if err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// Range implements the RegressionStore interface.
func (s *RegressionStoreDS) Range(ctx context.Context, begin, end types.CommitNumber) (map[types.CommitNumber]*regression.AllRegressionsForCommit, error) {
	beginDetail, err := s.lookup(ctx, &cid.CommitID{Offset: begin})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed for begin=%d", begin)
	}
	endDetail, err := s.lookup(ctx, &cid.CommitID{Offset: end})
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed for end=%d", end)
	}

	ret := map[types.CommitNumber]*regression.AllRegressionsForCommit{}
	q := ds.NewQuery(ds.REGRESSION).Filter("TS >=", beginDetail.Timestamp).Filter("TS <=", endDetail.Timestamp)
	it := ds.DS.Run(ctx, q)
	for {
		entry := &dsEntry{}
		key, err := it.Next(entry)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed to read from database: %s", err)
		}
		reg := regression.New()
		if err := json.Unmarshal([]byte(entry.Body), reg); err != nil {
			return nil, fmt.Errorf("Failed to decode JSON body: %s", err)
		}
		// We use a form of the types.CommitNumber serialized as "master-%06d"
		// as the key, so we need to convert that back to a types.CommitNumber.
		c, err := cid.FromID(key.Name)
		if err != nil {
			return nil, fmt.Errorf("Failed to turn commit id into a CommitID: %s", err)
		}
		ret[types.CommitNumber(c.Offset)] = reg
	}
	return ret, nil
}

// SetHigh implements the RegressionStore interface.
func (s *RegressionStoreDS) SetHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, high *clustering2.ClusterSummary) (bool, error) {
	isNew := false
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		r, err := s.loadFromDS(tx, cid)
		if err == datastore.ErrNoSuchEntity {
			r = regression.New()
		} else if err != nil {
			return err
		}
		isNew = r.SetHigh(alertID, df, high)
		return s.storeToDS(tx, cid, r)
	})
	return isNew, err
}

// SetLow implements the RegressionStore interface.
func (s *RegressionStoreDS) SetLow(ctx context.Context, cid *cid.CommitDetail, alertID string, df *dataframe.FrameResponse, low *clustering2.ClusterSummary) (bool, error) {
	isNew := false
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		r, err := s.loadFromDS(tx, cid)
		if err == datastore.ErrNoSuchEntity {
			r = regression.New()
		} else if err != nil {
			return err
		}
		isNew = r.SetLow(alertID, df, low)
		return s.storeToDS(tx, cid, r)
	})
	return isNew, err
}

// TriageLow implements the RegressionStore interface.
func (s *RegressionStoreDS) TriageLow(ctx context.Context, cid *cid.CommitDetail, alertID string, tr regression.TriageStatus) error {
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		r, err := s.loadFromDS(tx, cid)
		if err != nil {
			return fmt.Errorf("Failed to load regression.Regressions: %s", err)
		}
		if err = r.TriageLow(alertID, tr); err != nil {
			return fmt.Errorf("Failed to update regression.Regressions: %s", err)
		}
		return s.storeToDS(tx, cid, r)
	})
	return err
}

// TriageHigh implements the RegressionStore interface.
func (s *RegressionStoreDS) TriageHigh(ctx context.Context, cid *cid.CommitDetail, alertID string, tr regression.TriageStatus) error {
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		r, err := s.loadFromDS(tx, cid)
		if err != nil {
			return fmt.Errorf("Failed to load regression.Regressions: %s", err)
		}
		if err = r.TriageHigh(alertID, tr); err != nil {
			return fmt.Errorf("Failed to update regression.Regressions: %s", err)
		}
		return s.storeToDS(tx, cid, r)
	})
	return err
}

// Write implements the RegressionStore interface.
func (s *RegressionStoreDS) Write(ctx context.Context, regressions map[types.CommitNumber]*regression.AllRegressionsForCommit) error {
	for commitNumber, reg := range regressions {
		c := cid.CommitIDFromCommitNumber(commitNumber)
		commitDetail, err := s.lookup(ctx, c)
		if err != nil {
			return fmt.Errorf("Could not find details for cid %v: %s", c, err)
		}
		_, err = ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
			return s.storeToDS(tx, commitDetail, reg)
		})
		if err != nil {
			return fmt.Errorf("Could not store regressions for cid %v: %s", c, err)
		}
	}
	return nil
}

// Confirm the RegressionStoreDS implements the RegressionStore interface.
var _ regression.Store = (*RegressionStoreDS)(nil)
