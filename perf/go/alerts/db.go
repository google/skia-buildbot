// Save and retrieve alerts.Config's to/from a database.
//
// TODO(jcgregorio) Add a cleanup process that removes DELETED configs from the
// database after a long period of time, using the lastmodified timestamp.
package alerts

import (
	"context"
	"fmt"
	"sort"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/iterator"
)

// Store persists Config's to/from datastore.
type Store struct {
}

// NewStore returns a new Store.
func NewStore() *Store {
	return &Store{}
}

// Save can write a new, or update an existing, Config. New
// Config's will have an ID of -1.
func (s *Store) Save(cfg *Alert) error {
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("Failed to save invalid Config: %s", err)
	}
	key := ds.NewKey(ds.ALERT)
	if cfg.ID != INVALID_ID {
		key.ID = int64(cfg.ID)
	}
	if _, err := ds.DS.Put(context.TODO(), key, cfg); err != nil {
		return fmt.Errorf("Failed to write to database: %s", err)
	}
	return nil
}

func (s *Store) Delete(id int) error {
	key := ds.NewKey(ds.ALERT)
	key.ID = int64(id)

	_, err := ds.DS.RunInTransaction(context.TODO(), func(tx *datastore.Transaction) error {
		cfg := NewConfig()
		if err := tx.Get(key, cfg); err != nil {
			return fmt.Errorf("Failed to retrieve from datastore: %s", err)
		}
		cfg.State = DELETED
		if _, err := tx.Put(key, cfg); err != nil {
			return fmt.Errorf("Failed to write to database: %s", err)
		}
		return nil
	})
	return err
}

// ConfigSlice is a utility type for sorting Configs by DisplayName.
type ConfigSlice []*Alert

func (p ConfigSlice) Len() int           { return len(p) }
func (p ConfigSlice) Less(i, j int) bool { return p[i].DisplayName < p[j].DisplayName }
func (p ConfigSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func (s *Store) List(includeDeleted bool) ([]*Alert, error) {
	ret := []*Alert{}
	q := ds.NewQuery(ds.ALERT)
	if !includeDeleted {
		q = q.Filter("State =", int(ACTIVE))
	}
	it := ds.DS.Run(context.TODO(), q)
	for {
		cfg := NewConfig()
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
