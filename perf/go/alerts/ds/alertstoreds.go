// Save and retrieve alerts.Config's to/from a database.
//
// TODO(jcgregorio) Add a cleanup process that removes DELETED configs from the
// database after a long period of time, using the lastmodified timestamp.
package alertstoreds

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

// AlertStoreDS persists Config's to/from datastore.
type AlertStoreDS struct {
}

// NewAlertStoreDS returns a new Store.
func NewAlertStoreDS() *AlertStoreDS {
	return &AlertStoreDS{}
}

// Save can write a new, or update an existing, Config. New
// Config's will have an ID of -1.
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

// Delete removes the Alert with the given id.
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

// ConfigSlice is a utility type for sorting Configs by DisplayName.
type ConfigSlice []*alerts.Alert

func (p ConfigSlice) Len() int           { return len(p) }
func (p ConfigSlice) Less(i, j int) bool { return p[i].DisplayName < p[j].DisplayName }
func (p ConfigSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// List retrieves all the Alerts.
//
// If includeDeleted is true then deleted Alerts are also included in the
// response.
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

	sort.Sort(ConfigSlice(ret))
	return ret, nil
}

// Confirm this Google Cloud Datastore implements the AlertStore interface.
var _ alerts.AlertStore = (*AlertStoreDS)(nil)
