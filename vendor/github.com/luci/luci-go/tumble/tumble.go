// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"fmt"

	"github.com/luci/gae/filter/txnBuf"
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

// RunMutation immediately runs the Mutation `m` in a transaction. This method
// should be used to start a tumble chain when you have transactional checks
// to do (e.g. `m` implements the first transactional link in the chain).
//
// Usually this is called from your application's handlers to begin a tumble
// state machine as a result of some API interaction.
func RunMutation(c context.Context, m Mutation) error {
	cfg := getConfig(c)
	shardSet, _, _, err := enterTransactionInternal(txnBuf.FilterRDS(c), cfg, m, 0)
	if err != nil {
		return err
	}
	fireTasks(c, cfg, shardSet)
	return nil
}

func enterTransactionInternal(c context.Context, cfg *Config, m Mutation, round uint64) (map[taskShard]struct{}, []Mutation, []*datastore.Key, error) {
	fromRoot := m.Root(c)

	if fromRoot == nil {
		return nil, nil, nil, fmt.Errorf("tumble: Passing nil as fromRoot is illegal")
	}

	shardSet := map[taskShard]struct{}(nil)
	retMuts := []Mutation(nil)
	retMutKeys := []*datastore.Key(nil)

	err := datastore.Get(c).RunInTransaction(func(c context.Context) error {
		// do a Get on the fromRoot to ensure that this transaction is associated
		// with that entity group.
		_, _ = datastore.Get(c).Exists(fromRoot)

		muts, err := m.RollForward(c)
		if err != nil {
			return err
		}

		retMuts = muts
		shardSet, retMutKeys, err = putMutations(c, cfg, fromRoot, muts, round)

		return err
	}, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	return shardSet, retMuts, retMutKeys, nil
}

// PutNamedMutations writes the provided named mutations to the datastore.
//
// Named mutations are singletons children of a given `parent`. So for any given
// `parent`, there can be only one mutation with any given name.  Named
// mutations, by design, cannot collide with unnamed (anonymous) mutations.
// `parent` does NOT need to be a root Key; the mutations will be created as
// direct descendants of the provided parent entity. The names of the various
// mutations should be valid datastore key StringIDs, which generally means that
// they should be UTF-8 strings.
//
// The current implementation reserves 2 characters of the StringID (as of
// writing this means that a named mutation name may only be 498 bytes long).
//
// This can be used to leverage tumble for e.g. cancellable, delayed cleanup
// tasks (like timeouts).
//
// This may be called from within an existing datastore transaction which
// includes `parent` to make this Put atomic with the remainder of the
// transaction.
//
// If called multiple times with the same name, the newly named mutation will
// overwrite the existing mutation (assuming it hasn't run already).
func PutNamedMutations(c context.Context, parent *datastore.Key, muts map[string]Mutation) error {
	cfg := getConfig(c)

	now := clock.Now(c).UTC()

	shardSet := map[taskShard]struct{}{}
	toPut := make([]*realMutation, 0, len(muts))
	for name, m := range muts {
		realMut, err := newRealMutation(c, cfg, "n:"+name, parent, m, now)
		if err != nil {
			logging.WithError(err).Errorf(c, "error creating real mutation for %v", m)
			return err
		}
		toPut = append(toPut, realMut)
		shardSet[realMut.shard(cfg)] = struct{}{}
	}

	err := datastore.Get(c).Put(toPut)
	fireTasks(c, getConfig(c), shardSet)
	return err
}

// CancelNamedMutations does a best-effort cancellation of the named mutations.
func CancelNamedMutations(c context.Context, parent *datastore.Key, names ...string) error {
	ds := datastore.Get(c)
	toDel := make([]*datastore.Key, 0, len(names))
	nameSet := stringset.NewFromSlice(names...)
	nameSet.Iter(func(name string) bool {
		toDel = append(toDel, ds.NewKey("tumble.Mutation", "n:"+name, 0, parent))
		return true
	})
	return errors.Filter(ds.Delete(toDel), datastore.ErrNoSuchEntity)
}
