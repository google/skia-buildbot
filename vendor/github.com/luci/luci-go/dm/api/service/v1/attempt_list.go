// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import (
	"errors"
	"sort"

	"github.com/xtgo/set"
)

// Normalize sorts and uniq's attempt nums
func (a *AttemptList) Normalize() error {
	for q, vals := range a.GetTo() {
		if vals == nil {
			a.To[q] = &AttemptList_Nums{}
		} else {
			if err := vals.Normalize(); err != nil {
				return err
			}
		}
	}
	return nil
}

type revUint32Slice []uint32

func (u revUint32Slice) Len() int      { return len(u) }
func (u revUint32Slice) Swap(i, j int) { u[i], u[j] = u[j], u[i] }

// Less intentionally reverses the comparison of i and j.
func (u revUint32Slice) Less(i, j int) bool { return u[j] < u[i] }

// Normalize sorts and uniq's attempt nums. If Nums equals [0], [] or nil,
// it implies all attempts for the quest and will be normalized to nil.
//
// It is an error for Nums to contain 0 as well as other numbers.
func (a *AttemptList_Nums) Normalize() error {
	if len(a.Nums) == 0 || (len(a.Nums) == 1 && a.Nums[0] == 0) {
		a.Nums = nil
		return nil
	}
	slc := revUint32Slice(a.Nums)
	sort.Sort(slc)
	if a.Nums[len(a.Nums)-1] == 0 {
		return errors.New("AttemptList.Nums contains 0 as well as other values.")
	}
	a.Nums = a.Nums[:set.Uniq(slc)]
	return nil
}

// NewAttemptList is a convenience method for making a normalized
// *AttemptList with a pre-normalized literal map of quest -> attempt nums.
//
// If the provided data is invalid, this method will panic.
func NewAttemptList(data map[string][]uint32) *AttemptList {
	ret := &AttemptList{To: make(map[string]*AttemptList_Nums, len(data))}
	for qst, atmpts := range data {
		nums := &AttemptList_Nums{Nums: atmpts}
		if err := nums.Normalize(); err != nil {
			panic(err)
		}
		ret.To[qst] = nums
	}
	return ret
}

// AddAIDs adds the given Attempt_ID to the AttemptList
func (a *AttemptList) AddAIDs(aids ...*Attempt_ID) {
	for _, aid := range aids {
		if a.To == nil {
			a.To = map[string]*AttemptList_Nums{}
		}
		atmptNums := a.To[aid.Quest]
		if atmptNums == nil {
			atmptNums = &AttemptList_Nums{}
			a.To[aid.Quest] = atmptNums
		}
		atmptNums.Nums = append(atmptNums.Nums, aid.Id)
	}
}

// Dup does a deep copy of this AttemptList.
func (a *AttemptList) Dup() *AttemptList {
	ret := &AttemptList{}
	for k, v := range a.To {
		if ret.To == nil {
			ret.To = make(map[string]*AttemptList_Nums, len(a.To))
		}
		vals := &AttemptList_Nums{Nums: make([]uint32, len(v.Nums))}
		copy(vals.Nums, v.Nums)
		ret.To[k] = vals
	}
	return ret
}

// UpdateWith updates this AttemptList with all the entries in the other
// AttemptList.
func (a *AttemptList) UpdateWith(o *AttemptList) {
	for qid, atmpts := range o.To {
		if curAtmpts, ok := a.To[qid]; !ok {
			a.To[qid] = atmpts
		} else {
			curAtmpts.Nums = append(curAtmpts.Nums, atmpts.Nums...)
			curAtmpts.Normalize()
		}
	}
}
