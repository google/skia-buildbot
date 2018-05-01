package ignore

import (
	"context"
	"sort"
	"sync/atomic"

	"cloud.google.com/go/datastore"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/dsutil"
)

type cloudIgnoreStore struct {
	client     *datastore.Client
	listHelper *dsutil.ListHelper
	revision   int64
}

func NewCloudIgnoreStore(client *datastore.Client) (IgnoreStore, error) {
	if client == nil {
		return nil, sklog.FmtErrorf("Received nil for datastore client.")
	}

	store := &cloudIgnoreStore{
		client:     client,
		listHelper: dsutil.NewListHelper(client, ds.RECENT_KEYS, "ignore-recent-entries", dsutil.DefaultConsistencyDelta),
	}
	return store, nil
}

// Create implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Create(ignoreRule *IgnoreRule) error {
	createFn := func(tx *datastore.Transaction) error {
		key := dsutil.TimeSortableKey(ds.IGNORE_RULE, 0)
		ignoreRule.ID = key.ID
		var egroup errgroup.Group
		egroup.Go(func() error {
			_, err := tx.Put(key, ignoreRule)
			return err
		})
		egroup.Go(func() error {
			return c.listHelper.Add(tx, key)
		})

		return egroup.Wait()
	}

	// Run the relevant updates in a transaction.
	_, err := c.client.RunInTransaction(context.Background(), createFn)
	if err == nil {
		atomic.AddInt64(&c.revision, 1)
	}
	return err
}

// List implements the IgnoreStore interface.
func (c *cloudIgnoreStore) List(addCounts bool) ([]*IgnoreRule, error) {
	ctx := context.Background()
	var egroup errgroup.Group
	var queriedKeys []*datastore.Key
	egroup.Go(func() error {
		// Query all entities.
		query := datastore.NewQuery(string(ds.IGNORE_RULE)).KeysOnly()
		var err error
		queriedKeys, err = c.client.GetAll(ctx, query, nil)
		return err
	})

	var recentKeys dsutil.KeySlice
	egroup.Go(func() error {
		var err error
		recentKeys, err = c.listHelper.GetRecent()
		return err
	})

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	// Merge the keys to get all of the current keys.
	allKeys := recentKeys.Merge(queriedKeys)
	if len(allKeys) == 0 {
		return []*IgnoreRule{}, nil
	}

	ret := make([]*IgnoreRule, len(allKeys))
	if err := c.client.GetMulti(ctx, allKeys, ret); err != nil {
		return nil, err
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Expires.Before(ret[j].Expires) })
	return ret, nil
}

// Update implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Update(id int64, rule *IgnoreRule) error {
	ctx := context.Background()
	key := ds.NewKey(ds.IGNORE_RULE)
	key.ID = id
	ignoreRule := &IgnoreRule{}
	if err := c.client.Get(ctx, key, ignoreRule); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return sklog.FmtErrorf("Entity with id %d does not exist.", id)
		}
		return sklog.FmtErrorf("Error retrieving entity %d", id)
	}

	_, err := c.client.Put(ctx, key, rule)
	if err == nil {
		atomic.AddInt64(&c.revision, 1)
	}
	return err
}

// Delete implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Delete(id int64, userId string) (int, error) {
	if id <= 0 {
		return 0, sklog.FmtErrorf("Given id does not exist: %d", id)
	}

	deleteFn := func(tx *datastore.Transaction) error {
		key := ds.NewKey(ds.IGNORE_RULE)
		key.ID = id

		ignoreRule := &IgnoreRule{}
		if err := tx.Get(key, ignoreRule); err != nil {
			return err
		}

		var egroup errgroup.Group
		egroup.Go(func() error { return tx.Delete(key) })
		egroup.Go(func() error { return c.listHelper.Delete(tx, key) })
		return egroup.Wait()
	}

	// Run the relevant updates in a transaction.
	_, err := c.client.RunInTransaction(context.Background(), deleteFn)
	if err != nil {
		// Don't report an error if the item did not exist.
		if err == datastore.ErrNoSuchEntity {
			return 0, nil
		}
		return 0, err
	}

	atomic.AddInt64(&c.revision, 1)
	return 1, nil
}

// Revision implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Revision() int64 {
	return atomic.LoadInt64(&c.revision)
}

// BuildRuleMatcher implements the IgnoreStore interface.
func (c *cloudIgnoreStore) BuildRuleMatcher() (RuleMatcher, error) {
	return buildRuleMatcher(c)
}
