package unthrottle

import (
	"context"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
)

// Throttle tracks whether a given roller should be throttled.
// TODO(borenet): This should include throttle-enabling as well as disabling,
// and it should maintain history like modes and strategies.
type Throttle interface {
	// Unthrottle the given roller.
	Unthrottle(ctx context.Context, roller string) error
	// Reset the should-unthrottle status of the roller, allowing it to become
	// throttled again if necessary.
	Reset(ctx context.Context, roller string) error
	// Get determines whether the given roller should be unthrottled.
	Get(ctx context.Context, roller string) (bool, error)
}

// entry is a struct representing the should-unthrottle status for a roller.
type entry struct {
	ShouldUnthrottle bool `datastore:"shouldUnthrottle,noindex"`
}

// Fake ancestor we supply for unthrottling, to force consistency.
// We lose some performance this way but it keeps our tests from
// flaking.
func fakeAncestor() *datastore.Key {
	rv := ds.NewKey(ds.KIND_AUTOROLL_UNTHROTTLE_ANCESTOR)
	rv.ID = 13 // Bogus ID.
	return rv
}

// Return a datastore key for the given roller.
func key(roller string) *datastore.Key {
	k := ds.NewKey(ds.KIND_AUTOROLL_UNTHROTTLE)
	k.Parent = fakeAncestor()
	k.Name = roller + "_unthrottle"
	return k
}

// DatastoreThrottle is an implementation of Throttle which uses Datastore.
type DatastoreThrottle struct{}

// NewDatastore returns an implementation of Throttle which uses Datastore.
func NewDatastore(ctx context.Context) *DatastoreThrottle {
	return &DatastoreThrottle{}
}

// Unthrottle implements the Throttle interface.
func (t *DatastoreThrottle) Unthrottle(ctx context.Context, roller string) error {
	return set(ctx, roller, true)
}

// Reset implements the Throttle interface.
func (t *DatastoreThrottle) Reset(ctx context.Context, roller string) error {
	return set(ctx, roller, false)
}

// Set whether the given roller should be unthrottled.
func set(ctx context.Context, roller string, shouldUnthrottle bool) error {
	e := &entry{
		ShouldUnthrottle: shouldUnthrottle,
	}
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		_, err := tx.Put(key(roller), e)
		return err
	})
	return err
}

// Get implements the Throttle interface.
func (t *DatastoreThrottle) Get(ctx context.Context, roller string) (bool, error) {
	var e entry
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		return tx.Get(key(roller), &e)
	})
	if err != nil {
		if err == datastore.ErrNoSuchEntity {
			return false, nil
		} else {
			return false, err
		}
	}
	return e.ShouldUnthrottle, nil
}

var _ Throttle = &DatastoreThrottle{}
