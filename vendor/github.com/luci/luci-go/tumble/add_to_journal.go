// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tumble

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/gae/service/datastore/serialize"
	"github.com/luci/luci-go/common/logging"
	"golang.org/x/net/context"
)

// AddToJournal records one or more Mutation to the tumble journal, but does not
// execute any of them. This does so by running a transaction on a pseudo-random
// entity group, and journaling the mutations there.
func AddToJournal(c context.Context, m ...Mutation) error {
	(logging.Fields{"count": len(m)}).Infof(c, "tumble.AddToJournal")
	if len(m) == 0 {
		return nil
	}
	return RunMutation(c, addToJournalMutation(m))
}

type addToJournalMutation []Mutation

func (a addToJournalMutation) Root(c context.Context) *datastore.Key {
	hsh := sha256.New()
	for _, m := range a {
		hsh.Write(serialize.ToBytesWithContext(m.Root(c)))
	}
	return datastore.Get(c).MakeKey("tumble.temp", hex.EncodeToString(hsh.Sum(nil)))
}

func (a addToJournalMutation) RollForward(c context.Context) ([]Mutation, error) {
	return a, nil
}
