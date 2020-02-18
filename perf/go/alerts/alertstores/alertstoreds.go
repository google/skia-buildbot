// Package alertstores has implementations of the alerts.AlertStore interface.
package alertstores

import (
	"context"
	"fmt"
	"sort"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	"google.golang.org/api/iterator"
)

// AlertStoreDS implements the alerts.AlertStore interface on top of Google
// Cloud Datastore.
type AlertStoreDS struct {
}

// NewAlertStoreDS returns a new Store.
func NewAlertStoreDS() *AlertStoreDS {
	return &AlertStoreDS{}
}

// Save implements the alerts.AlertStore interface.
func (s *AlertStoreDS) Save(cfg *alerts.Alert) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("Failed to save invalid Config: %s", err)
	}
	key := ds.NewKey(ds.ALERT)
	if cfg.ID != alerts.INVALID_ID {
		key.ID = int64(cfg.ID)
	}
	if _, err := ds.DS.Put(context.TODO(), key, cfg); err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// Delete implements the alerts.AlertStore interface.
func (s *AlertStoreDS) Delete(id int) error {
	key := ds.NewKey(ds.ALERT)
	key.ID = int64(id)

	_, err := ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
		cfg := alerts.NewConfig()
		if err := tx.Get(key, cfg); err != nil {
			return fmt.Errorf("Failed to retrieve from datastore: %s", err)
		}
		cfg.State = alerts.DELETED
		if _, err := tx.Put(key, cfg); err != nil {
			return fmt.Errorf("Failed to write to database: %s", err)
		}
		return nil
	})
	return err
}

// configSlice is a utility type for sorting Configs by DisplayName.
type configSlice []*alerts.Alert

func (p configSlice) Len() int           { return len(p) }
func (p configSlice) Less(i, j int) bool { return p[i].DisplayName < p[j].DisplayName }
func (p configSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// List implements the alerts.AlertStore interface.
func (s *AlertStoreDS) List(includeDeleted bool) ([]*alerts.Alert, error) {
	ret := []*alerts.Alert{}
	q := ds.NewQuery(ds.ALERT)
	if !includeDeleted {
		q = q.Filter("State =", int(alerts.ACTIVE))
	}
	it := ds.DS.Run(context.TODO(), q)
	for {
		cfg := alerts.NewConfig()
		k, err := it.Next(cfg)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed retrieving alert list: %s", err)
		}
		cfg.ID = k.ID
		if err := cfg.Validate(); err != nil {
			sklog.Errorf("Found an invalid alert %v: %s", *cfg, err)
		}
		ret = append(ret, cfg)
	}

	sort.Sort(configSlice(ret))
	return ret, nil
}

// Confirm this Google Cloud Datastore implements the AlertStore interface.
var _ alerts.AlertStore = (*AlertStoreDS)(nil)
