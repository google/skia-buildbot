// Package dsalertstore implements the alerts.Store interface via Google
// Cloud Datastore.
package dsalertstore

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

// DSAlertStore implements the alerts.Store interface on top of Google
// Cloud Datastore.
type DSAlertStore struct {
}

// New returns a new Store.
func New() *DSAlertStore {
	return &DSAlertStore{}
}

// Save implements the alerts.Store interface.
func (s *DSAlertStore) Save(ctx context.Context, cfg *alerts.Alert) error {
	cfg.SetIDFromString(cfg.IDAsString)
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("Failed to save invalid Config: %s", err)
	}

	// Make sure StateAsString also appears in the legacy format of State since
	// it is used for filtering in the List() func.
	cfg.State = alerts.ConfigStateToInt(cfg.StateAsString)

	key := ds.NewKey(ds.ALERT)
	if cfg.ID != alerts.BadAlertID {
		key.ID = int64(cfg.ID)
	}
	if _, err := ds.DS.Put(ctx, key, cfg); err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

// Delete implements the alerts.Store interface.
func (s *DSAlertStore) Delete(ctx context.Context, id int) error {
	key := ds.NewKey(ds.ALERT)
	key.ID = int64(id)

	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		cfg := alerts.NewConfig()
		if err := tx.Get(key, cfg); err != nil {
			return fmt.Errorf("Failed to retrieve from datastore: %s", err)
		}

		// Set both State and StateAsString to deleted since State is used for filtering
		// in the List() func.
		cfg.StateAsString = alerts.DELETED
		cfg.State = alerts.ConfigStateToInt(alerts.DELETED)

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

// List implements the alerts.Store interface.
func (s *DSAlertStore) List(ctx context.Context, includeDeleted bool) ([]*alerts.Alert, error) {
	ret := []*alerts.Alert{}
	q := ds.NewQuery(ds.ALERT)
	if !includeDeleted {
		q = q.Filter("State =", alerts.ConfigStateToInt(alerts.ACTIVE))
	}
	it := ds.DS.Run(ctx, q)
	for {
		cfg := alerts.NewConfig()
		// NewConfig sets these values, but we want them cleared in this case so
		// upgradeAlert can do its work.
		cfg.DirectionAsString = ""
		cfg.StateAsString = ""
		k, err := it.Next(cfg)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("Failed retrieving alert list: %s", err)
		}
		cfg.ID = k.ID
		cfg.IDAsString = fmt.Sprintf("%d", k.ID)
		if err := cfg.Validate(); err != nil {
			sklog.Errorf("Found an invalid alert %v: %s", *cfg, err)
		}
		upgradeAlert(cfg)
		ret = append(ret, cfg)
	}

	sort.Sort(configSlice(ret))
	return ret, nil
}

// upgradeAlert migrates the legacy Direction and State properties into their
// new string based forms.
//
// Note that this will only affect an Alert once, i.e. once an Alert has been
// saved back into the datastore then the string version of the property is
// considered the source of truth and the integer values are then subsequently
// ignored.
func upgradeAlert(a *alerts.Alert) {
	if a.DirectionAsString == "" {
		// Convert legacy int values to the new string values.
		switch a.Direction {
		case 0:
			a.DirectionAsString = alerts.BOTH
		case 1:
			a.DirectionAsString = alerts.UP
		case 2:
			a.DirectionAsString = alerts.DOWN
		default:
			a.DirectionAsString = alerts.BOTH
		}
	}

	if a.StateAsString == "" {
		switch a.State {
		case 0:
			a.StateAsString = alerts.ACTIVE
		case 1:
			a.StateAsString = alerts.DELETED
		default:
			a.StateAsString = alerts.DELETED
		}
	}
}

// Confirm this Google Cloud Datastore implements the AlertStore interface.
var _ alerts.Store = (*DSAlertStore)(nil)
