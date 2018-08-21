package unthrottle

import (
	"context"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
)

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

// Unthrottle the given roller.
func Unthrottle(ctx context.Context, roller string) error {
	return set(ctx, roller, true)
}

// Reset the should-unthrottle status of the roller.
func Reset(ctx context.Context, roller string) error {
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

// Determine whether the given roller should be unthrottled.
func Get(ctx context.Context, roller string) (bool, error) {
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
