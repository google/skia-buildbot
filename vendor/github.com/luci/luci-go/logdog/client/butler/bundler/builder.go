// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package bundler

import (
	"fmt"
	"sort"

	"github.com/luci/luci-go/logdog/api/logpb"
)

// builderStream is builder data that is tracked for each individual stream.
type builderStream struct {
	// ButlerLogBundle_Entry is the stream's in-progress bundle entry.
	logpb.ButlerLogBundle_Entry
	// size incrementally tracks the size of the stream's entry.
	size int
}

// builder incrementally constructs ButlerLogBundle entries.
type builder struct {
	// size is the maximum permitted bundle size.
	size int

	// template is the base bundle template.
	template logpb.ButlerLogBundle
	// templateCachedSize is the cached size of the ButlerLogBundle template.
	templateCachedSize int

	// smap maps the builder state for each individual stream by stream name.
	streams map[string]*builderStream
}

func (b *builder) remaining() int {
	return b.size - b.bundleSize()
}

func (b *builder) ready() bool {
	// Have we reached our desired size?
	return b.hasContent() && (b.bundleSize() >= b.size)
}

func (b *builder) bundleSize() int {
	if b.templateCachedSize == 0 {
		b.templateCachedSize = protoSize(&b.template)
	}

	size := b.templateCachedSize
	for _, bs := range b.streams {
		size += sizeOfBundleEntryTag + varintLength(uint64(bs.size)) + bs.size
	}

	return size
}

func (b *builder) hasContent() bool {
	return len(b.streams) > 0
}

func (b *builder) add(template *logpb.ButlerLogBundle_Entry, le *logpb.LogEntry) {
	bs := b.getCreateBuilderStream(template)

	bs.Logs = append(bs.Logs, le)
	psize := protoSize(le)

	// Pay the cost of the additional LogEntry.
	bs.size += sizeOfLogEntryTag + varintLength(uint64(psize)) + psize
}

func (b *builder) setStreamTerminal(template *logpb.ButlerLogBundle_Entry, tidx uint64) {
	bs := b.getCreateBuilderStream(template)
	if bs.Terminal {
		if bs.TerminalIndex != tidx {
			panic(fmt.Errorf("attempt to change terminal index %d => %d", bs.TerminalIndex, tidx))
		}
		return
	}

	bs.Terminal = true
	bs.TerminalIndex = tidx

	// Pay the cost of the additional terminal fields.
	bs.size += ((sizeOfTerminalTag + sizeOfBoolTrue) +
		(sizeOfTerminalIndexTag + varintLength(bs.TerminalIndex)))
}

func (b *builder) bundle() *logpb.ButlerLogBundle {
	bundle := b.template

	names := make([]string, 0, len(b.streams))
	for k := range b.streams {
		names = append(names, k)
	}
	sort.Strings(names)

	bundle.Entries = make([]*logpb.ButlerLogBundle_Entry, len(names))
	for idx, name := range names {
		bundle.Entries[idx] = &b.streams[name].ButlerLogBundle_Entry
	}

	return &bundle
}

func (b *builder) getCreateBuilderStream(template *logpb.ButlerLogBundle_Entry) *builderStream {
	if bs := b.streams[template.Desc.Name]; bs != nil {
		return bs
	}

	// Initialize our maps (first time only).
	if b.streams == nil {
		b.streams = map[string]*builderStream{}
	}

	bs := builderStream{
		ButlerLogBundle_Entry: *template,
		size: protoSize(template),
	}
	b.streams[template.Desc.Name] = &bs
	return &bs
}
