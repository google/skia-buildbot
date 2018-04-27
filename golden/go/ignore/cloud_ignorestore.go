package ignore

import (
	"context"
	"sort"

	"golang.org/x/sync/errgroup"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/dsutil"
)

type cloudIgnoreStore struct {
	client     *datastore.Client
	listHelper *dsutil.ListHelper
	eventBus   eventbus.EventBus
}

func NewCloudExpectationsStore(client *datastore.Client) (IgnoreStore, error) {
	if client == nil {
		return nil, sklog.FmtErrorf("Received nil for datastore client.")
	}

	store := &cloudIgnoreStore{
		client: client,
	}
	return store, nil
}

// Create implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Create(ignoreRule *IgnoreRule) error {
	createFn := func(tx *datastore.Transaction) error {
		key := dsutil.TimedKey(ds.IGNORE_RULE)
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
	if _, err := c.client.RunInTransaction(context.Background(), createFn); err != nil {
		return err
	}
	return nil
}

// List implements the IgnoreStore interface.
func (c *cloudIgnoreStore) List(addCounts bool) ([]*IgnoreRule, error) {
	ctx := context.Background()
	var egroup errgroup.Group
	var keys []*datastore.Key
	egroup.Go(func() error {
		// Query all entities.
		query := datastore.NewQuery(string(ds.IGNORE_RULE)).KeysOnly()
		var err error
		keys, err = c.client.GetAll(ctx, query, nil)
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
	keys = recentKeys.Merge(keys)
	if len(keys) == 0 {
		return []*IgnoreRule{}, nil
	}

	ret := make([]*IgnoreRule, len(keys))
	if err := c.client.GetMulti(ctx, keys, ret); err != nil {
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
	return err
}

// Delete implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Delete(id int64, userId string) (int, error) { return 0, nil }

// Revision implements the IgnoreStore interface.
func (c *cloudIgnoreStore) Revision() int64 { return 0 }

// BuildRuleMatcher implements the IgnoreStore interface.
func (c *cloudIgnoreStore) BuildRuleMatcher() (RuleMatcher, error) { return nil, nil }
