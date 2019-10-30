package ds_ignorestore

import (
	"context"
	"sort"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	ttlcache "github.com/patrickmn/go-cache"
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
	ignoreCache    *ttlcache.Cache
}

const (
	// We could probably cache this longer, since we only expect there to be one
	// reader and writer (and we dump the cache on create/update/delete)
	ignoreCacheFreshness = 5 * time.Minute

	listCacheKey = "listCacheKey"
)

// dsRule represents how rules are stored in DataStore. This may be distinct to how
// they are represented by the frontend.
type dsRule struct {
	ID        int64
	Name      string
	UpdatedBy string
	Expires   time.Time
	Query     string
	Note      string
}

func fromRule(r *ignore.Rule) (*dsRule, error) {
	var id int64
	if r.ID != "" {
		var err error
		id, err = strconv.ParseInt(r.ID, 10, 64)
		if err != nil {
			return nil, skerr.Wrapf(err, "id must be int64: %q", id)
		}
	}

	return &dsRule{
		ID:        id,
		Name:      r.Name,
		UpdatedBy: r.UpdatedBy,
		Expires:   r.Expires,
		Query:     r.Query,
		Note:      r.Note,
	}, nil
}

func (r *dsRule) ToRule() *ignore.Rule {
	return &ignore.Rule{
		ID:        strconv.FormatInt(r.ID, 10),
		Name:      r.Name,
		UpdatedBy: r.UpdatedBy,
		Expires:   r.Expires,
		Query:     r.Query,
		Note:      r.Note,
	}
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
		ignoreCache:    ttlcache.New(ignoreCacheFreshness, ignoreCacheFreshness),
	}
	return store, nil
}

// Create implements the IgnoreStore interface.
func (c *DSIgnoreStore) Create(ctx context.Context, ir *ignore.Rule) error {
	c.ignoreCache.Delete(listCacheKey)

	r, err := fromRule(ir)
	if err != nil {
		return skerr.Wrap(err)
	}

	createFn := func(tx *datastore.Transaction) error {
		key := dsutil.TimeSortableKey(ds.IGNORE_RULE, 0)
		r.ID = key.ID

		// Add the new rule and put its key with the recently added keys.
		if _, err := tx.Put(key, r); err != nil {
			return err
		}
		ir.ID = strconv.FormatInt(r.ID, 10)

		return c.recentKeysList.Add(tx, key)
	}

	// Run the relevant updates in a transaction.
	_, err = c.client.RunInTransaction(ctx, createFn)
	return skerr.Wrap(err)
}

// List implements the IgnoreStore interface.
func (c *DSIgnoreStore) List(ctx context.Context) ([]*ignore.Rule, error) {
	if rules, ok := c.ignoreCache.Get(listCacheKey); ok {
		rv, ok := rules.([]*ignore.Rule)
		if ok {
			return rv, nil
		}
		sklog.Warningf("corrupt data in cache, refetching")
		c.ignoreCache.Delete(listCacheKey)
	}
	var egroup errgroup.Group
	var queriedKeys []*datastore.Key
	egroup.Go(func() error {
		// Query all entities.
		query := ds.NewQuery(ds.IGNORE_RULE).KeysOnly()
		var err error
		queriedKeys, err = c.client.GetAll(ctx, query, nil)
		return skerr.Wrap(err)
	})

	var recently *dsutil.Recently
	egroup.Go(func() error {
		var err error
		recently, err = c.recentKeysList.GetRecent()
		return skerr.Wrap(err)
	})

	if err := egroup.Wait(); err != nil {
		return nil, skerr.Wrapf(err, "getting keys of ignore rules")
	}

	// Merge the keys to get all of the current keys.
	allKeys := recently.Combine(queriedKeys)
	if len(allKeys) == 0 {
		return []*ignore.Rule{}, nil
	}

	rules := make([]*dsRule, len(allKeys))
	if err := c.client.GetMulti(ctx, allKeys, rules); err != nil {
		return nil, skerr.Wrap(err)
	}

	ret := make([]*ignore.Rule, 0, len(rules))
	for _, r := range rules {
		ret = append(ret, r.ToRule())
	}

	sort.Slice(ret, func(i, j int) bool { return ret[i].Expires.Before(ret[j].Expires) })

	c.ignoreCache.SetDefault(listCacheKey, ret)
	return ret, nil
}

// Update implements the IgnoreStore interface.
func (c *DSIgnoreStore) Update(ctx context.Context, id string, ir *ignore.Rule) error {
	r, err := fromRule(ir)
	if err != nil {
		return skerr.Wrap(err)
	}
	key := ds.NewKey(ds.IGNORE_RULE)
	key.ID, err = strconv.ParseInt(id, 10, 64)
	if err != nil {
		return skerr.Wrapf(err, "id must be int64: %q", id)
	}
	c.ignoreCache.Delete(listCacheKey)
	_, err = c.client.Mutate(ctx, datastore.NewUpdate(key, r))
	return skerr.Wrap(err)
}

// Delete implements the IgnoreStore interface.
func (c *DSIgnoreStore) Delete(ctx context.Context, idStr string) (int, error) {
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, skerr.Wrapf(err, "id must be int64: %q", idStr)
	}
	if id <= 0 {
		return 0, skerr.Fmt("Given id does not exist: %d", id)
	}

	deleteFn := func(tx *datastore.Transaction) error {
		key := ds.NewKey(ds.IGNORE_RULE)
		key.ID = id

		ir := &dsRule{}
		if err := tx.Get(key, ir); err != nil {
			return skerr.Wrap(err)
		}

		if err := tx.Delete(key); err != nil {
			return skerr.Wrap(err)
		}

		return c.recentKeysList.Delete(tx, key)
	}

	c.ignoreCache.Delete(listCacheKey)
	// Run the relevant updates in a transaction.
	_, err = c.client.RunInTransaction(ctx, deleteFn)
	if err != nil {
		// Don't report an error if the item did not exist.
		if skerr.Unwrap(err) == datastore.ErrNoSuchEntity {
			sklog.Warningf("Could not delete ignore with id %d because it did not exist", id)
			return 0, nil
		}
		return 0, skerr.Wrap(err)
	}

	return 1, nil
}
