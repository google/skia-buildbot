// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"fmt"
	"math"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/datastore"
	dm "github.com/luci/luci-go/dm/api/service/v1"
)

// QuestKeyFromID makes a datastore.Key given the QuestID.
func QuestKeyFromID(c context.Context, qid string) *datastore.Key {
	return datastore.Get(c).MakeKey("Quest", qid)
}

// QuestFromID produces an empty Quest model from the QuestID.
func QuestFromID(qid string) *Quest {
	return &Quest{ID: qid}
}

// QuestIDFromKey makes a QuestID from the given datastore.Key. It panics if the
// Key does not point to a Quest.
func QuestIDFromKey(k *datastore.Key) string {
	if k.Kind() != "Quest" || k.Parent() != nil {
		panic(fmt.Errorf("invalid Quest key: %s", k))
	}
	return k.StringID()
}

// AttemptKeyFromID makes a datastore.Key given the AttemptID.
func AttemptKeyFromID(c context.Context, aid *dm.Attempt_ID) *datastore.Key {
	return datastore.Get(c).MakeKey("Attempt", aid.DMEncoded())
}

// AttemptFromID produces an empty Attempt model from the AttemptID.
func AttemptFromID(aid *dm.Attempt_ID) *Attempt {
	ret := &Attempt{}
	ret.ID = *aid
	return ret
}

// AttemptIDFromKey makes a AttemptID from the given datastore.Key. It panics if the
// Key does not point to a Attempt.
func AttemptIDFromKey(k *datastore.Key) *dm.Attempt_ID {
	if k.Kind() != "Attempt" || k.Parent() != nil {
		panic(fmt.Errorf("invalid Attempt key: %s", k))
	}
	ret := &dm.Attempt_ID{}
	if err := ret.SetDMEncoded(k.StringID()); err != nil {
		panic(fmt.Errorf("invalid Attempt key: %s: %s", k, err))
	}
	return ret
}

// ExecutionKeyFromID makes a datastore.Key given the ExecutionID.
func ExecutionKeyFromID(c context.Context, eid *dm.Execution_ID) *datastore.Key {
	return datastore.Get(c).MakeKey("Attempt", eid.AttemptID().DMEncoded(), "Execution", eid.Id)
}

// ExecutionFromID produces an empty Execution model from the ExecutionID.
func ExecutionFromID(c context.Context, eid *dm.Execution_ID) *Execution {
	ret := &Execution{}
	ret.ID = invertedHexUint32(eid.Id)
	ret.Attempt = AttemptKeyFromID(c, eid.AttemptID())
	return ret
}

// ExecutionIDFromKey makes a ExecutionID from the given datastore.Key. It panics if the
// Key does not point to a Execution.
func ExecutionIDFromKey(k *datastore.Key) *dm.Execution_ID {
	if k.Kind() != "Execution" || k.Parent() == nil {
		panic(fmt.Errorf("invalid Execution key: %s", k))
	}
	id := k.IntID()
	if id <= 0 || id > math.MaxUint32 {
		panic(fmt.Errorf("invalid Execution key: %s", k))
	}
	atmpt := AttemptIDFromKey(k.Parent())
	return &dm.Execution_ID{Quest: atmpt.Quest, Attempt: atmpt.Id, Id: uint32(id)}
}
