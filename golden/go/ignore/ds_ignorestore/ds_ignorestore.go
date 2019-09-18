package ds_ignorestore

import (
	"context"
	"sort"
	"sync/atomic"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/dsutil"
	"go.skia.org/infra/golden/go/ignore"
	"golang.org/x/sync/errgroup"
)

// DSIgnoreStore implements the IgnoreStore interface
type DSIgnoreStore struct {
	client         *datastore.Client
	recentKeysList *dsutil.RecentKeysList
	revision       int64
}

// New returns an IgnoreStore instance that is backed by Cloud Datastore.
func New(client *datastore.Client) (*DSIgnoreStore, error) {
	if client == nil {
		return nil, skerr.Fmt("Received nil for datastore client.")
	}

	containerKey := ds.NewKey(ds.HELPER_RECENT_KEYS)
	containerKey.Name = "ignore:recent-keys"

	store := &DSIgnoreStore{
		client:         client,
		recentKeysList: dsutil.NewRecentKeysList(client, containerKey, dsutil.DefaultConsistencyDelta),
	}
	return store, nil
}

// Create implements the IgnoreStore interface.
func (c *DSIgnoreStore) Create(ignoreRule *ignore.Rule) error {
	createFn := func(tx *datastore.Transaction) error {
		key := dsutil.TimeSortableKey(ds.IGNORE_RULE, 0)
		ignoreRule.ID = key.ID

		// Add the new rule and put its key with the recently added keys.
		if _, err := tx.Put(key, ignoreRule); err != nil {
			return err
		}

		return c.recentKeysList.Add(tx, key)
	}

	// Run the relevant updates in a transaction.
	_, err := c.client.RunInTransaction(context.TODO(), createFn)

	// TODO(stephana): Look into removing the revision feature. I don't think
	// this is really necessary going forward.

	if err == nil {
		atomic.AddInt64(&c.revision, 1)
	}
	return err
}

// List implements the IgnoreStore interface.
func (c *DSIgnoreStore) List() ([]*ignore.Rule, error) {
	ctx := context.TODO()
	var egroup errgroup.Group
	var queriedKeys []*datastore.Key
	egroup.Go(func() error {
		// Query all entities.
		query := ds.NewQuery(ds.IGNORE_RULE).KeysOnly()
		var err error
		queriedKeys, err = c.client.GetAll(ctx, query, nil)
		return err
	})

	var recently *dsutil.Recently
	egroup.Go(func() error {
		var err error
		recently, err = c.recentKeysList.GetRecent()
		return err
	})

	if err := egroup.Wait(); err != nil {
		return nil, skerr.Fmt("Error getting keys of ignore rules: %s", err)
	}

	// Merge the keys to get all of the current keys.
	allKeys := recently.Combine(queriedKeys)
	if len(allKeys) == 0 {
		return []*ignore.Rule{}, nil
	}

	ret := make([]*ignore.Rule, len(allKeys))
	if err := c.client.GetMulti(ctx, allKeys, ret); err != nil {
		return nil, err
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Expires.Before(ret[j].Expires) })

	return ret, nil
}

// Update implements the IgnoreStore interface.
func (c *DSIgnoreStore) Update(id int64, rule *ignore.Rule) error {
	ctx := context.TODO()
	key := ds.NewKey(ds.IGNORE_RULE)
	key.ID = id
	_, err := c.client.Mutate(ctx, datastore.NewUpdate(key, rule))
	if err == nil {
		atomic.AddInt64(&c.revision, 1)
	}
	return err
}

// Delete implements the IgnoreStore interface.
func (c *DSIgnoreStore) Delete(id int64) (int, error) {
	if id <= 0 {
		return 0, skerr.Fmt("Given id does not exist: %d", id)
	}

	deleteFn := func(tx *datastore.Transaction) error {
		key := ds.NewKey(ds.IGNORE_RULE)
		key.ID = id

		ignoreRule := &ignore.Rule{}
		if err := tx.Get(key, ignoreRule); err != nil {
			return err
		}

		if err := tx.Delete(key); err != nil {
			return err
		}

		return c.recentKeysList.Delete(tx, key)
	}

	// Run the relevant updates in a transaction.
	_, err := c.client.RunInTransaction(context.TODO(), deleteFn)
	if err != nil {
		// Don't report an error if the item did not exist.
		if err == datastore.ErrNoSuchEntity {
			sklog.Warningf("Could not delete ignore with id %d because it did not exist", id)
			return 0, nil
		}
		return 0, err
	}

	atomic.AddInt64(&c.revision, 1)
	return 1, nil
}

// Revision implements the IgnoreStore interface.
func (c *DSIgnoreStore) Revision() int64 {
	return atomic.LoadInt64(&c.revision)
}

// BuildRuleMatcher implements the IgnoreStore interface.
func (c *DSIgnoreStore) BuildRuleMatcher() (ignore.RuleMatcher, error) {
	ir, err := c.List()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return ignore.BuildRuleMatcher(ir)
}
