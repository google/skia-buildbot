package ignore

import (
	"context"
	"sort"
	"sync/atomic"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/dsutil"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

// cloudIgnoreStore implements the IgnoreStore interface
type cloudIgnoreStore struct {
	client         *datastore.Client
	recentKeysList *dsutil.RecentKeysList
	revision       int64
	lastTilePair   *types.TilePair
	expStore       expstorage.ExpectationsStore
	tileStream     <-chan *types.TilePair
}

// NewCloudIgnoreStore returns an IgnoreStore instance that is backed by Cloud Datastore.
func NewCloudIgnoreStore(client *datastore.Client, expStore expstorage.ExpectationsStore, tileStream <-chan *types.TilePair) (IgnoreStore, error) {
	if client == nil {
		return nil, sklog.FmtErrorf("Received nil for datastore client.")
	}

	containerKey := ds.NewKey(ds.HELPER_RECENT_KEYS)
	containerKey.Name = "ignore:recent-keys"

	store := &cloudIgnoreStore{
		client:         client,
		recentKeysList: dsutil.NewRecentKeysList(client, containerKey, dsutil.DefaultConsistencyDelta),
		expStore:       expStore,
		tileStream:     tileStream,
	}
	return store, nil
}

// Create implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Create(ignoreRule *IgnoreRule) error {
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

// TODO(stephana): Remove the addCounts flag in the signature of the List function
// and expose the AddIgnoreCounts function. This would remove the expsStore and
// tileStream members of the cloudIgnoreStore struct and simplify the interface.

// List implements the IgnoreStore interface.
func (c *cloudIgnoreStore) List(addCounts bool) ([]*IgnoreRule, error) {
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
		return nil, sklog.FmtErrorf("Error getting keys of ignore rules: %s", err)
	}

	// Merge the keys to get all of the current keys.
	allKeys := recently.Combine(queriedKeys)
	if len(allKeys) == 0 {
		return []*IgnoreRule{}, nil
	}

	ret := make([]*IgnoreRule, len(allKeys))
	if err := c.client.GetMulti(ctx, allKeys, ret); err != nil {
		return nil, err
	}
	sort.Slice(ret, func(i, j int) bool { return ret[i].Expires.Before(ret[j].Expires) })

	if addCounts {
		var err error
		c.lastTilePair, err = addIgnoreCounts(ret, c, c.lastTilePair, c.expStore, c.tileStream)
		if err != nil {
			sklog.Errorf("Unable to add counts to ignore list result: %s", err)
		}
	}

	return ret, nil
}

// Update implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Update(id int64, rule *IgnoreRule) error {
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
func (c *cloudIgnoreStore) Delete(id int64) (int, error) {
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
