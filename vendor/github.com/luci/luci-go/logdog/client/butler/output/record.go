// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package output

import (
	"container/list"
	"fmt"
	"sort"
	"sync"

	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"
)

// Range marks an inclusive log entry index range [Start-End].
type Range struct {
	Start uint64
	End   uint64
}

func (sr *Range) String() string {
	if sr.Start == sr.End {
		return fmt.Sprintf("[%d]", sr.Start)
	}
	return fmt.Sprintf("[%d-%d]", sr.Start, sr.End)
}

// StreamEntryRecord tracks an individual range of indices from the same stream.
type StreamEntryRecord struct {
	// Ranges is the sorted set of Range observed for this stream.
	Ranges []Range
}

// EntryRecord is a record of which log entries have been sent for a given
// stream.
type EntryRecord struct {
	// Streams is a map of a given stream to its record.
	Streams map[types.StreamPath]*StreamEntryRecord
}

type streamEntryTracker struct {
	sync.Mutex

	// ranges is a sorted linked-list of *Range elements, each of which
	// represents a contiguous range of stream indexes.
	ranges list.List
}

func (t *streamEntryTracker) record() *StreamEntryRecord {
	t.Lock()
	defer t.Unlock()

	r := StreamEntryRecord{
		Ranges: make([]Range, 0, t.ranges.Len()),
	}
	for f := t.ranges.Front(); f != nil; f = f.Next() {
		r.Ranges = append(r.Ranges, *(f.Value.(*Range)))
	}
	return &r
}

func (t *streamEntryTracker) track(logs []*logpb.LogEntry) {
	if len(logs) == 0 {
		return
	}

	idxs := make(uint64Slice, len(logs))
	for i, le := range logs {
		idxs[i] = le.StreamIndex
	}
	sort.Sort(idxs)

	var ranges []*Range
	var lr *Range
	for _, idx := range idxs {
		if lr == nil || idx > (lr.End+1) {
			lr = &Range{Start: idx, End: idx}
			ranges = append(ranges, lr)
		} else {
			lr.End = idx
		}
	}

	t.Lock()
	defer t.Unlock()
	for _, rng := range ranges {
		t.mergeRangeLocked(rng)
	}
}

func (t *streamEntryTracker) mergeRangeLocked(r *Range) {
	e := t.getRangeElementBeforeLocked(r.Start)
	if e != nil {
		// Can we merge "r" into "e"?
		est := e.Value.(*Range)
		if (est.End + 1) >= r.Start {
			if est.End < r.End {
				est.End = r.End
			}
			r = est
		} else {
			// Disjoint between "e" and "r", add a new element for "r".
			e = t.ranges.InsertAfter(r, e)
		}
	} else {
		// No elements before "r"; push to front.
		e = t.ranges.PushFront(r)
	}

	// Merge right.
	for re := e.Next(); re != nil; re = e.Next() {
		rst := re.Value.(*Range)
		if rst.Start > (r.End + 1) {
			// Disjoint, done merging right.
			break
		}

		// Merge rst into r.
		if rst.End > r.End {
			r.End = rst.End
		}
		t.ranges.Remove(re)
	}
}

func (t *streamEntryTracker) getRangeElementBeforeLocked(idx uint64) *list.Element {
	// NOTE: We could optimize this search to perform a binary search on minimally
	// the right half of the list or, if we track center, the whole list. This
	// seems like overkill for debug code, though.
	var last *list.Element
	for cur := t.ranges.Front(); cur != nil; last, cur = cur, cur.Next() {
		if sr := cur.Value.(*Range); sr.Start > idx {
			break
		}
	}
	return last
}

// EntryTracker tracks individual which log entries have been sent for any
// given log entry stream.
type EntryTracker struct {
	mu      sync.Mutex
	streams map[types.StreamPath]*streamEntryTracker
}

// Record exports a snapshot of the current tracking state.
func (o *EntryTracker) Record() *EntryRecord {
	o.mu.Lock()
	defer o.mu.Unlock()

	var r EntryRecord
	if len(o.streams) > 0 {
		r.Streams = make(map[types.StreamPath]*StreamEntryRecord, len(o.streams))
		for k, v := range o.streams {
			r.Streams[k] = v.record()
		}
	}
	return &r
}

// Track adds the log entries contained in the supplied bundle to the record.
func (o *EntryTracker) Track(b *logpb.ButlerLogBundle) {
	entries := b.GetEntries()
	if len(entries) == 0 {
		return
	}

	st := make([]*streamEntryTracker, len(entries))
	func() {
		o.mu.Lock()
		defer o.mu.Unlock()

		for i, e := range entries {
			st[i] = o.getOrCreateStreamLocked(e.GetDesc().Path())
		}
	}()

	for i, e := range entries {
		st[i].track(e.GetLogs())
	}
}

func (o *EntryTracker) getOrCreateStreamLocked(s types.StreamPath) *streamEntryTracker {
	v, ok := o.streams[s]
	if !ok {
		v = &streamEntryTracker{}

		if o.streams == nil {
			o.streams = make(map[types.StreamPath]*streamEntryTracker)
		}
		o.streams[s] = v
	}
	return v
}

// uint64Slice is an ascendingly-sortable slice of uint64
type uint64Slice []uint64

func (s uint64Slice) Len() int           { return len(s) }
func (s uint64Slice) Less(a, b int) bool { return s[a] < s[b] }
func (s uint64Slice) Swap(a, b int)      { s[a], s[b] = s[b], s[a] }
