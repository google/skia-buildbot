// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package mutate

import (
	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/dm/api/service/v1"
	"github.com/luci/luci-go/dm/appengine/model"
	"github.com/luci/luci-go/tumble"
	"golang.org/x/net/context"
)

// MaxEnsureAttempts limits the maximum number of EnsureAttempt entities that
// the EnsureQuestAttempts mutation will emit. If there are more AttemptIDs
// than this maximum, then EnsureQuestAttempts will do tail-recursion to process
// the remainder.
const MaxEnsureAttempts = 10

// EnsureQuestAttempts ensures that the given Attempt exists. If it doesn't, it's
// created in a NeedsExecution state.
type EnsureQuestAttempts struct {
	Quest *model.Quest
	AIDs  []uint32

	// DoNotMergeQuest causes this mutation to not attempt to merge the BuiltBy of
	// Quest.
	DoNotMergeQuest bool
}

// Root implements tumble.Mutation.
func (e *EnsureQuestAttempts) Root(c context.Context) *datastore.Key {
	return model.QuestKeyFromID(c, e.Quest.ID)
}

// RollForward implements tumble.Mutation.
func (e *EnsureQuestAttempts) RollForward(c context.Context) (muts []tumble.Mutation, err error) {
	if !e.DoNotMergeQuest {
		if _, err = (&MergeQuest{Quest: e.Quest}).RollForward(c); err != nil {
			return
		}
	}

	if len(e.AIDs) > 0 {
		lim := len(e.AIDs)
		if lim > MaxEnsureAttempts {
			lim = MaxEnsureAttempts + 1
		}
		muts = make([]tumble.Mutation, 0, lim)
		for i, aid := range e.AIDs {
			if i > MaxEnsureAttempts {
				muts = append(muts, &EnsureQuestAttempts{e.Quest, e.AIDs[i:], true})
				break
			}
			muts = append(muts, &EnsureAttempt{dm.NewAttemptID(e.Quest.ID, aid)})
		}
	}

	return
}

func init() {
	tumble.Register((*EnsureQuestAttempts)(nil))
}
