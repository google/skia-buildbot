// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package output

import (
	"fmt"
	"testing"

	"github.com/luci/luci-go/common/sync/parallel"
	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/common/types"

	. "github.com/smartystreets/goconvey/convey"
)

type bundleBuilder struct {
	bundle *logpb.ButlerLogBundle
}

func (b *bundleBuilder) addStream(name types.StreamPath, idxs ...uint64) {
	if b.bundle == nil {
		b.bundle = &logpb.ButlerLogBundle{}
	}
	sp, sn := name.Split()
	be := logpb.ButlerLogBundle_Entry{
		Desc: &logpb.LogStreamDescriptor{
			Prefix: string(sp),
			Name:   string(sn),
		},
	}
	if len(idxs) > 0 {
		be.Logs = make([]*logpb.LogEntry, len(idxs))
		for i, idx := range idxs {
			be.Logs[i] = &logpb.LogEntry{
				StreamIndex: idx,
			}
		}
	}
	b.bundle.Entries = append(b.bundle.Entries, &be)
}

func (b *bundleBuilder) push(et *EntryTracker) {
	if b.bundle != nil {
		et.Track(b.bundle)
		b.bundle = nil
	}
}

func TestRecord(t *testing.T) {
	Convey(`An EntryTracker`, t, func() {
		var et EntryTracker
		var bb bundleBuilder

		Convey(`Will return an empty EntryRecord if empty.`, func() {
			So(et.Record(), ShouldResemble, &EntryRecord{})
		})

		Convey(`Will record continuous [0-10]`, func() {
			bb.addStream("foo/+/bar", 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
			bb.push(&et)

			So(et.Record(), ShouldResemble, &EntryRecord{
				Streams: map[types.StreamPath]*StreamEntryRecord{
					types.StreamPath("foo/+/bar"): {
						Ranges: []Range{{0, 10}},
					},
				},
			})
		})

		Convey(`Will record disjoint [0-2], [4], [6-7], [9-10]`, func() {
			bb.addStream("foo/+/bar", 0, 1, 2, 4, 6, 7)
			bb.addStream("foo/+/bar", 9, 10)
			bb.push(&et)

			So(et.Record(), ShouldResemble, &EntryRecord{
				Streams: map[types.StreamPath]*StreamEntryRecord{
					types.StreamPath("foo/+/bar"): {
						Ranges: []Range{{0, 2}, {4, 4}, {6, 7}, {9, 10}},
					},
				},
			})
		})

		Convey(`Will record reverse [0-2], [4], [6-7], [9-10]`, func() {
			bb.addStream("foo/+/bar", 10, 9)
			bb.addStream("foo/+/bar", 7, 6, 4, 2, 1, 0)
			bb.push(&et)

			So(et.Record(), ShouldResemble, &EntryRecord{
				Streams: map[types.StreamPath]*StreamEntryRecord{
					types.StreamPath("foo/+/bar"): {
						Ranges: []Range{{0, 2}, {4, 4}, {6, 7}, {9, 10}},
					},
				},
			})
		})

		Convey(`Will merge contiguous {0, 1, 2, 3, 4, 5} => [0-5]`, func() {
			bb.addStream("foo/+/bar", 0)
			bb.addStream("foo/+/bar", 1)
			bb.addStream("foo/+/bar", 2)
			bb.addStream("foo/+/bar", 3)
			bb.addStream("foo/+/bar", 4)
			bb.addStream("foo/+/bar", 5)
			bb.push(&et)

			So(et.Record(), ShouldResemble, &EntryRecord{
				Streams: map[types.StreamPath]*StreamEntryRecord{
					types.StreamPath("foo/+/bar"): {
						Ranges: []Range{{0, 5}},
					},
				},
			})
		})

		Convey(`Will merge sparse {2, 0, 5, 1, 4, 3} => [0-5]`, func() {
			bb.addStream("foo/+/bar", 2)
			bb.addStream("foo/+/bar", 0)
			bb.addStream("foo/+/bar", 5)
			bb.addStream("foo/+/bar", 1)
			bb.addStream("foo/+/bar", 4)
			bb.addStream("foo/+/bar", 3)
			bb.push(&et)

			So(et.Record(), ShouldResemble, &EntryRecord{
				Streams: map[types.StreamPath]*StreamEntryRecord{
					types.StreamPath("foo/+/bar"): {
						Ranges: []Range{{0, 5}},
					},
				},
			})
		})

		Convey(`Will merge multiple streams across multiple goroutines in pseudorandom order.`, func() {
			const workers = 20
			const entries = 1000
			const streams = 10

			// Build all of the single-entry tasks to deploy. This enables
			// pseudorandom deployment b/c of map internals.
			type task struct {
				stream types.StreamPath
				entry  int
			}

			remaining := make(map[task]struct{}, entries*streams)
			exp := map[types.StreamPath]*StreamEntryRecord{}
			for s := 0; s < streams; s++ {
				stream := types.StreamPath(fmt.Sprintf("foo/+/%d", s))

				for e := 0; e < entries; e++ {
					remaining[task{stream, e}] = struct{}{}
				}

				// Since we're iterating over streams, we might as well build the
				// expected state: that every stream has a single range,
				// [0..<entries-1>].
				exp[stream] = &StreamEntryRecord{
					Ranges: []Range{{0, entries - 1}},
				}
			}

			parallel.WorkPool(workers, func(taskC chan<- func() error) {
				// Iterate over remaining things to send. Send one log entry per stream.
				// Since this is a map, it will iterate in pseudorandom order.
				for k := range remaining {
					k := k

					taskC <- func() error {
						var b bundleBuilder
						b.addStream(k.stream, uint64(k.entry))
						b.push(&et)
						return nil
					}
				}
			})

			So(et.Record(), ShouldResemble, &EntryRecord{
				Streams: exp,
			})
		})
	})
}
