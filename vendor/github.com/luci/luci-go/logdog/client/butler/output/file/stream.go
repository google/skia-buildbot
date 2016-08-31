// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package file

import (
	"container/heap"

	"github.com/luci/luci-go/logdog/api/logpb"
	"github.com/luci/luci-go/logdog/client/butler/output"
)

// stream is the stateful output for a single log stream.
type stream struct {
	// desc is this log stream's descriptor.
	desc *logpb.LogStreamDescriptor

	// entries is a binary heap of buffered log entries.
	entries logEntryHeap
	// stats is the set of output stats for this stream.
	stats output.StatsBase
}

func newStream(desc *logpb.LogStreamDescriptor) *stream {
	s := stream{
		desc: desc,
	}
	heap.Init(&s.entries)
	return &s
}

func (s *stream) ingestBundleEntry(be *logpb.ButlerLogBundle_Entry) {
	for _, le := range be.GetLogs() {
		heap.Push(&s.entries, le)
	}
}

func (s *stream) getBundleEntry() *logpb.ButlerLogBundle_Entry {
	be := logpb.ButlerLogBundle_Entry{
		Desc: s.desc,
	}

	if len(s.entries) > 0 {
		be.Logs = make([]*logpb.LogEntry, 0, len(s.entries))
		for s.entries.Len() > 0 {
			be.Logs = append(be.Logs, s.entries[0])
			heap.Pop(&s.entries)
		}

		be.Terminal = true
		be.TerminalIndex = be.Logs[len(be.Logs)-1].StreamIndex
	}

	return &be
}

// logEntryHeap is a heap.Interface implementation that stores log entries.
type logEntryHeap []*logpb.LogEntry

func (h logEntryHeap) Len() int           { return len(h) }
func (h logEntryHeap) Less(i, j int) bool { return h[i].StreamIndex < h[j].StreamIndex }
func (h logEntryHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *logEntryHeap) Push(e interface{}) {
	*h = append(*h, e.(*logpb.LogEntry))
}

func (h *logEntryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
