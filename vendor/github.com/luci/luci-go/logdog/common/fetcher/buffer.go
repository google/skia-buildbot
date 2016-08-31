// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package fetcher

import (
	"container/list"

	"github.com/luci/luci-go/logdog/api/logpb"
)

type logBuffer struct {
	l      list.List
	cur    []*logpb.LogEntry
	curIdx int

	count int
}

func (b *logBuffer) current() *logpb.LogEntry {
	for b.curIdx >= len(b.cur) {
		if b.l.Len() == 0 {
			return nil
		}
		b.cur = b.l.Remove(b.l.Front()).([]*logpb.LogEntry)
		b.curIdx = 0
	}

	return b.cur[b.curIdx]
}

func (b *logBuffer) next() {
	b.curIdx++
	b.count--
}

func (b *logBuffer) append(le ...*logpb.LogEntry) {
	b.l.PushBack(le)
	b.count += len(le)
}

func (b *logBuffer) size() int {
	return b.count
}
